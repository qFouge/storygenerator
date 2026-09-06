// Package story holds the schema every stage of the pipeline agrees on: the
// script generator writes it, the dialogue synthesizer reads it. Keeping it in
// its own package means neither side can quietly drift from the other.
package story

import (
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf8"
)

// Character is one speaker, defined once per story and referenced by ID from
// every line. VoiceID is the whole point: the same character must map to the
// same ElevenLabs voice in every chunk, and later in every future episode.
//
// Notes is not sent to ElevenLabs. It is fed back into the script-generation
// prompt so the character's personality stays consistent in the *writing*,
// which is the half of consistency that a fixed voice ID cannot buy.
type Character struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	VoiceID     string `json:"voice_id"`
	Notes       string `json:"notes"`
}

// Line is one spoken utterance. Speaker is a Character.ID, never a display
// name: the ID is what the voice lookup keys on.
type Line struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// Chunk is one Text-to-Dialogue request worth of story. The script generator
// decides the boundaries, because it understands narrative beats; a blind text
// splitter would cut mid-exchange.
type Chunk struct {
	ChunkIndex int    `json:"chunk_index"`
	Lines      []Line `json:"lines"`
}

// Chars counts the characters ElevenLabs will bill and limit: the sum of every
// line's text. It counts runes, not bytes — French accents are multi-byte in
// UTF-8, so len() would overcount "déjà" as 6 characters and reject payloads
// that are actually within budget.
func (c Chunk) Chars() int {
	n := 0
	for _, l := range c.Lines {
		n += utf8.RuneCountInString(l.Text)
	}
	return n
}

// Story is the full script: metadata, the cast, and the story already split
// into synthesizable chunks.
type Story struct {
	Title                    string      `json:"title"`
	Language                 string      `json:"language"` // ISO 639-1, e.g. "fr"
	TargetAudience           string      `json:"target_audience"`
	LearningGoal             string      `json:"learning_goal"`
	EstimatedDurationSeconds int         `json:"estimated_duration_seconds"`
	Characters               []Character `json:"characters"`
	Chunks                   []Chunk     `json:"chunks"`
}

// Voices maps character ID to voice ID, which is the lookup the synthesizer
// needs for every line.
func (s Story) Voices() map[string]string {
	v := make(map[string]string, len(s.Characters))
	for _, c := range s.Characters {
		v[c.ID] = c.VoiceID
	}
	return v
}

// Load reads a story from a JSON file.
func Load(path string) (Story, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Story{}, err
	}
	var s Story
	// Wrapping with %w keeps the original error inspectable by callers via
	// errors.Is / errors.As, while adding the context of which file failed.
	if err := json.Unmarshal(b, &s); err != nil {
		return Story{}, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}
