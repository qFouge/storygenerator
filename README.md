# storygenerator

Generates ~5-minute multi-character audio stories that entertain and usually
teach something. A script generator writes the story as structured JSON already
split into synthesizable chunks; ElevenLabs Text-to-Dialogue turns each chunk
into audio with one distinct voice per character.

Code and documentation are in English. **Stories and audio are generated in
French.**

**Current state: the synthesis slice only.** One hand-written chunk goes to
ElevenLabs and comes back as an mp3. Script generation and stitching are not
implemented yet — see "What doesn't exist yet".

## Quick start

```bash
cp .env.example .env          # fill in ELEVENLABS_API_KEY
set -a; . ./.env; set +a      # no dotenv dependency; the shell loads it

go test ./...
go run ./cmd/dialogue -list-voices                       # stock voices your plan can use (free, no credits)
go run ./cmd/dialogue -list-voices -voice-language fr    # ...only ones labelled French, if any
go run ./cmd/dialogue                    # dry-run: prints the exact request body
go run ./cmd/dialogue -send              # calls ElevenLabs, writes audio/chunk_0.mp3
go run ./cmd/dialogue -send -stability 1 # same chunk, robust delivery
go run ./cmd/dialogue -send -seed 42     # pinned seed, to A/B a wording change
```

`-story` points at a different script file, `-chunk` picks a chunk by
`chunk_index`, `-out` changes the output directory. `-seed` makes takes
repeatable on a best-effort basis only; ElevenLabs does not guarantee
determinism. A 429 or 5xx is retried up to five times with doubling backoff;
any other error fails at once.

**On a free plan, use only the stock voices.** Voice Library voices return
`402 paid_plan_required` through the API regardless of your credit balance — the
constraint is the plan, not the credits. `-list-voices` shows the voices your
key is actually allowed to synthesize with, with their `language` and `accent`
labels; put three of their `voice_id`s into the `characters` of
`testdata/chunk-fr.json`.

**A voice speaking French is not the same as a voice sounding French.**
`language_code: fr` makes any voice read French as French — but a voice not
trained on French carries its own accent. Check the `LANGUAGE` and `ACCENT`
columns. No stock voice is currently French, so every line in
`testdata/chunk-fr.json` opens with a `[strong French accent]` tag, the form
the Eleven v4 prompting guide documents.

**Emotion is written into the line, not set by a flag.** The stock voices are
flat at every stability value, so lines carry several tags placed mid-sentence,
capitals on the word that carries the beat, and ellipses for weight. Tags and
capitals are billed as ordinary text: the test chunk costs 1560 characters
against 1214 untagged, so about six takes a month rather than eight. The
reasoning is in [docs/text-to-dialogue.md](docs/text-to-dialogue.md).

**Dry-run is the default on purpose.** The free tier is 10,000 credits a month
at roughly one credit per character, and the test chunk is 1560 characters — so
about six takes a month. `-send` is the only thing that spends them.

## Pipeline

```
testdata/chunk-fr.json ──► [1] load the story          story/story.go
                                 │  Story{Characters, Chunks}
                                 │
                           [2] build the request       dialogue/dialogue.go
                                 │  speaker → voice_id
                                 │  reject unknown speakers
                                 │  reject chunks over 2000 characters
                                 │
                           [3] POST /v1/text-to-dialogue
                                 │  model_id: eleven_v4
                                 │  language_code: fr
                                 ▼
                           audio/chunk_0.mp3

                           [0] script generation (Gemini)  ── not implemented
                           [4] stitching (ffmpeg)          ── not implemented
```

## The structuring decisions

**One `voice_id` per character, fixed in the story file.** The cast is declared
once in `characters` and every line references a character by `id`.
`dialogue.Payload` refuses any line whose speaker is not in the cast, so a
character cannot drift to another voice between chunks — there is nowhere for a
second voice to come from.

**Stability is one request-level float, not a per-character mode.** The brief
asked for `robust` narration alongside `natural` dialogue, but the API
documents `settings.stability` as a single double for the whole request. Since
a chunk exists precisely so several characters can speak in one call,
per-character stability is not expressible. Delivery is steered per line by
audio tags instead.

**The script generator does the chunking, not a text splitter.** Splitting
generated text on a character count lands mid-exchange. The model that
understands narrative beats picks the boundaries; the synthesizer only enforces
the ceiling as a backstop.

**Characters are counted as runes, not bytes.** French accents are two bytes in
UTF-8, so `len` would report a valid 1500-character chunk as 2900 and refuse
it.

**Emotion is a property of the text, not of a setting.** `stability` amplifies
whatever emotion the words already carry; it does not choose one. So delivery is
directed by several tags per line placed mid-sentence, capitals on the beat
word, and punctuation — established by listening, not from the docs.

**Non-verbal sounds are written out as words.** `[chuckles]` produced nothing
audible: a tag asking a voice for a sound it never made in training has nothing
to draw on, unlike one colouring an existing delivery. So the laugh is spelled
in the line — "Héhé... héhé !" — with the tag kept as reinforcement.

The full reasoning, including what was rejected, is in
[docs/text-to-dialogue.md](docs/text-to-dialogue.md).

## Files

| Path | Role |
|---|---|
| `story/story.go` | The schema both ends agree on: `Story`, `Character`, `Chunk`, `Line` |
| `dialogue/dialogue.go` | Chunk → Text-to-Dialogue request body, and the POST |
| `dialogue/voices.go` | Lists the stock voices the plan allows |
| `cmd/dialogue/` | CLI for one chunk |
| `testdata/chunk-fr.json` | Hand-written French three-character test chunk |
| `docs/` | Design decisions and archived briefs |
| `audio/` | Generated audio (git-ignored) |
| `out/` | Generated scripts (git-ignored) |

`go.mod` has no external dependencies. Everything here is the standard library.

## What doesn't exist yet

- **Script generation.** `scriptgen` will call Gemini
  (`google.golang.org/genai`) with the story schema as `ResponseSchema`, so the
  model is structurally forced into valid JSON instead of being asked nicely.
  It comes after the listening test, because the prompt has to encode a
  stability decision that has not been made yet.
- **Stitching.** Concatenating `chunk_*.mp3` with a short silence between them,
  by shelling out to `ffmpeg` — Go has no `pydub` equivalent.
- **A voice bank.** With one story, the cast inside the story file *is* the
  voice bank. A shared `voicebank.json` earns its place when a second story
  reuses the same characters.
- **A good-take cache.** Synthesis is not deterministic and a chunk may need several
  generations. Caching the take you liked, per chunk, avoids re-synthesizing
  four good minutes to fix one bad one.

The previous direction of this project — road-trip stories about Loire châteaux,
generated in French with the Anthropic API — is archived in
[docs/brief-initial.md](docs/brief-initial.md) and
[docs/coherence-multi-personnages.md](docs/coherence-multi-personnages.md), and
its code is in the first commit.

## Security

`.env` is git-ignored; `.env.example` holds placeholders only. No key belongs in
a tracked file, a prompt, or a story JSON. Keys are read from the environment at
call time and never written anywhere.

### What access the ElevenLabs key needs

ElevenLabs keys are **restricted by default** — you enable features explicitly
when creating one. This pipeline calls exactly one endpoint,
`POST /v1/text-to-dialogue`, so grant only the scope that covers speech
synthesis and leave everything else off: Speech to Text, Dubbing, Agents,
Studio, voice cloning and workspace administration are all unused.

Read access to voices is also needed, because `-list-voices` calls
`GET /v2/voices` to find which voices the plan permits. That call spends no
credits. Nothing here ever writes a voice, so write access is not required.

The dashboard's exact scope labels are not documented, so pick the narrowest
set that covers Text to Dialogue and verify with a dry-run followed by one
`-send`. A key that is too narrow fails loudly with a 401; a key that is too
broad fails silently, which is the worse of the two.

Two settings worth using beyond scopes:

- **Credit quota.** A per-key monthly character limit, after which charged
  requests fail. Set it below your plan's allowance — a few thousand
  characters is plenty here, and it means a retry loop cannot drain the month.
- **Expiry.** User keys can be given a lifetime (15 minutes to 30 days). A
  development key that expires on its own is one you cannot forget to rotate.

IP allowlisting is also available but impractical from a laptop with a changing
address; it becomes worth setting if this ever runs from a fixed CI host.

Note that the free tier is licensed for personal, non-commercial use.
