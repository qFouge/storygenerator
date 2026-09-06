// Command dialogue synthesizes one chunk of a story through the ElevenLabs
// Text-to-Dialogue endpoint.
//
// It is dry-run by default: credits are only spent when -send is passed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qdefou/storygenerator/dialogue"
	"github.com/qdefou/storygenerator/story"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("story", "testdata/chunk-fr.json", "story JSON file")
	index := flag.Int("chunk", 0, "chunk_index to synthesize")
	stability := flag.Float64("stability", dialogue.DefaultStability, "request-level stability: 0 creative, 0.5 natural, 1 robust")
	send := flag.Bool("send", false, "actually call ElevenLabs and spend credits")
	listVoices := flag.Bool("list-voices", false, "list the stock voices a free account can use, then exit")
	voiceLang := flag.String("voice-language", "", "with -list-voices, keep only voices labelled with this language (ISO 639-1, e.g. fr)")
	outDir := flag.String("out", "audio", "directory for the generated mp3")
	flag.Parse()

	if *listVoices {
		return printVoices(*voiceLang)
	}

	s, err := story.Load(*path)
	if err != nil {
		return err
	}

	chunk, err := pick(s, *index)
	if err != nil {
		return err
	}

	req, err := dialogue.Payload(s, chunk, *stability)
	if err != nil {
		return err
	}

	fmt.Printf("%s — chunk %d: %d lines, %d characters, %d voices\n",
		s.Title, chunk.ChunkIndex, len(chunk.Lines), chunk.Chars(), len(s.Characters))

	if !*send {
		body, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		fmt.Println("\ndry-run: nothing sent. Add -send to call ElevenLabs.")
		return nil
	}

	fmt.Printf("sending... (~%d credits)\n", chunk.Chars())
	audio, err := dialogue.Synthesize(context.Background(), os.Getenv("ELEVENLABS_API_KEY"), req)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	out := filepath.Join(*outDir, fmt.Sprintf("chunk_%d.mp3", chunk.ChunkIndex))
	if err := os.WriteFile(out, audio, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d KB)\n", out, len(audio)/1024)
	return nil
}

func pick(s story.Story, index int) (story.Chunk, error) {
	for _, c := range s.Chunks {
		if c.ChunkIndex == index {
			return c, nil
		}
	}
	return story.Chunk{}, fmt.Errorf("no chunk with chunk_index %d in %q", index, s.Title)
}

// printVoices lists the voices the key is actually allowed to synthesize with.
// Free accounts are limited to the stock voices; Voice Library voices fail with
// 402 paid_plan_required at synthesis time. Listing costs no credits.
func printVoices(language string) error {
	voices, err := dialogue.DefaultVoices(context.Background(), os.Getenv("ELEVENLABS_API_KEY"), language)
	if err != nil {
		return err
	}
	if len(voices) == 0 {
		if language != "" {
			// An empty result here is an answer, not a failure: it means no
			// stock voice is labelled with that language.
			fmt.Printf("no stock voice is labelled language=%q.\n", language)
			fmt.Println("run without -voice-language to see every stock voice, and expect an accent from the voice's own language.")
			return nil
		}
		return fmt.Errorf("no stock voices returned")
	}

	// LANGUAGE and ACCENT are the columns that decide whether a voice can carry
	// a French story without sounding foreign to it.
	fmt.Printf("%-22s  %-14s  %-8s  %-8s  %-8s  %-14s  %s\n",
		"VOICE_ID", "NAME", "LANGUAGE", "ACCENT", "GENDER", "AGE", "DESCRIPTION")
	for _, v := range voices {
		fmt.Printf("%-22s  %-14s  %-8s  %-8s  %-8s  %-14s  %s\n",
			v.VoiceID, v.Name, v.Label("language"), v.Label("accent"),
			v.Label("gender"), v.Label("age"), v.Label("description"))
	}
	fmt.Printf("\n%d stock voices. Copy three voice_ids into the characters of testdata/chunk-fr.json.\n", len(voices))
	return nil
}
