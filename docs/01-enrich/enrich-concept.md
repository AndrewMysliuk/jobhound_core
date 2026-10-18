# Vacancy facts — concept

> **Sequence 1.**
> Pair: [enrich-schemas-contracts.md](enrich-schemas-contracts.md), [enrich-tasks.md](enrich-tasks.md).

## Why

**Problem today:** `gate_role` is a title-phrase list that hard-rejects. It is the wrong tool for role, discipline, language, qualification, years, work mode, and eligibility — the wording varies, and one phrase kills the job. Stack tokens (`vue`, `golang`, `php`) already match on word boundaries with weights in YAML. Those stay.

**New solution:**

1. The model answers closed questions about one vacancy. It returns enum values and a quote. It does not score and it does not pick a bucket.
2. Go owns the fields, the enums, the prompt, the few-shots, and `PromptVersion`. The profile YAML owns which fields are on, the allow-pool, and the deltas.
3. A verdict is stored once per `(job_id, prompt_version, model, input_hash)`. The next profile run reads it. A YAML edit rescores with no model call. A prompt edit bumps `PromptVersion` and leaves old rows in place.
4. A field affects the bucket only after `make eval` has been run for it. Until then the value is shown and the delta is 0.

Primary entity: **Verdict**. One row is shared by both profiles. The profile only decides what to do with it.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/enrich/storage` |
| Orchestration | **Temporal** | `internal/enrich/workflows` |
| Model | **Anthropic Message Batches** | `internal/enrich/impl` |
| Scoring | existing phrase scorer, plus verdict signals | `internal/scoring` |
| Profiles | YAML allow-pool and deltas | `internal/profiles` |

The description is untrusted text. The prompt marks it as data, the reply is checked with JSON Schema, and a quote that is not a substring of the vacancy is discarded.

No new product route. Facts ride on the existing profile job payload. `ProfileRunWorkflow` does not wait for the batch.

## Verdict

**Input:** normalized title and description. The hash is SHA-256 of those two strings. Collector location is not part of the hash and not part of the prompt in this slice. Passing it as a separate block is open; until that is decided, work mode and country are whatever the posting text says.

**Output, per field:**

- **Pool fields** return every value the text supports. There is no primary. `role_function`, `primary_stack`, `discipline`, `language_requirement`, `qualification_required`.
- **Single-value fields** return one value. Two asserted values make the field `unknown`. `work_mode`, `work_country`, `sponsorship`, `years_experience_required`.

An asserted value needs a quote. Two absence values may omit it: `qualification_required = none`, `language_requirement = english_only`. Any other value with a missing or non-matching quote becomes `unknown`. A value outside the enum becomes `unknown`. On a pool field, one bad value is dropped; if nothing valid remains, the field is `unknown`.

`unknown` is an answer. An empty pool is a different answer: the model looked and asserted nothing. Pending is neither: the verdict row for the current hash does not exist yet.

**Lifecycle:** the profile run scores with phrases immediately. Jobs with no current verdict go to a batch. When the row is written, those jobs are rescored. `user_status` stays as it was.

The one-time backfill is the current TOP and REVIEW pool, plus jobs REJECTED by `GATE_ROLE`. TOP+REVIEW alone never shows the jobs the gate already killed.

Statuses on the job payload: **`off`** (enrich disabled) → **`pending`** (enabled, no row for this hash) → **`ready`**.

## Fields

| Field | Values | Profile | Shape |
| ----- | ------ | ------- | ----- |
| `role_function` | frontend, fullstack, backend, devops, data, ml, security, qa, mobile, non_engineering, management | andrew | pool |
| `primary_stack` | vue, react, angular, node, go, php, java, dotnet, python, other | andrew | pool |
| `discipline` | architecture, landscape, urban, interior, structural, mep, bim | lada | pool |
| `qualification_required` | none, riba_part2, riba_part3, arb, ncarb, ro_oar | lada | pool |
| `language_requirement` | english_only, german_fluent, romanian_required, other | lada | pool |
| `work_mode` | remote_full, hybrid, onsite | both | single |
| `work_country` | eu_ok, us_only, country_specific | both | single |
| `sponsorship` | no_sponsorship | both | single |
| `years_experience_required` | 0-1, 2-4, 5+ | both | single |

`work_country` and `sponsorship` are separate. "Based in Germany, no visa sponsorship" is both facts. `work_mode` has no country value; the country lives on `work_country`.

Years are one bucket. The prompt maps title words into that bucket even when there is no digit: junior / intern / entry → `0-1`, senior / staff / principal → `5+`. The quote is the word. Gold-set checks this. A missing number and no such word is `unknown`, not `0-1`.

`primary_stack` is extracted and shown. It does not replace `signals_plus` or `foreign_stack`.

The allow-pool is per profile, in YAML. Andrew's `role_function` pool is the functions he wants. Lada's `discipline` pool is the disciplines she wants. A returned value inside the pool uses that value's delta (default 0). A returned value outside the pool uses its own delta.

## Runtime

**Start:** `ProfileRunWorkflow` scores the window, then starts `EnrichBatchWorkflow` for job ids that have no row for the current `(prompt_version, model, input_hash)`. The child is abandoned when the parent finishes. Scoring has already completed.

**Backfill:** the same workflow, with the TOP + REVIEW + `GATE_ROLE` set. Not a second code path.

**Rescore:** a stored verdict triggers `Evaluate` again for matches of that job. Phrase signals and fact signals are recomputed. `user_status` is not written.

**Weight edit:** rescore, no model call.

**Prompt edit:** bump `PromptVersion` in Go. Old rows stay. The hash miss treats them as pending until the new batch lands.

**Vacancy text change:** hash miss, same as a prompt bump.

**Enrich disabled** (no API key): nothing is submitted, `facts_status` is `off`, scoring is phrases only.

### One vacancy

1. Normalize title and description. Hash them.
2. A row for this version, model, and hash is reused.
3. Otherwise the batch sends the delimited text and receives one JSON object.
4. Schema check, enum check, quote check. Failures coerce that value to `unknown`. They do not invent a bucket.
5. Store the verdict.
6. Rescore matches. A field with `scored: false` still emits a signal, with delta 0. Gates in YAML are ignored for that field.

**Failure:** a transport or batch error is a Temporal retry. No verdict row is written, so the job stays `pending`. A model reply that fails schema validation is stored as every field `unknown` only after the retries for a malformed body are exhausted — an empty row must not look like a successful abstention on the first glitch. `unknown` from a valid reply is stored as-is.

## Scoring rules

| Situation | Behavior |
| --------- | -------- |
| Enrich off | phrase scoring only, `facts_status = off` |
| No row for the current hash | `pending`, no fact signals |
| Field `scored: false` | signal visible, delta 0, YAML gates ignored |
| Value in the allow-pool | that value's delta, or 0 when the delta is omitted |
| Value outside the allow-pool | that value's delta |
| Value listed under `gates`, and the field is scored | `GATE_HARD`, bucket REJECTED |
| Empty pool | no signal for that field |
| Field `unknown` | no signal for that field |
| Phrase that this field replaces fired, and the allow-pool still matches | `CONFLICT`, delta 0, phrase gate still rejects |
| Two facts and a phrase both active for the same wording | the field is not turned on until those phrases are removed in the same change |

The model never sets REJECTED. A reject happens only when a measured field lists that value under `gates` and scoring applies it.

`CONFLICT` is the list of phrase rules that disagree with a verdict. It is emitted only for phrases explicitly mapped to a field. Unmapped phrases do not emit it. Seniority phrases (`junior`, `senior`, `lead`, `principal`, and the same kind) map to `years_experience_required`, not to `role_function` or `discipline`.

Turning a field on and deleting the phrases it replaces is one change. Lada's seniority lines in `gate_role` stay until years have been measured, even if `discipline` is already scored.

| Field, once scored | Phrases removed in that same change |
| --- | --- |
| `role_function` | andrew `gate_role` lines mapped to a function |
| `discipline` | lada `gate_role` lines mapped to a discipline |
| `years_experience_required` | seniority lines on both profiles, lada `mid_level`, the architecture years gate in the scorer |
| `qualification_required` | lada `gate_hard` licence lines |
| `language_requirement` | lada `gate_hard` language lines |
| `work_mode` | hybrid / onsite lines inside `office_pressure` |
| `work_country` | right-to-work and "authorized to work" lines, `ambiguous_us` |
| `sponsorship` | "no visa sponsorship" lines |
| `primary_stack` | none in this slice |

## Quote check

Normalize both the vacancy and the quote: Unicode NFC, newlines to spaces, collapse repeated whitespace, trim. Compare case-sensitively. A quote that is not a substring fails closed to `unknown` for that value. Absence values skip the check.

## Gold-set

`make eval` reads labeled vacancies and stored (or fixture) predictions. It prints, per field:

- accuracy where the label is not `unknown`
- share of predicted `unknown`
- share of confident errors (a value was predicted, the label differs or is `unknown`)

A model that answers `unknown` for everything is not a pass. The sample mixes jobs that survived scoring, jobs `GATE_ROLE` rejected, and known traps (`solutions engineer`, `java` beside javascript, `Landschaftsarchitekt`, a "part 3" that is not RIBA).

A field's `scored` flag is flipped only after that printout. This slice ships the flag as `false` for every field.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/enrich`** | contract, prompt, batch call, verdict storage, `EnrichBatchWorkflow` |
| **`internal/scoring`** | reads a ready verdict inside `Evaluate`; emits `LLM` and `CONFLICT` |
| **`internal/profiles`** | parses the `facts` block |
| **`internal/publicapi`** | adds `facts_status` and signal `value` on the existing job payload |
| **`cmd/worker`** | registers the workflow |

`internal/scoring/impl` does not import the Anthropic SDK or the Temporal SDK.

## Layers

- **Saved** — verdict rows, `PromptVersion`, enum text, few-shots.
- **Run** — batch id and which job ids were submitted. The profile run row is unchanged.
- **Wire** — model JSON in `internal/enrich/schema`. Profile job JSON gains `facts_status` and an optional signal `value`. The quote is the existing `phrase`.

## Implementation order

1. Fold the Latin diacritics the matcher still misses (`äöüß` → `aous`, `ß` → `ss`). Romanian fold already exists. This is phrase matching, independent of the model.
2. Verdict table, prompt, batch workflow, quote check.
3. Scoring reads verdicts with every field `scored: false`.
4. `make eval` and a small fixture. The 100-vacancy labeling is manual and is not a code task.
5. After a measured field, set `scored: true` and remove the mapped phrases in that same change. Not in the first implementation pass.

Early check: one fixture vacancy runs through quote coercion and `Evaluate`, produces an `LLM` signal at delta 0, and leaves the bucket on the phrase result.

## Out of scope

- The one-off Haiku audit of jobs already killed by `gate_role`. It stays outside the product path.
- Replacing stack weights with `primary_stack`.
- Putting collector location into the prompt or the hash.
- A numeric accuracy bar. The printout is the input to flipping `scored`.
- Choosing the bucket inside the prompt.
