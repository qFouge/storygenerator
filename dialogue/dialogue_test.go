package dialogue

import (
	"strings"
	"testing"

	"github.com/qdefou/storygenerator/story"
)

func cast() story.Story {
	return story.Story{
		Language: "fr",
		Characters: []story.Character{
			{ID: "narrateur", VoiceID: "voice-n"},
			{ID: "lucie", VoiceID: "voice-l"},
		},
	}
}

func TestPayloadMapsSpeakersToVoices(t *testing.T) {
	chunk := story.Chunk{Lines: []story.Line{
		{Speaker: "narrateur", Text: "Ce matin-là, la mer avait disparu."},
		{Speaker: "lucie", Text: "Où est-elle passée ?"},
		{Speaker: "narrateur", Text: "Personne ne le savait encore."},
	}}

	got, err := Payload(cast(), chunk, DefaultStability)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}

	want := []Input{
		{Text: "Ce matin-là, la mer avait disparu.", VoiceID: "voice-n"},
		{Text: "Où est-elle passée ?", VoiceID: "voice-l"},
		{Text: "Personne ne le savait encore.", VoiceID: "voice-n"},
	}
	if len(got.Inputs) != len(want) {
		t.Fatalf("got %d inputs, want %d", len(got.Inputs), len(want))
	}
	for i := range want {
		if got.Inputs[i] != want[i] {
			t.Errorf("input %d = %+v, want %+v", i, got.Inputs[i], want[i])
		}
	}
	if got.ModelID != ModelID {
		t.Errorf("model_id = %q, want %q", got.ModelID, ModelID)
	}
	if got.LanguageCode != "fr" {
		t.Errorf("language_code = %q, want \"fr\"", got.LanguageCode)
	}
	if got.Settings == nil || got.Settings.Stability != DefaultStability {
		t.Errorf("settings = %+v, want stability %v", got.Settings, DefaultStability)
	}
	if got.Seed != nil {
		t.Errorf("seed = %v, want omitted", *got.Seed)
	}
}

func TestPayloadRejectsUnknownSpeaker(t *testing.T) {
	chunk := story.Chunk{Lines: []story.Line{
		{Speaker: "aristide", Text: "Je ne suis pas au casting."},
	}}

	_, err := Payload(cast(), chunk, DefaultStability)
	if err == nil {
		t.Fatal("want an error for a speaker absent from the cast")
	}
	if !strings.Contains(err.Error(), "aristide") {
		t.Errorf("error should name the offending speaker, got: %v", err)
	}
}

func TestPayloadRejectsSpeakerWithoutVoice(t *testing.T) {
	s := cast()
	s.Characters = append(s.Characters, story.Character{ID: "aristide"})
	chunk := story.Chunk{Lines: []story.Line{
		{Speaker: "aristide", Text: "On m'a oublié à la distribution des voix."},
	}}

	if _, err := Payload(s, chunk, DefaultStability); err == nil {
		t.Fatal("want an error when voice_id is empty")
	}
}

func TestPayloadRejectsOversizedChunk(t *testing.T) {
	chunk := story.Chunk{Lines: []story.Line{
		{Speaker: "narrateur", Text: strings.Repeat("a", MaxChars)},
		{Speaker: "lucie", Text: "a"},
	}}

	if _, err := Payload(cast(), chunk, DefaultStability); err == nil {
		t.Fatalf("want an error above the %d character limit", MaxChars)
	}
}

// The limit ElevenLabs enforces is characters, not bytes. French accents are
// two bytes each in UTF-8, so a byte-based count would reject this chunk even
// though it is only 1500 characters.
func TestPayloadCountsCharactersNotBytes(t *testing.T) {
	chunk := story.Chunk{Lines: []story.Line{
		{Speaker: "narrateur", Text: strings.Repeat("é", 1500)},
	}}

	if n := len(chunk.Lines[0].Text); n <= MaxChars {
		t.Fatalf("test is not exercising the byte/rune difference: %d bytes", n)
	}
	if _, err := Payload(cast(), chunk, DefaultStability); err != nil {
		t.Errorf("1500 accented characters should be accepted: %v", err)
	}
}

func TestPayloadRejectsEmptyChunk(t *testing.T) {
	if _, err := Payload(cast(), story.Chunk{}, DefaultStability); err == nil {
		t.Fatal("want an error for a chunk with no lines")
	}
}

// Guards the real test script against schema drift: it must load and turn into
// a valid payload without touching the network.
func TestTestdataChunkIsSynthesizable(t *testing.T) {
	s, err := story.Load("../testdata/chunk-fr.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Chunks) == 0 {
		t.Fatal("no chunks in the test story")
	}
	for _, c := range s.Chunks {
		req, err := Payload(s, c, DefaultStability)
		if err != nil {
			t.Fatalf("chunk %d: %v", c.ChunkIndex, err)
		}
		if len(req.Inputs) != len(c.Lines) {
			t.Errorf("chunk %d: %d inputs for %d lines", c.ChunkIndex, len(req.Inputs), len(c.Lines))
		}
	}
}
