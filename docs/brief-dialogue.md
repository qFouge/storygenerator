# Project brief — archive

The brief given at the re-briefing of the project (September 2026), kept
verbatim as a historical reference. It replaced
[brief-initial.md](brief-initial.md) wholesale. The decisions taken in response
— including where the brief turned out to disagree with the ElevenLabs API — are
in [text-to-dialogue.md](text-to-dialogue.md); the current state of the project
is in the [README](../README.md).

> Note: three claims in this brief do not survive contact with the documented
> API (per-character `stability`, the ten-voice ceiling, and the named
> Creative/Natural/Robust modes). See the reality table in the design note.

---

# Multi-Character Story-to-Audio Pipeline — Brief for Claude Code

## 1. Goal

Generate ~5-minute audio stories (entertaining, and usually teaching something) for
children or curious adults. Multiple characters per story (not capped at 4).
Character voice + personality must stay consistent for the full runtime.

Language: **Go** (learning goal). Script generation: **Google Gemini API**.
Audio synthesis: **ElevenLabs Text-to-Dialogue (`eleven_v3`)**.

## 2. Language & LLM choice

**Why Go changes two things:**
- ElevenLabs has no official Go SDK (only Python/TypeScript). Unofficial community
  clients exist (`github.com/haguro/elevenlabs-go`, `github.com/taigrr/elevenlabs`)
  but predate the Text-to-Dialogue (multi-voice) endpoint, so they likely don't
  cover it. **Recommendation: call the REST API directly with `net/http` +
  `encoding/json`.** It's one struct and one POST call — more instructive than
  waiting on a library, and not much more code.
- No Go equivalent to Python's `pydub` for audio stitching. Practical approach:
  shell out to `ffmpeg` via `os/exec` to concatenate chunk files with a short
  silence between them. Normal practice even in mature Go audio tooling.

**Why Gemini for script generation:**
- Genuine free tier via Google AI Studio — no credit card, and it *resets*
  monthly/daily rather than being a one-time expiring trial credit.
- Official Go SDK: `google.golang.org/genai` (`go get google.golang.org/genai`).
- Native schema-constrained JSON output — hand it the schema in §4 and get
  back structurally valid JSON instead of parsing free text, which matters
  since this feeds directly into ElevenLabs' `inputs` array.

**Alternatives to know about:**
- **Groq** — also a genuine free tier, no card, very fast (LPU hardware,
  Llama/Gemma/DeepSeek models). No official Go SDK, but OpenAI-compatible, so
  a minimal `net/http` client works. Good if you want to compare open-weight
  output quality/cost against Gemini's, or want raw iteration speed.
- **Anthropic's Claude API** — no permanent free tier. New accounts get a
  one-time ~$5 credit that expires in 30 days, then pay-as-you-go. Strong
  writing quality, but doesn't fit "free tier to keep testing on."

## 3. ElevenLabs constraints to design around

| Constraint | Detail | Why it matters here |
|---|---|---|
| Free tier | **10,000 credits/month**, resets monthly, no card required, personal/non-commercial use only | ~1 credit/character on the website for `eleven_v3` (API is discounted, exact rate unpublished) → roughly 2 full 5-min test stories/month before you'd consider upgrading. Check elevenlabs.io's pricing page since tiers shift. |
| Endpoint | `POST /v1/text-to-dialogue` (batch) or `/v1/text-to-dialogue/stream` | Core synthesis call |
| `inputs` | List of `{text, voice_id}` pairs | One entry per spoken line |
| Max unique voices/request | **10** | Comfortably covers >4 characters |
| **Character limit** | Keep total chars across all `inputs[].text` **≤ 2000 per request** | A 5-min script (~3,500–4,500 chars) must be split into **2–3 chunks**, synthesized separately, then stitched into one file |
| Model | Only `eleven_v3` supports Text-to-Dialogue + audio tags | Set `model_id: "eleven_v3"` |
| `settings.stability` | `Creative` (expressive, can hallucinate) / `Natural` (balanced, default recommendation) / `Robust` (very stable, less expressive, ~v2-like) | Use `Natural` as default; `Robust` for the narrator if it drifts; `Creative` sparingly for comic-relief characters |
| Pauses | No SSML `<break>` in v3 | Use ellipses (`…`), em dashes, and audio tags for pacing instead |
| Voice choice | Professional Voice Clones aren't fully v3-optimized yet | Pick voices from the Voice Library, Instant Voice Clone, or Voice Design — no cloning setup needed to start |
| Reliability | v3 is alpha; short lines (<~250 chars) are more prone to inconsistent delivery; "several generations might be required" | Prefer slightly fuller lines over one-word interjections; plan for a regenerate-and-pick-best step during testing |
| `seed` | Optional, for reproducible output on a *given* input | Useful while iterating on one chunk's wording, not a substitute for consistent voice_id/settings |

## 4. Script schema (the contract between the LLM and the synthesizer)

The LLM should output the *entire* story as one JSON object, already segmented into
chunks that respect the 2000-character ceiling. This keeps chunking decisions in the
hands of the model that understands narrative beats, not a blind text-splitter.

```json
{
  "title": "The Case of the Missing Photosynthesis",
  "target_audience": "kids 7-10",
  "learning_goal": "how plants make food from sunlight",
  "estimated_duration_seconds": 300,
  "characters": [
    {
      "id": "narrator",
      "display_name": "Narrator",
      "voice_id": "VOICE_ID_NARRATOR",
      "stability": "robust",
      "notes": "warm, steady storyteller tone"
    },
    {
      "id": "pip",
      "display_name": "Pip the Squirrel",
      "voice_id": "VOICE_ID_PIP",
      "stability": "natural",
      "notes": "curious, energetic, asks a lot of questions"
    },
    {
      "id": "dr_oak",
      "display_name": "Dr. Oak the Tree",
      "voice_id": "VOICE_ID_DR_OAK",
      "stability": "natural",
      "notes": "slow, wise, a little dramatic"
    }
  ],
  "chunks": [
    {
      "chunk_index": 0,
      "lines": [
        { "speaker": "narrator", "text": "[warmly] Deep in Willow Forest, something strange was happening..." },
        { "speaker": "pip", "text": "[curious] Dr. Oak, why do your leaves turn green in the morning and stay green all day?" }
      ]
    },
    {
      "chunk_index": 1,
      "lines": [
        { "speaker": "dr_oak", "text": "[chuckles] Ah, young Pip... it's a bit of magic called photosynthesis." }
      ]
    }
  ]
}
```

Rules for the LLM generating this:
- Every chunk's summed `text` length must stay **under ~1,800 characters** (buffer below the 2,000 hard limit).
- Split chunks at natural scene/beat boundaries, never mid-sentence or mid-exchange.
- The same `speaker` id must map to the same `voice_id` across every chunk (defined once in `characters`, referenced by id in `lines`).
- Character count is not capped — use as many as the story needs.

Corresponding Go structs (for `scriptgen/schema.go`):

```go
type Character struct {
    ID          string `json:"id"`
    DisplayName string `json:"display_name"`
    VoiceID     string `json:"voice_id"`
    Stability   string `json:"stability"` // "creative" | "natural" | "robust"
    Notes       string `json:"notes"`
}

type Line struct {
    Speaker string `json:"speaker"`
    Text    string `json:"text"`
}

type Chunk struct {
    ChunkIndex int    `json:"chunk_index"`
    Lines      []Line `json:"lines"`
}

type Story struct {
    Title                     string      `json:"title"`
    TargetAudience            string      `json:"target_audience"`
    LearningGoal              string      `json:"learning_goal"`
    EstimatedDurationSeconds  int         `json:"estimated_duration_seconds"`
    Characters                []Character `json:"characters"`
    Chunks                    []Chunk     `json:"chunks"`
}
```

## 5. LLM prompt template (script-generation system prompt)

```
You write short, engaging audio-drama scripts for AI text-to-speech dialogue synthesis.

TASK
Write a ~5-minute (600-750 word) spoken story for {audience} that entertainingly
teaches: {topic}. Use {n_characters}+ distinct characters (a narrator plus at least
two others) with clear, consistent personalities.

OUTPUT FORMAT
Return ONLY valid JSON matching this schema: {paste schema from section 4}.

WRITING RULES
- Give each character a distinct voice/personality that stays consistent for every
  line they speak (same speech patterns, vocabulary level, energy).
- Segment the story into chunks. Each chunk's combined line text must stay under
  1,800 characters. Split only at natural scene or beat boundaries.
- Insert ElevenLabs v3 audio tags in square brackets to guide delivery, drawn from:
  emotion/delivery: [excited] [curious] [whispers] [sighs] [chuckles] [laughs]
  [sarcastic] [mischievously] [crying] [warmly] [dramatically]
  sound cues (sparing, kid-appropriate): [applause] [gulps] [gasp]
  Place a tag immediately before the text it should affect. Don't overuse tags -
  one or two per line is usually enough.
- Do not use SSML <break> tags. For pauses, use ellipses (...) or em dashes.
- Avoid single-word or very short lines where possible (v3 is less stable below
  ~250 characters of context); let short reactions sit inside a fuller line when
  it fits naturally.
- Write numbers, abbreviations, and units the way they should be spoken aloud
  (e.g. "twenty three" not "23", "Doctor" not "Dr.").
- Keep content age-appropriate for {audience}.
- End with a light, satisfying resolution that reinforces {topic}.
```

When calling Gemini from Go, pass this schema as the `ResponseSchema` in
`GenerateContentConfig` so the model is structurally forced into valid JSON
rather than relying on prompt instructions alone.

## 6. Consistency strategy (the part you care most about)

1. **Voice identity**: one `voice_id` per character, fixed once in a `voice_bank`
   mapping and reused verbatim in every chunk and every future story featuring that
   character (recurring-character series become possible this way).
2. **Delivery identity**: store each character's default `stability` mode alongside
   their `voice_id`, so narration always renders the same way relative to their
   dialogue counterparts.
3. **Personality identity**: the `notes` field in `characters` should be fed back
   into the LLM prompt on every generation (including future episodes) so tone of
   voice in the *writing* — not just the audio — stays consistent.
4. **Regeneration loop**: since v3 can require a few tries, synthesize each chunk,
   listen, and keep a "good take" cache per chunk rather than regenerating the whole
   5 minutes when only one chunk sounds off.

## 7. Suggested repo shape for Claude Code to scaffold (Go)

```
/scriptgen/main.go       -> calls Gemini (google.golang.org/genai) with the §5 prompt,
                             using the §4 schema as ResponseSchema
/scriptgen/schema.go     -> Go structs from §4 (Story, Character, Chunk, Line)
/voicebank.json          -> character id -> {voice_id, stability}, filled in after
                             picking voices in the ElevenLabs dashboard (or via GET /v2/voices)
/dialogue/payload.go     -> turns each chunk's Lines into a Text-to-Dialogue request
                             body: {inputs: [{text, voice_id}], model_id: "eleven_v3", settings: {...}}
/dialogue/synthesize.go  -> POSTs each chunk to https://api.elevenlabs.io/v1/text-to-dialogue
                             via net/http, saves chunk_N.mp3
/stitch/stitch.go        -> shells out to ffmpeg (os/exec) to concatenate chunk_*.mp3
                             in order, with a short silence/crossfade between chunks
.env                     -> XI_API_KEY, GEMINI_API_KEY
```

## 8. Documentation to hand to Claude Code

ElevenLabs:
- Docs index built for AI agents (fastest way to pull the full doc set): `https://elevenlabs.io/docs/llms.txt` (index) or `https://elevenlabs.io/docs/llms-full.txt` (everything in one file)
- Text to Dialogue overview: `https://elevenlabs.io/docs/overview/capabilities/text-to-dialogue`
- Quickstart with working code: `https://elevenlabs.io/docs/eleven-api/guides/cookbooks/text-to-dialogue`
- Full API reference (Create dialogue): `https://elevenlabs.io/docs/api-reference/text-to-dialogue/convert`
- Full API reference (Stream dialogue): `https://elevenlabs.io/docs/api-reference/text-to-dialogue/stream`
- Eleven v3 prompting guide (audio tags, stability modes, multi-speaker conventions): `https://elevenlabs.io/docs/best-practices/prompting`
- Models & metering: `https://elevenlabs.io/docs/overview/models`

Gemini:
- Go SDK repo + usage examples: `https://github.com/googleapis/go-genai`
- Rate limits / free tier details: `https://ai.google.dev/gemini-api/docs/rate-limits`

## 9. Suggested first test task for Claude Code

Don't start with the full 5-minute pipeline or the LLM call. First prove the
smallest useful slice, in this order:

1. Pick 3 voices from the ElevenLabs Voice Library (narrator + 2 characters), note their `voice_id`s.
2. Hand-write one ~150-word, 3-character test chunk matching the §4 schema — skip Gemini entirely for this step.
3. In Go: marshal it into a Text-to-Dialogue request body, `POST` to `/v1/text-to-dialogue` via `net/http`, save the mp3.
4. Listen for: does each character sound distinct, do the audio tags land, does stability feel right — decide `Natural` vs `Robust` per character before scaling up.
5. Only once that works, wire up `scriptgen/main.go` (Gemini call + schema) to generate a full 5-minute, multi-chunk story, and `stitch/stitch.go` to assemble it.

---

## Divergences between the brief and what was implemented

| Brief | Implemented | Why |
|---|---|---|
| Per-character `stability` (§4, §6.2) | Dropped; one request-level `stability` float | `settings` is documented as request-level with a single `stability` double. Per-character modes are not expressible in one call. |
| Max 10 unique voices per request (§3) | No such limit enforced | The capability page states there is no limit on the number of speakers. |
| `Creative` / `Natural` / `Robust` as named modes | Raw `float64`, default `0.5` | The numeric values behind the UI labels are not stated in the API reference; inventing an enum would have encoded a guess. |
| Schema has no language field | Added `Story.Language` (ISO 639-1) | Stories are generated in French, and the endpoint accepts `language_code`. |
| `/scriptgen/schema.go` owns the structs | `story/story.go` owns them | The schema is the contract between the generator and the synthesizer, so it belongs to neither. |
| `voicebank.json` (§6.1, §7) | Not created yet | With one story, the cast in the story file *is* the voice bank. The abstraction earns its place at the second story. |
| Steps 5 and beyond (Gemini, stitching) | Not implemented | §9 says prove the smallest slice first. |
| `.env` holds `XI_API_KEY` (§7) | `ELEVENLABS_API_KEY` | `xi-api-key` is the HTTP *header* name; the ElevenLabs docs pair it with an `ELEVENLABS_API_KEY` variable (`xi-api-key: $ELEVENLABS_API_KEY`). Their SDK quickstarts use the same name. |
| "Pick voices from the Voice Library [...] no cloning setup needed to start" (§3) | Stock (default) voices only; `-list-voices` filters to them | Free plans get `402 paid_plan_required` for Voice Library voices *via the API*, whatever the credit balance. The constraint is the plan, not the credits. |
