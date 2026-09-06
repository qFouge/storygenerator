# Multi-voice synthesis: decisions

Design note for the Text-to-Dialogue slice. Answers the open points of
[brief-dialogue.md](brief-dialogue.md), including the three places where the
brief and the documented API disagree. Verified against the ElevenLabs API
reference and the Text-to-Dialogue capability page in September 2026.

---

## 1. Character consistency splits into three, and only two of them are buyable

The brief's §6 treats consistency as one problem with four mechanisms. In
practice it splits along a hard line: what the API can hold constant, and what
only the *writing* can.

**Voice identity — bought with `voice_id`.** One voice per character, declared
once in `Story.Characters` and referenced by ID from every line. The lookup is
`Story.Voices()`, and `dialogue.Payload` refuses any line whose speaker is not
in the cast. A character therefore cannot drift to another voice between
chunks, because there is nowhere for a second voice to come from.

**Personality identity — bought with `notes`.** The `notes` field is never sent
to ElevenLabs. It exists to be fed back into the script-generation prompt, so
that Aristide keeps speaking slowly and without condescension in episode nine.
This is the half of consistency a fixed voice cannot buy: the same voice reading
out-of-character lines still sounds wrong.

**Delivery identity — not buyable.** The brief (§4, §6.2) wants a `stability`
mode per character: `robust` for the narrator, `natural` for the others. The API
reference documents `settings` as a **request-level** object holding a single
`stability` **double**, default `0.5`. There is no per-input settings field.
Since a chunk's whole point is that several characters speak inside one request,
per-character stability cannot exist in this design.

So `Character.Stability` is not in the Go schema. Delivery is steered per line
by audio tags (`[chuckles]`, `[warmly]`) and per request by one `stability`
float.

*Rejected:* one request per stability group — narrator lines in a robust
request, dialogue in a natural one, stitched afterwards. It technically
delivers per-character stability, and it costs the thing Text-to-Dialogue
exists for: the model no longer sees the surrounding turns, so the interleaved
back-and-forth loses its timing and reactions. Not worth it before anyone has
heard a narrator actually drift. Reopen only with a recording that proves it.

## 2. Named stability modes are not encoded, because the numbers are not published

The prompting guide describes Creative, Natural and Robust in prose. The API
reference says `stability` is a double defaulting to `0.5`. Neither page states
which number each label maps to. `0.0 / 0.5 / 1.0` is the obvious guess and it
is probably right, but a `map[string]float64` in the code would present that
guess as knowledge.

So the CLI takes `-stability` as a raw float, `DefaultStability` is the API's
own `0.5`, and the flag's help text says "0 creative, 0.5 natural, 1 robust" —
where a wrong guess is visibly a hint, not a contract.

## 3. Chunking belongs to the model that understands beats

The 2000-character ceiling forces a five-minute script into two or three
requests. Splitting text after generation means splitting on a character count,
which lands mid-exchange as often as not.

**Decision: the script generator emits `chunks` already split**, and is told to
cut only at scene or beat boundaries, staying under 1800 characters for buffer.
The synthesizer never splits anything; it synthesizes what it is handed.

`dialogue.Payload` still enforces the ceiling. That is not redundant with the
prompt instruction: a model instruction is a hope, and the failure it prevents
here costs real credits on a request the API would reject anyway. Every caller
routes through `Payload`, so one guard covers every path — including the Gemini
one that does not exist yet.

## 4. The limit counts characters, and French is where that matters

`Chunk.Chars()` uses `utf8.RuneCountInString`, not `len`. In Go, `len` on a
string returns bytes, and every accented French character is two bytes in
UTF-8. A byte-based count would report a 1500-character French chunk as over
2900 and refuse to synthesize it. The pipeline generates French, so this is not
a theoretical edge case; it is the normal case. A test asserts it.

## 5. No SDK, and no dependencies at all

ElevenLabs publishes no official Go client, and the community ones predate the
multi-voice endpoint. The brief's recommendation — call the REST API with
`net/http` and `encoding/json` — is right, and it turns out to cost one struct
and one function.

The side effect is worth naming: after removing the previous direction's
Anthropic SDK, `go.mod` has **no external dependencies**. Nothing to audit,
nothing to upgrade, no `go.sum`. Gemini's SDK will be the first one back, and it
will be the only one.

## 6. Dry-run is the default, because takes cost money

The free tier is 10,000 credits a month at roughly one credit per character. The
test chunk is 1214 characters, so a single take costs about 12% of the monthly
budget and there are only eight or so in a month. Eleven v3 is alpha and the
brief warns that several generations may be needed for a good take.

So `cmd/dialogue` prints the exact request body and stops. `-send` is what
spends credits, and the character count is printed before the call either way.
Getting the payload wrong should cost nothing.

## 6b. On the free plan, only stock voices work through the API

The brief (§3) suggests picking voices from the Voice Library because it needs
no cloning setup. That is true in the dashboard and false through the API on a
free plan: a Voice Library voice returns

```
402 Payment Required
{"detail":{"code":"paid_plan_required",
 "message":"Free users cannot use library voices via the API."}}
```

Free accounts may synthesize with the **stock (default) voices** only.
Community Voice Library voices are a paid feature *for API use*, independently
of the credit balance — so credits are not the binding constraint here, the
plan is.

This is what justified `-list-voices`, which had been skipped as unnecessary
when voice IDs were going to be pasted in by hand. `GET /v2/voices` accepts
`voice_type=default`, which returns exactly the set the key is allowed to use,
so the filter is doing real work rather than being cosmetic: an unfiltered list
would be mostly voices that fail at synthesis time, after the payload is built
and the request is sent.

The call costs no credits, which is why it is not behind `-send`.

## 6c. A voice not trained on French will sound foreign reading French

Eleven v3 takes the *language* from the input text, so French text produces
French speech with any voice. The *accent* is a different question, and
ElevenLabs is explicit about it: a voice not trained on the language being
generated carries an accent from the language it was recorded in, and their
guidance is to pick a voice trained on the target language.

So "does the voice speak French" and "does the voice sound French" are two
questions, and only the first is answered by `language_code: "fr"`.

Voice records carry `language` and `accent` labels, and `GET /v2/voices` can
filter on them. `-list-voices` therefore prints both columns and accepts
`-voice-language fr`. Whether any *stock* voice is labelled French is unknown
and cannot be checked without a key — the command is built so that its output
answers the question rather than a doc claiming to.

An empty result under `-voice-language fr` is treated as an answer, not an
error: it would mean the free plan has no French-trained voice, and the command
says so and points back to the unfiltered list.

Three ways out if no stock voice is French, in increasing cost:

1. Accept the accent. A slightly Anglophone narrator is not fatal to a
   children's story, and this costs nothing.
2. Steer with an audio tag. Eleven v3 accepts accent tags such as
   `[French accent]`, which direct delivery in-region. Cheap to try, and it
   fits the tag mechanism the scripts already use.
3. Pay. Voice Library voices — which do include French ones — unlock on a paid
   plan. This is a plan decision, not a code change.

**Decision: option 2.** `-list-voices -voice-language fr` returned nothing, so
the free plan has no French-trained voice. Every line therefore opens with a
`[French accent]` tag, ahead of its emotion tag, and the cast is drawn from the
stock voices: Bill (Wise, Mature, Balanced, labelled `old`) narrates, Brian
(Deep, Resonant and Comforting) plays Aristide, Laura (Enthusiast, Quirky
Attitude, labelled `young`) plays Lucie.

Casting against the voice labels rather than the names is what made this
tractable: Aristide's note asks for "grave et rassurante", which is Brian's
description almost word for word, and Lucie needed the `age` label more than
any timbre adjective.

One risk this cast carries: two of the three voices are male, and the voice
labelled `old` narrates while the old character is played by a `middle_aged`
one. Whether the narrator and Aristide stay distinguishable — and whether the
inverted ages are audible — is a question for the first listen, not something
the labels can settle.

The tag lives in the story data, not in code. Nothing wraps or injects it: the
script generator will be told to emit it, exactly as it is told to emit emotion
tags. A code path that rewrote line text before synthesis would mean the JSON
on disk was no longer what was sent, which is worth more than the keystrokes it
would save.

Two costs to name. Tags are part of `inputs[].text`, so they are billed: the
accent tags added 96 characters to the test chunk, 1214 to 1310, about 8% per
take. And the brief warns against overusing tags — two per line is its own
stated ceiling, which this now sits exactly on. If tags start being read aloud
instead of acted on, the accent tag is the one to drop first, because the
emotion tags are doing work the accent tag only approximates.

**Lucie is the remaining casting risk.** The stock set has no child voice; the
closest label is `young`, which is what Jessica carries. A young adult reading
a seven-year-old's lines may or may not hold up for an audience of children,
and that is an ear judgement, not a schema one. If it does not hold up, the
ways out are to rewrite her older — at the cost of the identification a child
listener gets from a child character — or to pay for Voice Library access.

One expiry to note: the documentation states that the current default voices
are being replaced and expire on 31 December 2026. Hardcoding stock voice IDs
into story files is fine for now, but those IDs have a known end date, which is
a second argument for a voice bank once there is more than one story.

## 7. What was deliberately not built

- **`voicebank.json`** (§6.1, §7). With one story, the cast inside the story
  file *is* the voice bank. A separate file earns its place when a second story
  needs the same characters — that is also when the recurring-character series
  the brief wants becomes real.
- **`scriptgen` (Gemini)** and **`stitch` (ffmpeg)**. §9 of the brief says prove
  the smallest slice first, and both of those are downstream of a decision that
  can only be made by listening: which stability, and whether the chosen voices
  handle French well.
- **The streaming endpoint.** Nothing listens live; a file is the deliverable.
- **`seed`.** Useful for A/B-ing a wording change on one chunk. There is nothing
  to compare yet, and the field is one pointer away when there is.
- **A good-take cache** (§6.4). Correct idea, premature: it needs more than one
  chunk and more than one take to have anything to cache.

## 8. Still open

- **Which stability.** The whole point of the slice. Listen to `chunk_0.mp3` at
  `0.5`, then at `1.0`, and decide before any longer script is generated.
- **Whether `[French accent]` actually works.** It is a documented v3 tag, but
  a tag inside French text is an untested combination: the failure mode is the
  bracketed words being spoken instead of applied. First thing to listen for.
- **Whether a `young` voice passes for a child.** The stock set has no child
  voice; Laura is the closest. Judge by ear before deciding to rewrite Lucie
  older or to pay.
- **Whether the narrator and Aristide stay distinct.** Both are male stock
  voices. If they blend, swapping them is free — Bill is the one labelled
  `old`, which is Aristide's description.
- **Whether audio tags work in French text.** Tags are English tokens in square
  brackets sitting inside French sentences. The docs do not say what happens
  when the two languages mix, and the failure mode to listen for is a tag being
  read aloud rather than acted on.
- **The real credit rate.** The brief says the API is discounted relative to the
  website's one-credit-per-character, but the rate is unpublished. The first
  `-send` against a known 1214-character chunk measures it against the dashboard
  balance.
