# Vacancy facts — schemas and contracts

> Implementation contracts for [enrich-concept.md](enrich-concept.md).
> Frontend: `jobhound_frontend` `app/types/IProfiles.ts` and the profile job table. No frontend feature folder.

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/enrich` | — | `job_verdicts` | `EnrichBatchWorkflow` |
| `internal/scoring` | — | reads `job_verdicts` while scoring | existing `ProfileRunWorkflow` starts the child |
| `internal/profiles` | — | YAML `facts` block | — |
| `internal/publicapi` | existing `GET /api/v1/profiles/{id}/jobs` | — | — |

Scoring stays the owner of buckets. Enrich does not import `internal/scoring`.

---

## Types (`internal/enrich/schema`)

Enums follow the Canonical Enum Pattern.

```go
const PromptVersion = 1

type FactsStatus string // off | pending | ready

type FieldName string
// role_function, primary_stack, discipline, qualification_required,
// language_requirement, work_mode, work_country, sponsorship,
// years_experience_required

type RoleFunction string
// frontend, fullstack, backend, devops, data, ml, security, qa, mobile,
// non_engineering, management

type PrimaryStack string
// vue, react, angular, node, go, php, java, dotnet, python, other

type Discipline string
// architecture, landscape, urban, interior, structural, mep, bim

type Qualification string
// none, riba_part2, riba_part3, arb, ncarb, ro_oar

type LanguageRequirement string
// english_only, german_fluent, romanian_required, other

type WorkMode string
// remote_full, hybrid, onsite

type WorkCountry string
// eu_ok, us_only, country_specific

type Sponsorship string
// no_sponsorship

type YearsExperience string
// 0-1, 2-4, 5+

// QuotedValue is one model answer. Quote is nil for an allowed absence
// and for unknown.
type QuotedValue struct {
	Value string  `json:"value"`
	Quote *string `json:"quote"`
}

// ModelReply is the batch JSON. Pool fields are arrays. Single-value
// fields are one object. Unknown is the string "unknown".
type ModelReply struct {
	RoleFunction            []QuotedValue `json:"role_function"`
	PrimaryStack            []QuotedValue `json:"primary_stack"`
	Discipline              []QuotedValue `json:"discipline"`
	QualificationRequired   []QuotedValue `json:"qualification_required"`
	LanguageRequirement     []QuotedValue `json:"language_requirement"`
	WorkMode                QuotedValue   `json:"work_mode"`
	WorkCountry             QuotedValue   `json:"work_country"`
	Sponsorship             QuotedValue   `json:"sponsorship"`
	YearsExperienceRequired QuotedValue   `json:"years_experience_required"`
}

type Verdict struct {
	JobID         string
	PromptVersion int
	Model         string
	InputHash     string
	Facts         ModelReply
	CreatedAt     time.Time
}
```

`unknown` is not a member of the domain enums. It is a stored answer meaning abstention. Parsers accept it before the enum parse.

Absence values, quote optional:

- `qualification_required`: `none`
- `language_requirement`: `english_only`

Pool fields: `role_function`, `primary_stack`, `discipline`, `qualification_required`, `language_requirement`. The other four are single-value. A single-value field with two different non-unknown values is stored as `unknown`.

`english_only` together with another language drops `english_only`. `none` together with another qualification drops `none`.

Enum descriptions and few-shots live in `internal/enrich` next to the prompt, not in YAML.

### Phrase map

Go holds which current phrases a field replaces. A phrase not in the map never emits `CONFLICT`. Seniority phrases map to `years_experience_required` only.

```go
type PhraseMap struct {
	Field   FieldName
	Phrases []string
}
```

---

## Profile YAML (`internal/profiles/schema`)

```yaml
facts:
  role_function:
    scored: false
    allow: [frontend, fullstack, backend]
    deltas:
      devops: -4
    gates: []
```

`scored` defaults to false when the block or the field is omitted. Omitted delta means 0. `gates` is ignored while `scored` is false. This slice ships both profile files with every field `scored: false`. Allow-pools and numeric deltas may be filled; they do not change buckets until `scored` is true.

`primary_stack.scored` stays false after later fields turn on. Stack weights stay in `signals_plus` and `penalties_soft`.

---

## Contract (`internal/enrich`)

```go
type API interface {
	SubmitMissing(ctx context.Context, jobIDs []string) error
	Verdict(ctx context.Context, jobID, model string, promptVersion int, inputHash string) (schema.Verdict, error)
}

var (
	ErrNotFound = errors.New("verdict not found")
)
```

`SubmitMissing` skips ids that already have a row. `Verdict` returns `ErrNotFound` when the caller should treat the job as pending.

No Temporal SDK imports in `impl/`.

---

## Storage

| Table / column | Role |
| -------------- | ---- |
| `job_verdicts` | one ready reply |
| `job_id` | FK to `jobs.id`, cascade delete |
| `prompt_version` | Go `PromptVersion` at write time |
| `model` | the model string used |
| `input_hash` | hex SHA-256 |
| `facts` | JSONB `ModelReply` after coercion |
| `created_at` | write time |

Unique `(job_id, prompt_version, model, input_hash)`. No status column. Pending is the absence of a row.

**Migration:** `migrations/000002_job_verdicts.up.sql` and `.down.sql`.

**Hash input:** NFC title, a single `\n`, NFC description, then the same whitespace collapse as the quote check. Location, salary, and URL are not included.

---

## Model JSON

Embedded schema: `internal/enrich/handlers` is not used. The schema file lives in `internal/enrich/schema/json_schema/model_reply.schema.json` and is validated in `impl` before coercion. `DisallowUnknownFields` on the decoder.

```json
{
  "role_function": [{ "value": "frontend", "quote": "Frontend Engineer" }],
  "primary_stack": [],
  "discipline": [],
  "qualification_required": [{ "value": "none", "quote": null }],
  "language_requirement": [{ "value": "english_only", "quote": null }],
  "work_mode": { "value": "unknown", "quote": null },
  "work_country": { "value": "country_specific", "quote": "must be based in Germany" },
  "sponsorship": { "value": "no_sponsorship", "quote": "no visa sponsorship" },
  "years_experience_required": { "value": "0-1", "quote": "Junior" }
}
```

The prompt wraps title and description in delimiters and tells the model the block is data.

Quote check: NFC, newlines to spaces, collapse whitespace, trim, case-sensitive substring of the normalized title + description. Failure stores `unknown` for that value and clears the quote.

---

## Scoring signals

Existing `internal/scoring/schema.Signal` gains an optional value. Existing codes stay.

| Code | When | Delta |
| ---- | ---- | ----- |
| `LLM` | one coerced value on a ready verdict, value is not `unknown`, pool not empty | YAML delta, or 0 when `scored` is false |
| `CONFLICT` | a mapped phrase fired and the verdict's allow-pool still intersects | 0 |
| `GATE_HARD` | scored field, value listed in `gates` | 0, bucket REJECTED via the existing gate rule |

`rule` is the field name (`role_function`). `phrase` is the quote. `value` is the enum string. A gate signal uses `rule` `facts.<field>` and `phrase` the quote.

`Evaluate` takes the ready verdict as an argument. It does not query storage. The score activity loads verdicts and passes them in. A missing verdict passes a pending marker, which adds no fact signals.

Rescore updates `bucket`, `score`, `signals`, `run_id`, `updated_at`. It does not update `user_status`.

---

## HTTP — existing job payload

`GET /api/v1/profiles/{profile_id}/jobs` and the PATCH 200 body gain:

```json
{
  "facts_status": "pending",
  "signals": [
    {
      "code": "LLM",
      "rule": "role_function",
      "value": "frontend",
      "phrase": "Frontend Engineer",
      "delta": 0
    }
  ]
}
```

`facts_status`: `off` | `pending` | `ready`.

`value` is omitted on phrase signals. Frontend `IProfileSignal` gains optional `value` and the two new codes. `IProfileJob` gains `facts_status`.

No new error codes.

---

## Temporal

| Kind | Name | Module |
|------|------|--------|
| Workflow | `EnrichBatchWorkflow` | `internal/enrich/workflows` |
| Activity | `SubmitBatch`, `PollBatch`, `StoreVerdicts`, `RescoreJobs` | `internal/enrich/workflows/activities` |

`New` registers `EnrichBatchWorkflow`.

**Start:** child of `ProfileRunWorkflow`, parent close policy abandon. Workflow id `enrich-{promptVersion}-{model}-{hash of the sorted job ids}` so a duplicate start is the same run.

**Payload:**

```go
type EnrichBatchInput struct {
	JobIDs        []string
	PromptVersion int
	Model         string
}
```

The activity re-reads job text from storage. The payload does not carry descriptions.

`RescoreJobs` loads each profile that has a match for the job and calls the scoring service. It does not duplicate `Evaluate`.

---

## Config

| Env | Meaning | Default |
| --- | ------- | ------- |
| `JOBHOUND_ANTHROPIC_API_KEY` | batch API key | empty → enrich off |
| `JOBHOUND_ENRICH_MODEL` | model string stored on the row | required when the key is set; empty key ignores it |

Names and parsing in `internal/config` only. `PromptVersion` is not an env var.

---

## Eval

`make eval` compares `testdata/enrich/gold.jsonl` with `testdata/enrich/predictions.jsonl`. Both lines are `{ "id", "facts": ModelReply }`. The command prints the three metrics per field and exits 0. It does not call the network. It does not fail the build on a low score.

A tiny fixture is enough for the test. The 100 labeled vacancies are added by hand later.

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Impl | `internal/enrich/impl/*_test.go` | quote coercion, pool vs single-value, absence quotes, eval metrics |
| Storage | `internal/enrich/storage/*_test.go` | unique key, `ErrNotFound` |
| Scoring impl | `internal/scoring/impl/evaluate_test.go` | delta 0 while unscored, gate ignored, `CONFLICT` only for a mapped phrase, seniority phrase does not conflict with `role_function` |
| Public API | `internal/publicapi/handlers/get_profile_jobs_test.go` | `facts_status` and `value` on the JSON |
| Workflow | `internal/scoring/workflows/profile_run_test.go` | parent finishes without waiting; child id is stable |

Diacritic fold is covered through the existing scorer tests, not a standalone utils test.
