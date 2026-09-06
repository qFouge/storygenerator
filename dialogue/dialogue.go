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
	"net/http"

	"github.com/qdefou/storygenerator/story"
)

// Endpoint is the batch (non-streaming) Text-to-Dialogue call.
const Endpoint = "https://api.elevenlabs.io/v1/text-to-dialogue"

// ModelID is the only model that supports Text-to-Dialogue and audio tags.
const ModelID = "eleven_v3"

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
// Both endpoints this package touches need the same key header and the same
// treatment of error bodies, so neither duplicates it.
func call(ctx context.Context, apiKey, method, url string, body []byte) ([]byte, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no API key: set ELEVENLABS_API_KEY")
	}

	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// The error body is where quota, plan and validation failures are
		// explained — a bare status code would have hidden the fact that free
		// keys cannot use Voice Library voices. Capped so a huge body cannot
		// flood the terminal.
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("elevenlabs %s: %s", resp.Status, msg)
	}
	return io.ReadAll(resp.Body)
}
