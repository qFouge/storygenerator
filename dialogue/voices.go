package dialogue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"sort"
)

// VoicesEndpoint lists the voices available to the account.
const VoicesEndpoint = "https://api.elevenlabs.io/v2/voices"

// Voice is the subset of a voice record worth showing when picking a cast.
type Voice struct {
	VoiceID  string            `json:"voice_id"`
	Name     string            `json:"name"`
	Category string            `json:"category"`
	Labels   map[string]string `json:"labels"`
}

// Label returns one label, or "-" when absent, so callers can format a table
// without checking every key.
func (v Voice) Label(key string) string {
	if s := v.Labels[key]; s != "" {
		return s
	}
	return "-"
}

// DefaultVoices lists the stock voices, which are the ones a free account may
// use through the API. Community Voice Library voices return 402
// paid_plan_required on the free plan, so filtering server-side with
// voice_type=default returns exactly the usable set rather than a list where
// most entries fail at synthesis time.
//
// language filters on the voice's own language label (ISO 639-1, e.g. "fr").
// This matters more than it looks: ElevenLabs' guidance is that a voice not
// trained on the target language carries an accent from the language it was
// recorded in. Pass an empty string to list every stock voice.
//
// This call costs no credits.
func DefaultVoices(ctx context.Context, apiKey, language string) ([]Voice, error) {
	url := VoicesEndpoint + "?voice_type=default&page_size=100"
	if language != "" {
		url += "&language=" + urlpkg.QueryEscape(language)
	}
	body, err := call(ctx, apiKey, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Voices []Voice `json:"voices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%s: %w", VoicesEndpoint, err)
	}

	sort.Slice(payload.Voices, func(i, j int) bool {
		return payload.Voices[i].Name < payload.Voices[j].Name
	})
	return payload.Voices, nil
}
