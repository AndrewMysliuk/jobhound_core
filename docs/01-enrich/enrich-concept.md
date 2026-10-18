# Vacancy facts — concept

> **Sequence 1.**
> Pair: [enrich-schemas-contracts.md](enrich-schemas-contracts.md).

## Why

**Goal:** more relevant vacancies in `top` and `review`, and a lower share of mistakes in those buckets. The pull is already large. The time goes into cleaning it by hand.

**Problem today:** country, timezone, work rights, and pay are free text. The same fact is written as `USA`, `U.S.`, `Remote - United States`, `Bangalore`, `LATAM`, `₹`, `lakh`, `25 LPA`. Phrase lists and the salary parser do not cover those spellings. The salary parser only understands EUR, USD, and GBP, so a rupee salary never even becomes a penalty. A remote job skips the country allow-list, and the collector often has no country code to check. Growing the phrase list does not close this. Junk stays in the buckets that get read, and relevant vacancies stay mixed in with it.

**New solution:**

1. The model reads one vacancy and answers a fixed list of questions. The answers are normalized facts: country codes, regions, currency, a timezone zone, a stack token. A quote points at the span that justified the answer.
2. The model does not know which profile is asking, does not score, and does not pick `top`, `review`, or `rejected`.
3. One verdict is stored per `(job_id, prompt_version, model, input_hash)`. Every profile reads that same row. The profile YAML decides which answers reject and which do nothing.
4. A YAML edit rescores with no model call. A prompt edit bumps `PromptVersion` and leaves old rows in place.
5. A fact changes a bucket only after `make eval` for that field. Until then the answer is shown and the delta is 0. The phrase lists stay until the field that replaces them is turned on in that same change.

Primary entity: **Verdict**.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/enrich/storage` |
| Orchestration | **Temporal** | `internal/enrich/workflows` |
| Model | **Anthropic Message Batches** (Haiku via `JOBHOUND_ENRICH_MODEL`) | `internal/enrich/impl` |
| Scoring | existing phrase scorer, then the stored answers | `internal/scoring` |
| Profiles | YAML rules over those answers | `internal/profiles` |

The description is untrusted text. The prompt marks it as data, the reply is checked with JSON Schema, and a quote that is not a substring of the vacancy is discarded.

No new product route. Facts ride on the existing profile job payload. `ProfileRunWorkflow` does not wait for the batch.

## What the model is asked

The prompt is these questions, plus the instruction that the block is data. Go owns the wording, the few-shots, and `PromptVersion`.

1. Where is the company? Answer with a country code or a region, however the text spells the place.
2. Where must the person be? Anywhere, not stated, or only these places.
3. Does the posting require an existing right to work, and in which country?
4. Which timezone or working hours are required?
5. Which currency is the pay in, and what range is stated?
6. What is the role, the stack, the discipline, the required language, the required qualification, and the years asked for?

Answers Go does not invent and does not translate:

- A city or a nickname becomes a code in the model, not in a city dictionary. `Bangalore` → `IN`, `São Paulo` → `BR`, `LATAM` → `LATAM`, `U.S.` → `US`.
- `₹`, `INR`, `lakh`, `LPA` become currency `INR`. The amount is kept when the text states one. A currency does not imply a country.
- A location badge such as `Remote - United States` is the office. It becomes a candidate restriction only when the text also says the person must be there, or must already have the right to work.
- "Anywhere", "worldwide", and a remote posting that names no candidate restriction are the same answer: hiring scope `open`.
- Nothing stated is `open` or `none`, not `unknown`. `unknown` is for a contradiction, or for an answer that failed the quote check.

## Verdict

**Input:** title, description, the raw location line, and the raw salary line. The hash is SHA-256 of those four strings. The collector's parsed country codes are not in the prompt and not in the hash. The model normalizes the text itself.

**Output, per answer:**

- **Place lists** return every code the text supports. `office`, and the country lists inside `hiring_scope` and `work_authorization`.
- **Scope answers** return one value. `hiring_scope` is `open` or `restricted`. `work_authorization` is `none` or `required`. `timezone` is `open` or `restricted`. `work_mode` is `remote_full`, `hybrid`, or `onsite`. Two asserted values make that answer `unknown`.
- **Compensation** returns one currency code, an optional min and max, and a period. No currency in the text leaves the currency empty.
- **Role answers** stay pools. `role_function`, `primary_stack`, `discipline`, `language_requirement`, `qualification_required`. `years_experience_required` stays one bucket.

An asserted code needs a quote. The quote is the span in the vacancy (`Bangalore`, `25 LPA`, `authorized to work in the United States`), not the code. These absence values may omit the quote: hiring scope `open`, authorization `none`, timezone `open`, qualification `none`, language `english_only`, empty office, empty currency. Any other value with a missing or non-matching quote becomes `unknown`. A country code that is not ISO 3166 alpha-2 is dropped. A currency that is not ISO 4217 is dropped. A region or a timezone zone outside the enum is dropped. If a restricted answer loses every code, the answer becomes `unknown`.

`unknown` is an answer. An empty office is a different answer: the model looked and found no place. Pending is neither: the verdict row for the current hash does not exist yet.

**Lifecycle:** the profile run scores with phrases immediately. Jobs with no current verdict go to a batch. When the row is written, those jobs are rescored. `user_status` stays as it was.

Statuses on the job payload: **`off`** (enrich disabled) → **`pending`** (enabled, no row for this hash) → **`ready`**.

## Answers

| Answer | Shape | Values |
| ------ | ----- | ------ |
| `office` | countries and regions | ISO country, region enum below |
| `hiring_scope` | one scope, plus countries and regions when `restricted` | `open`, `restricted` |
| `work_authorization` | one value, plus countries when `required` | `none`, `required` |
| `timezone` | one scope, plus zones when `restricted` | `open`, `restricted` |
| `compensation` | currency, min, max, period | ISO 4217, period `year` `month` `week` `day` `hour` |
| `work_mode` | one value | `remote_full`, `hybrid`, `onsite` |
| `role_function` | pool | frontend, fullstack, backend, devops, data, ml, security, qa, mobile, non_engineering, management |
| `primary_stack` | pool | vue, react, angular, node, go, php, java, dotnet, python, other |
| `discipline` | pool | architecture, landscape, urban, interior, structural, mep, bim |
| `qualification_required` | pool | none, riba_part2, riba_part3, arb, ncarb, ro_oar |
| `language_requirement` | pool | english_only, german_fluent, romanian_required, other |
| `years_experience_required` | one value | `0-1`, `2-4`, `5+` |

Regions: `WORLDWIDE`, `EUROPE`, `EMEA`, `AMERICAS`, `LATAM`, `APAC`, `AFRICA`, `MENA`.

Timezone zones: `CET`, `GMT`, `EET`, `US_EASTERN`, `US_CENTRAL`, `US_MOUNTAIN`, `US_PACIFIC`, `IST`, `BRT`, `AMERICAS`, `APAC`.

`primary_stack` is extracted and shown. It does not replace `signals_plus` or `foreign_stack` in this slice. `Vue.js` and `Golang` are the model's job to map onto `vue` and `go`.

Years are one bucket. The prompt maps title words into that bucket even when there is no digit: junior / intern / entry → `0-1`, senior / staff / principal → `5+`. The quote is the word. A missing number and no such word is `unknown`, not `0-1`.

## How a profile reads the answers

Both current profiles can work from `EUROPE` (the country set already used by the scorer, GB included). That residence is not the office allow-list. Andrew's office allow-list is his current `allow_countries`, including `US`, plus `EUROPE`, `EMEA`, `WORLDWIDE`. Lada's is the same list without `US`. Allowed currencies for both: `EUR`, `USD`, `GBP`. Allowed timezone zones for both: `CET`, `GMT`, `EET`.

Once the answer is scored, a profile cuts like this. The model never sets the bucket.

| Answer | Cut |
| ------ | --- |
| Office has a country or region outside that profile's allow-list | `GATE_HARD` |
| `hiring_scope` is `restricted` and none of its countries or regions intersect `EUROPE` | `GATE_HARD` |
| `work_authorization` is `required` | `GATE_HARD` |
| `timezone` is `restricted` and no zone is `CET`, `GMT`, or `EET` | `GATE_HARD` |
| Currency is present and is not `EUR`, `USD`, or `GBP` | `GATE_HARD` |
| Office empty, currency empty, scope `open`, timezone `open`, authorization `none` | no cut from these answers |
| Answer `unknown` | no cut from that answer |

`FR` intersects `EUROPE`, so "must be based in France" is not a scope cut. A right-to-work sentence on the same posting still cuts, because authorization is a separate answer. `US` does not intersect `EUROPE`, so a US-only hiring scope cuts even though Andrew allows a US office. `IN` and `LATAM` cut on the office rule or the scope rule. `INR` cuts on the currency rule even when no country was named.

| Text | Answers | Andrew | Lada |
| ---- | ------- | ------ | ---- |
| Remote, United States, nothing else | office `US`, scope `open` | keep | cut |
| Remote, work from anywhere, US company | office `US`, scope `open` | keep | cut |
| Authorized to work in the US | office `US`, authorization `required` | cut | cut |
| Remote, US Eastern hours | office `US`, timezone `US_EASTERN` | cut | cut |
| France, remote, hire from anywhere | office `FR`, scope `open` | keep | keep |
| Right to work in France | office `FR`, authorization `required` | cut | cut |
| Must be based in France | scope `restricted` `FR` | keep on scope | keep on scope |
| Remote, Bangalore | office `IN` | cut | cut |
| LATAM only | scope `restricted` `LATAM` | cut | cut |
| Remote, 25 LPA | currency `INR` | cut | cut |

A profile added later is a YAML file over the same verdict. It does not call the model again. A profile that wants India allows `IN` and `INR` in its own YAML.

## Runtime

**Start:** `ProfileRunWorkflow` scores the window, then starts `EnrichBatchWorkflow` for job ids that have no row for the current `(prompt_version, model, input_hash)`. The child is abandoned when the parent finishes. Scoring has already completed.

**Backfill:** the same workflow. Not a second code path. Which vacancies go into the first batch is a decision to make first: [To decide](#to-decide).

**Rescore:** a stored verdict triggers `Evaluate` again for matches of that job. Phrase signals and fact signals are recomputed. `user_status` is not written.

**Weight edit:** rescore, no model call.

**Prompt edit:** bump `PromptVersion` in Go. Old rows stay. The hash miss treats them as pending until the new batch lands.

**Vacancy text change:** hash miss, same as a prompt bump. A change to the raw location or the raw salary is a text change.

**Enrich disabled** (no API key): nothing is submitted, `facts_status` is `off`, scoring is phrases only.

### One vacancy

1. Normalize title, description, raw location, and raw salary. Hash them.
2. A row for this version, model, and hash is reused.
3. Otherwise the batch sends the delimited text and receives one JSON object.
4. Schema check, code check, quote check. Failures coerce that value to `unknown` or drop the bad code. They do not invent a bucket.
5. Store the verdict.
6. Rescore matches. A field with `scored: false` still emits a signal, with delta 0. Profile cuts are ignored for that field.

**Failure:** a transport or batch error is a Temporal retry. No verdict row is written, so the job stays `pending`. A model reply that fails schema validation is stored as every answer `unknown` only after the retries for a malformed body are exhausted. An empty row must not look like a successful abstention on the first glitch. `unknown` from a valid reply is stored as-is.

## Scoring rules

| Situation | Behavior |
| --------- | -------- |
| Enrich off | phrase scoring only, `facts_status = off` |
| No row for the current hash | `pending`, no fact signals |
| Field `scored: false` | signal visible, delta 0, profile cuts ignored |
| Scored answer that hits a cut in the table above | `GATE_HARD`, bucket REJECTED |
| Scored answer that does not hit a cut | `LLM` signal, delta from YAML or 0 |
| Empty office, empty currency, `open`, `none` | no signal for that answer |
| Answer `unknown` | no signal for that answer |
| Phrase that this answer replaces fired, and the verdict still matches the allow side | `CONFLICT`, delta 0, phrase gate still rejects |
| Two facts and a phrase both active for the same wording | the field is not turned on until those phrases are removed in the same change |

`CONFLICT` is the list of phrase rules that disagree with a verdict. It is emitted only for phrases explicitly mapped to a field. Unmapped phrases do not emit it. Seniority phrases (`junior`, `senior`, `lead`, `principal`, and the same kind) map to `years_experience_required`, not to `role_function` or `discipline`.

Turning a field on and deleting the phrases it replaces is one change.

| Field, once scored | Phrases removed in that same change |
| --- | --- |
| `office`, `hiring_scope` | `ambiguous_us`, the location lines inside `office_pressure` |
| `work_authorization` | right-to-work, "authorized to work", and "no visa sponsorship" lines |
| `timezone` | `gate_location.reject_timezones` |
| `compensation` | none; the currency gate is new. EUR and USD floors in `signals_plus.salary` stay |
| `work_mode` | hybrid / onsite lines inside `office_pressure` |
| `role_function` | andrew `gate_role` lines mapped to a function |
| `discipline` | lada `gate_role` lines mapped to a discipline |
| `years_experience_required` | seniority lines on both profiles, lada `mid_level`, the architecture years gate in the scorer |
| `qualification_required` | lada `gate_hard` licence lines |
| `language_requirement` | lada `gate_hard` language lines |
| `primary_stack` | none in this slice |

The first fields to turn on, after `make eval`, are `office`, `hiring_scope`, `work_authorization`, `timezone`, and `compensation`. Those are the cuts. Role and stack stay at delta 0 until their own eval.

## Quote check

Normalize both the vacancy and the quote: Unicode NFC, newlines to spaces, collapse repeated whitespace, trim. Compare case-sensitively. The vacancy text for the check is title, description, raw location, and raw salary. A quote that is not a substring fails closed to `unknown` for that value. Absence values skip the check.

## Gold-set

`make eval` reads labeled vacancies and stored (or fixture) predictions. It prints, per answer:

- accuracy where the label is not `unknown`
- share of predicted `unknown`
- share of confident errors (a value was predicted, the label differs or is `unknown`)

A model that answers `unknown` for everything is not a pass. The sample includes the rows in the table above, jobs that survived scoring, jobs `GATE_ROLE` rejected, and the existing traps (`solutions engineer`, `java` beside javascript, `Landschaftsarchitekt`, a "part 3" that is not RIBA).

A field's `scored` flag is flipped only after that printout. This slice ships the flag as `false` for every field.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/enrich`** | contract, questions, batch call, verdict storage, `EnrichBatchWorkflow` |
| **`internal/scoring`** | reads a ready verdict inside `Evaluate`; emits `LLM` and `CONFLICT`; applies profile cuts when the field is scored |
| **`internal/profiles`** | parses the `facts` block |
| **`internal/publicapi`** | adds `facts_status` and signal `value` on the existing job payload |
| **`cmd/worker`** | registers the workflow |

`internal/scoring/impl` does not import the Anthropic SDK or the Temporal SDK.

## Layers

- **Saved** — verdict rows, `PromptVersion`, the questions, few-shots.
- **Run** — batch id and which job ids were submitted. The profile run row is unchanged.
- **Wire** — model JSON in `internal/enrich/schema`. Profile job JSON gains `facts_status` and an optional signal `value`. The quote is the existing `phrase`.

## Implementation order

1. Fold the Latin diacritics the matcher still misses (`äöüß` → `aous`, `ß` → `ss`). Romanian fold already exists. This is phrase matching, independent of the model.
2. Verdict table, questions, batch workflow, quote check.
3. Scoring reads verdicts with every field `scored: false`.
4. `make eval` and a small fixture. The labeled vacancies are manual and are not a code task.
5. After a measured field, set `scored: true` and remove the mapped phrases in that same change. Not in the first implementation pass. The first flip is the five cut answers, not role or stack.

Early check: one fixture vacancy runs through quote coercion and `Evaluate`, produces an `LLM` signal at delta 0, and leaves the bucket on the phrase result.

## To decide

Think this through, then pick one side before the first batch. The catalog is about 4k vacancies.

| Set | What happens |
| --- | --- |
| Every raw vacancy, then bucket | One normalized row per text. Top, review, and reject are decided afterwards from those answers. A later profile rescores the same rows and does not call the model. The first pass is the whole catalog. Only a new or changed text goes again. |
| Reject with today's phrases first, then model only `top` and `review` | The batch is smaller. The rows exist only for what the current profile phrases already kept. A job those phrases rejected never gets answers, including one they rejected by mistake. The next profile, with other phrases, has no row for the rest of the catalog and pays for its own pass. |

## Out of scope

- Replacing stack weights with `primary_stack`.
- A city, currency, or timezone phrase dictionary. The model emits the code.
- A numeric accuracy bar. The printout is the input to flipping `scored`.
- Choosing the bucket inside the prompt.
- Giving the model the collector's parsed country codes.
