# Multi-voice synthesis: decisions

Design note for the Text-to-Dialogue slice. It answers the open points of
[brief-dialogue.md](brief-dialogue.md), records every place the brief and the
documented API disagree — the full list is the divergence table at the foot of
the brief — and keeps the results of listening to what came back, which is the
only way several of these questions could be settled at all.

Claims about the API were verified against the ElevenLabs API reference, the
Text-to-Dialogue capability page, the v3 prompting guide and the audio-tags
guide in September 2026. Claims about how things *sound* come from actually
listening to generated chunks, and are marked as such.

---

## 0. English code and documentation, French stories

Decided at the re-briefing: Go identifiers, JSON field names, comments, the
README and these notes are written in English; the stories and the synthesized
audio are in French. The previous direction had been French throughout, down to
identifiers like `Valider` and `Demande`.

The split has three consequences that show up repeatedly below, and it is worth
naming them together because they are easy to mistake for unrelated problems:

- the character ceiling must be counted in runes, since French is accented (§4);
- `language_code` is set to `"fr"`, which is a field the brief's schema did not
  have (§1);
- the voice casting problem is entirely created by it — the stock voices are
  not French (§8) — as is the need to write non-verbal sounds in French
  orthography (§10).

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

## 7. On the free plan, only stock voices work through the API

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

## 8. A voice not trained on French will sound foreign reading French

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
stock voices: Brian (Deep, Resonant and Comforting) narrates, George (Warm,
Captivating Storyteller) plays Aristide, Laura (Enthusiast, Quirky Attitude,
labelled `young`) plays Lucie.

Casting against the voice labels rather than the names is what made this
tractable, and the labels reward being read literally: George is described as a
storyteller and Aristide is precisely the character who tells the story inside
the story, while Lucie needed the `age` label more than any timbre adjective.

The cast reached this shape by ear over several rounds. Bill, the only voice
labelled `old`, narrated first and was the flattest of the lot despite being the
best fit on paper for a wise narrator — which is the lesson of §9 in miniature:
the label predicts fit, not expressiveness.

One risk remains: two of the three voices are male and both are `middle_aged`,
so whether the narrator and Aristide stay distinguishable is a question for the
ear. "Deep, Resonant" against "Warm, Captivating" is a wider gap than the
previous pairing had, but it is still two middle-aged men.

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

## 9. Expressiveness lives in the text, not in the stability knob

First listening result: the accent tag works and the voices carry French, but
the two male voices came out emotionally flat, the narrator worst. Turning
`stability` down to `0` (Creative) did not open them up — it made the whole
chunk sound *sad*, and `1` (Robust) was no better.

That is the stability knob doing what it says. It controls how far the model may
depart from the voice's recorded delivery; it does not tell it *which* emotion
to depart towards. With a wistful text — a sea that has vanished, a child
falling silent — the extra latitude at `0` went into melancholy, because that
is what the words carried. The knob amplifies the text's emotion; it does not
choose it.

**Decision: emotional direction is written into the line, and stability stays at
the default.** Three mechanisms, all documented for v3, all inside
`inputs[].text`:

1. **Several tags per line, placed mid-sentence**, each immediately before the
   phrase it affects. A line is not one emotion; it is a small arc.
2. **Capitalization for emphasis** on the one word that carries the beat —
   "quelque chose d'IMPOSSIBLE", "Elle la TIRE". This is the per-word
   granularity that tags cannot express.
3. **Punctuation.** The documentation is explicit that it significantly affects
   delivery in v3: ellipses add pause and weight, exclamation marks lift.

The test chunk was rewritten on all three, and its emotional register moved
from wistful to wondering — the narrator now opens on something IMPOSSIBLE and
closes on delight rather than on a child going quiet.

The tag vocabulary is also wider than the brief's §5 list, which matters when
that list becomes `scriptgen`'s prompt. Beyond the emotions the brief names, the
documentation and the audio-tags guide use `[sorrowful]`, `[calm]`,
`[cheerfully]`, `[playfully]`, `[flatly]`, `[deadpan]`, `[nervous]`,
`[frustrated]`, and cognitive beats such as `[pauses]`, `[hesitates]` and
`[stammers]`. Tags are written lowercase in square brackets.

**This contradicts the brief.** Its §5 instructs the script generator "don't overuse
tags — one or two per line is usually enough". Against these stock voices that
is too thin: lines now carry three to five tags. The brief's advice is sound for
an expressive voice and wrong for a flat one, so the rule that goes into
`scriptgen`'s prompt has to be conditional on the cast, not absolute.

The cost is real and worth tracking: tags and emphasis are billed as ordinary
text. The chunk went from 1214 characters bare, to 1310 with accent tags, to
1504 now — 24% above where it started, on a 10,000-character monthly budget.
Roughly six takes a month rather than eight.

If tags start being *spoken* rather than acted on, thinning them is the first
move, and the accent tag is the one to drop first since the emotion tags are
doing the work this section is about.

## 10. Non-verbal tags depend on the voice; write the sound into the words

Second listening result: the emotion tags landed and the delivery improved, but
`[chuckles]` produced nothing audible at all.

This is documented behaviour rather than a bug. The audio-tags guide says a tag
read as nothing — or read aloud — "typically occurs when the selected voice
doesn't match the requested delivery, such as a naturally soft-spoken voice
being asked to perform several [shouts]", and its remedy is to "choose a voice
with a wide emotional range for tag-heavy scripts".

That remedy is unavailable here. Every voice the free plan offers is described as
controlled — Deep and Resonant, Warm and Captivating, Enthusiast and Quirky.
None is a wide-range performer, and a purely non-verbal tag has nothing in the
voice's recorded range to draw on. Emotional *colouring* tags work because they
bend an existing delivery; a laugh has to be produced from scratch, and a voice
that never laughed in training cannot.

**Decision: non-verbal sounds are written into the text, with the tag kept
alongside as reinforcement.** Aristide's line now reads "Héhé... héhé !
[chuckles] Ah non, ma petite Lucie" rather than "[chuckles] Ah non". The
written laugh is speech, so it cannot be skipped; the tag then shapes how it is
performed instead of being solely responsible for its existence.

This generalises the §9 finding: the text is the reliable channel and the tags
are the modifier. It is also the rule `scriptgen`'s prompt needs for every
non-verbal — `[sighs]`, `[gasps]`, `[laughs]` — not just this one, since they
all ask the voice for a sound rather than for a manner.

Written laughter is language-specific, which the prompt will have to say out
loud: "Héhé" is a French chuckle, "heh heh" is not.

Cost: 14 characters, 1504 to 1518.

## 11. Which versions are current (checked September 2026)

Three version numbers appear in this pipeline and they are independent of each
other, which invites the suspicion that one of them is stale. It is worth
writing down so the question does not have to be reopened:

| Where | Value | Status |
|---|---|---|
| Voice listing | `GET /v2/voices` | Current. There is no `/v3/voices`; the legacy one is `/v1/voices`, which this code does not use. |
| Synthesis | `POST /v1/text-to-dialogue` | Current. This endpoint has only ever been v1. |
| Model | `eleven_v3` | Current. No v4 exists; it is still documented as the most advanced model. |

So `v2` in the voices URL is the newest version of that endpoint, not a leftover,
and the `v3` in the model id is a model generation rather than an API version.

One neighbouring model is worth knowing about and rejecting explicitly:
`eleven_v3_conversational`, a realtime variant at roughly 280ms latency that
drives a **WebSocket** Text-to-Dialogue API. It buys latency, not quality, and
this pipeline writes files that nobody listens to as they are produced. It would
be the right model for a live agent and is the wrong one here.

`eleven_turbo_v2_5` and `eleven_turbo_v2` are documented as outclassed by their
Flash equivalents; neither is relevant, since only `eleven_v3` supports
multi-speaker dialogue with audio tags.

One more pair of numbers worth separating, because meeting the larger one first
would suggest the code's limit is wrong: `eleven_v3` is documented with a
5,000-character limit as a *model*, while Text to Dialogue caps a *request* at
2,000 characters across all of `inputs[].text`. The lower one binds here, and
`MaxChars` is set from it.

## 12. What listening has established so far

Three takes have been generated and listened to. Since several of these
questions cannot be answered from documentation at all, the results are the
evidence behind §8, §9 and §10 and are worth stating plainly:

| Question | Answer |
|---|---|
| Do the stock voices carry French? | Yes. |
| Is `[French accent]` acted on or read aloud? | Acted on. |
| Do emotion tags land? | Yes, once several are placed mid-line with emphasis and punctuation. |
| Does `[chuckles]` land? | No — nothing audible. Sound-producing tags need the sound written into the text. |
| Does `stability` control expressiveness? | No. `0` made the chunk sound sad; `1` was no better. It amplifies the text's emotion rather than choosing one. |
| Which cast? | Settled by ear over three rounds, not from labels — see §8. |

The casting rounds are worth recording because the discarded ones carry the
lesson: Adam (Dominant, Firm) and Bella were replaced for fit, Jessica for
Laura, and Bill — the only voice labelled `old`, and the best match on paper for
a wise narrator — was dropped for being the flattest of the lot.

## 13. What was deliberately not built

- **`voicebank.json`** (brief §6.1 and §7). With one story, the cast inside the story
  file *is* the voice bank. A separate file earns its place when a second story
  needs the same characters — that is also when the recurring-character series
  the brief wants becomes real.
- **`scriptgen` (Gemini)** and **`stitch` (ffmpeg)**. §9 of the brief says prove
  the smallest slice first, and both of those are downstream of a decision that
  can only be made by listening: which stability, and whether the chosen voices
  handle French well.
- **The streaming endpoint.** Nothing listens live; a file is the deliverable.
- **An explicit `output_format`.** The endpoint takes it as a query parameter
  and defaults to `mp3_44100_128`, so the code relies on that default while
  naming its output `chunk_N.mp3`. The two agree today and nothing verifies that
  they still would: a changed default would produce correct audio under a
  misleading extension. Setting it explicitly is one query parameter, and is the
  right fix the moment anything but a person's ear consumes these files.
- **`seed`.** Useful for A/B-ing a wording change on one chunk. There is nothing
  to compare yet, and the field is one pointer away when there is.
- **A good-take cache** (brief §6.4). Correct idea, premature: it needs more than one
  chunk and more than one take to have anything to cache.


## 14. Still open

- **Which stability.** Answered, and the answer is that it is the wrong knob for
  this problem: `0` sounded sad, `1` no better, so the default `0.5` stands and
  expressiveness is written into the text instead (§9).
- **Whether the written laugh lands where the bare tag did not.** "Héhé..."
  cannot be skipped the way `[chuckles]` was, but whether it sounds like a laugh
  or like two spoken syllables is the open question.
- **Whether lower stability would have rescued the tags instead.** `Robust` is
  documented as "less responsive to directional prompts", which implies
  `Creative` is more responsive. `0` was rejected earlier for sounding sad, but
  that was a different cast reading a wistful text; both have since changed, so
  the earlier verdict does not carry over. One take would settle it.
- **Whether a `young` voice passes for a child.** The stock set has no child
  voice; Laura is the closest. Judge by ear before deciding to rewrite Lucie
  older or to pay.
- **Whether the narrator and Aristide stay distinct.** Both are male and both
  `middle_aged`. The stock set offers no third male register, so if they blend
  the fix is not another swap.
- **How far the tag-heavy approach can be pushed.** Answered in part by
  listening: English tags inside French text are acted on, not read aloud, and
  three to five per line are tolerated. Colouring tags work; sound-producing
  ones did not (§10). Where the ceiling actually sits is untested.
- **The real credit rate.** The brief says the API is discounted relative to the
  website's one-credit-per-character, but the rate is unpublished. The first
  `-send` against a known 1214-character chunk measures it against the dashboard
  balance.
