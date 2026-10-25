// Package dialogue turns a story chunk into an ElevenLabs Text-to-Dialogue
// request and performs the call. ElevenLabs publishes no official Go SDK, and
// the community ones predate the multi-voice endpoint, so this talks to the
// REST API directly: one struct and one POST.
package dialogue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/qdefou/storygenerator/story"
)

// Endpoint is the batch (non-streaming) Text-to-Dialogue call. output_format
// is pinned rather than left to the API default, so the .mp3 extension the CLI
// writes cannot silently disagree with what comes back.
const Endpoint = "https://api.elevenlabs.io/v1/text-to-dialogue?output_format=mp3_44100_128"

// ModelID is the model ElevenLabs recommends for Text-to-Dialogue. eleven_v3
// also supports it — see docs/text-to-dialogue.md §11.
const ModelID = "eleven_v4"

// maxAttempts bounds the retries on 429 and 5xx, so a bad day at ElevenLabs
// cannot turn into an endless loop.
const maxAttempts = 5

// retryWait is the first backoff delay; it doubles on every retry. It is a
// variable rather than a constant only so the test does not sleep for real.
var retryWait = 5 * time.Second

// client bounds each attempt. http.DefaultClient has no timeout at all, so a
// stalled connection would hang the CLI forever.
var client = &http.Client{Timeout: 5 * time.Minute}

// MaxChars is the documented ceiling on the summed length of inputs[].text for
// a single request. The script generator aims well below it; this is the
// backstop that refuses to spend credits on a request the API would reject.
const MaxChars = 2000

// DefaultStability corresponds to the "Natural" mode in the ElevenLabs UI and
// is the API default. It is a request-level knob, not a per-character one — see
// docs/text-to-dialogue.md.
const DefaultStability = 0.5

// Input is one spoken line, paired with the voice that should speak it.
type Input struct {
	Text    string `json:"text"`
	VoiceID string `json:"voice_id"`
}

// Settings applies to the whole request. ElevenLabs documents stability as a
// single double for the request, which is why it is not per character.
type Settings struct {
	Stability float64 `json:"stability"`
}

// Request is the JSON body of a Text-to-Dialogue call. Pointer fields with
// omitempty are the Go idiom for "optional": a nil pointer disappears from the
// JSON entirely, where a zero float64 would be sent as a meaningful 0.
type Request struct {
	Inputs       []Input   `json:"inputs"`
	ModelID      string    `json:"model_id"`
	LanguageCode string    `json:"language_code,omitempty"`
	Settings     *Settings `json:"settings,omitempty"`
	Seed         *int      `json:"seed,omitempty"`
}

// Payload builds the request body for one chunk.
//
// Every caller routes through here, so this is where the two failure modes
// that would otherwise reach the API belong: a line whose speaker has no voice,
// and a chunk over the character ceiling.
func Payload(s story.Story, chunk story.Chunk, stability float64) (Request, error) {
	if n := chunk.Chars(); n > MaxChars {
		return Request{}, fmt.Errorf("chunk %d: %d characters exceeds the %d limit", chunk.ChunkIndex, n, MaxChars)
	}
	if len(chunk.Lines) == 0 {
		return Request{}, fmt.Errorf("chunk %d: no lines", chunk.ChunkIndex)
	}

	voices := s.Voices()
	inputs := make([]Input, len(chunk.Lines))
	for i, l := range chunk.Lines {
		voice, ok := voices[l.Speaker]
		if !ok {
			return Request{}, fmt.Errorf("chunk %d line %d: speaker %q is not in the cast", chunk.ChunkIndex, i, l.Speaker)
		}
		if voice == "" {
			return Request{}, fmt.Errorf("chunk %d line %d: speaker %q has no voice_id", chunk.ChunkIndex, i, l.Speaker)
		}
		inputs[i] = Input{Text: l.Text, VoiceID: voice}
	}

	return Request{
		Inputs:       inputs,
		ModelID:      ModelID,
		LanguageCode: s.Language,
		Settings:     &Settings{Stability: stability},
	}, nil
}

// Synthesize POSTs the request and returns the audio bytes (mp3 by default).
func Synthesize(ctx context.Context, apiKey string, r Request) ([]byte, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return call(ctx, apiKey, http.MethodPost, Endpoint, body)
}

// call performs an authenticated request and returns the raw response body.
// Both endpoints this package touches need the same key header, the same
// treatment of error bodies and the same retries, so neither duplicates them.
//
// Only 429 and 5xx are retried. ElevenLabs documents a 429 as either the
// plan's concurrency cap or platform-wide load, both of which clear on their
// own. A 4xx such as 402 paid_plan_required or a validation error would fail
// identically every time, so it is returned at once.
func call(ctx context.Context, apiKey, method, url string, body []byte) ([]byte, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no API key: set ELEVENLABS_API_KEY")
	}

	wait := retryWait
	for attempt := 1; ; attempt++ {
		status, data, err := once(ctx, apiKey, method, url, body)
		if err != nil {
			return nil, err
		}
		if status == http.StatusOK {
			return data, nil
		}

		err = fmt.Errorf("elevenlabs %d %s: %s", status, http.StatusText(status), data)
		retryable := status == http.StatusTooManyRequests || status >= 500
		if !retryable || attempt == maxAttempts {
			return nil, err
		}
		log.Printf("%v — retry %d/%d in %s", err, attempt, maxAttempts-1, wait)

		// select waits on whichever happens first: the backoff elapsing, or
		// the caller cancelling the context. A bare time.Sleep would ignore
		// a cancellation for up to the full delay.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
	}
}

// once performs a single attempt. A non-200 response is not an error here: it
// comes back as a status and body, so call can decide whether to retry.
func once(ctx context.Context, apiKey, method, url string, body []byte) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("xi-api-key", apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// The error body is where quota, plan and validation failures are
		// explained — a bare status code would have hidden the fact that free
		// keys cannot use Voice Library voices. Capped so a huge body cannot
		// flood the terminal.
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return resp.StatusCode, msg, nil
	}
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}
