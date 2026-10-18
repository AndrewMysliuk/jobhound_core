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

Scoring stays the owner of buckets. Enrich does not import `internal/scoring`. The model reply is the normalized record. Profile YAML is the only place that turns an answer into a cut.

---

## Types (`internal/enrich/schema`)

Enums follow the Canonical Enum Pattern. Country codes and currency codes are not enums: they are checked strings, ISO 3166-1 alpha-2 and ISO 4217. A city name never appears as a stored value.

```go
const PromptVersion = 1

type FactsStatus string // off | pending | ready

type FieldName string
// office, hiring_scope, work_authorization, timezone, compensation,
// work_mode, role_function, primary_stack, discipline,
// qualification_required, language_requirement, years_experience_required

type Region string
// WORLDWIDE, EUROPE, EMEA, AMERICAS, LATAM, APAC, AFRICA, MENA

type TimezoneZone string
// CET, GMT, EET, US_EASTERN, US_CENTRAL, US_MOUNTAIN, US_PACIFIC,
// IST, BRT, AMERICAS, APAC

type Scope string          // open | restricted
type Authorization string  // none | required
type WorkMode string       // remote_full | hybrid | onsite
type PayPeriod string      // year | month | week | day | hour

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

type YearsExperience string
// 0-1, 2-4, 5+

// QuotedValue is one role answer. Quote is nil for an allowed absence
// and for unknown.
type QuotedValue struct {
	Value string  `json:"value"`
	Quote *string `json:"quote"`
}

// Office is where the company is. Empty countries and regions, with
// Unknown false, means the model found no place. Unknown true means
// abstention. A location badge is an office, not a hiring scope.
type Office struct {
	Countries []string `json:"countries"`
	Regions   []string `json:"regions"`
	Quote     *string  `json:"quote"`
	Unknown   bool     `json:"unknown"`
}

// HiringScope is where the person must be. Open covers "anywhere" and
// "not stated". Countries and regions are set only when Value is restricted.
type HiringScope struct {
	Value     string   `json:"value"`
	Countries []string `json:"countries"`
	Regions   []string `json:"regions"`
	Quote     *string  `json:"quote"`
}

// WorkAuthorization is an existing right-to-work requirement.
// Countries may be empty when the text requires it and names no country.
type WorkAuthorization struct {
	Value     string   `json:"value"`
	Countries []string `json:"countries"`
	Quote     *string  `json:"quote"`
}

type TimezoneConstraint struct {
	Value string   `json:"value"`
	Zones []string `json:"zones"`
	Quote *string  `json:"quote"`
}

// Compensation is the offered pay. Empty Currency means no pay was stated.
// A currency does not fill Office.
type Compensation struct {
	Currency string   `json:"currency"`
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Period   string   `json:"period"`
	Quote    *string  `json:"quote"`
}

// ModelReply is the batch JSON. Unknown on a scope or a quoted value is
// the string "unknown". Unknown on an office is the bool.
type ModelReply struct {
	Office                  Office            `json:"office"`
	HiringScope             HiringScope       `json:"hiring_scope"`
	WorkAuthorization       WorkAuthorization `json:"work_authorization"`
	Timezone                TimezoneConstraint `json:"timezone"`
	Compensation            Compensation      `json:"compensation"`
	WorkMode                QuotedValue       `json:"work_mode"`
	RoleFunction            []QuotedValue     `json:"role_function"`
	PrimaryStack            []QuotedValue     `json:"primary_stack"`
	Discipline              []QuotedValue     `json:"discipline"`
	QualificationRequired   []QuotedValue     `json:"qualification_required"`
	LanguageRequirement     []QuotedValue     `json:"language_requirement"`
	YearsExperienceRequired QuotedValue       `json:"years_experience_required"`
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

`unknown` is not a member of the domain enums. Parsers accept it before the enum parse.

Absence values, quote optional:

- `hiring_scope`: `open`
- `work_authorization`: `none`
- `timezone`: `open`
- `qualification_required`: `none`
- `language_requirement`: `english_only`
- `office`: empty codes, `unknown: false`
- `compensation`: empty `currency`

An asserted code needs a quote. The quote is the vacancy span, not the code. A non-ISO country or currency is dropped. A region or zone outside the enum is dropped. `restricted` or `required` that loses every code and has no remaining country becomes `unknown`, except authorization, which may stay `required` with an empty country list when the quote is valid and names no country. `english_only` together with another language drops `english_only`. `none` together with another qualification drops `none`.

Enum descriptions and few-shots live in `internal/enrich` next to the prompt, not in YAML. The prompt states the six questions from the concept, including: a currency does not imply a country, and a location badge is the office unless the text restricts the person.

### Phrase map

Go holds which current phrases a field replaces. A phrase not in the map never emits `CONFLICT`. Seniority phrases map to `years_experience_required` only.

```go
type PhraseMap struct {
	Field   FieldName
	Phrases []string
}
```

`office` and `hiring_scope` replace `ambiguous_us` and the location lines in `office_pressure`. `work_authorization` replaces the right-to-work, "authorized to work", and "no visa sponsorship" lines. `timezone` replaces `gate_location.reject_timezones`. `compensation` replaces nothing. `primary_stack` replaces nothing in this slice.

---

## Profile YAML (`internal/profiles/schema`)

```yaml
facts:
  residence_regions: [EUROPE]
  office:
    scored: false
    allow_countries: [US, DE, FR, RO, GB]
    allow_regions: [EUROPE, EMEA, WORLDWIDE]
  hiring_scope:
    scored: false
  work_authorization:
    scored: false
    gates: [required]
  timezone:
    scored: false
    allow: [CET, GMT, EET]
  compensation:
    scored: false
    allow_currencies: [EUR, USD, GBP]
  role_function:
    scored: false
    allow: [frontend, fullstack, backend]
    deltas:
      devops: -4
    gates: []
```

`residence_regions` expands through the scorer's existing Europe country set, GB included. It is not the office allow-list. Andrew's `office.allow_countries` is his current `allow_countries`, including `US`. Lada's is that list without `US`. Both use the currency and timezone allows above.

`scored` defaults to false when the block or the field is omitted. Omitted delta means 0. Cuts are ignored while `scored` is false. This slice ships both profile files with every field `scored: false`.

`primary_stack.scored` stays false after the location and pay fields turn on. Stack weights stay in `signals_plus` and `penalties_soft`.

A later profile that wants India adds `IN` to `office.allow_countries` and `INR` to `allow_currencies`. That is a YAML edit and a rescore. No new model call.

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

**Hash input:** NFC title, `\n`, NFC description, `\n`, NFC raw location, `\n`, NFC raw salary, then the same whitespace collapse as the quote check. The collector's parsed country codes and the URL are not included.

---

## Model JSON

Embedded schema: `internal/enrich/handlers` is not used. The schema file lives in `internal/enrich/schema/json_schema/model_reply.schema.json` and is validated in `impl` before coercion. `DisallowUnknownFields` on the decoder.

```json
{
  "office": {
    "countries": ["IN"],
    "regions": [],
    "quote": "Bangalore",
    "unknown": false
  },
  "hiring_scope": { "value": "open", "countries": [], "regions": [], "quote": null },
  "work_authorization": { "value": "none", "countries": [], "quote": null },
  "timezone": { "value": "open", "zones": [], "quote": null },
  "compensation": {
    "currency": "INR",
    "min": 2500000,
    "max": null,
    "period": "year",
    "quote": "25 LPA"
  },
  "work_mode": { "value": "remote_full", "quote": "Remote" },
  "role_function": [{ "value": "frontend", "quote": "Frontend Engineer" }],
  "primary_stack": [{ "value": "vue", "quote": "Vue.js" }],
  "discipline": [],
  "qualification_required": [{ "value": "none", "quote": null }],
  "language_requirement": [{ "value": "english_only", "quote": null }],
  "years_experience_required": { "value": "unknown", "quote": null }
}
```

The prompt wraps title, description, raw location, and raw salary in delimiters and tells the model the block is data.

Quote check: NFC, newlines to spaces, collapse whitespace, trim, case-sensitive substring of the normalized title + description + raw location + raw salary. Failure stores `unknown` for that value and clears the quote. On `office`, failure sets `unknown` true and clears the codes.

---

## Scoring signals

Existing `internal/scoring/schema.Signal` gains an optional value. Existing codes stay.

| Code | When | Delta |
| ---- | ---- | ----- |
| `LLM` | one coerced code or scope on a ready verdict, not `unknown`, not an absence | YAML delta, or 0 when `scored` is false |
| `CONFLICT` | a mapped phrase fired and the verdict still matches the allow side | 0 |
| `GATE_HARD` | scored answer hits a profile cut | 0, bucket REJECTED via the existing gate rule |

Cuts, applied only when that field is `scored`:

| Answer | Cut |
| ------ | --- |
| Each office country or region outside `allow_countries` / `allow_regions` | one `GATE_HARD`, `value` is that code |
| `hiring_scope` `restricted` and no country or region intersects `residence_regions` | `GATE_HARD` |
| `work_authorization` value listed in `gates` (`required`) | `GATE_HARD` |
| `timezone` `restricted` and no zone is in `allow` | `GATE_HARD` |
| `compensation.currency` set and not in `allow_currencies` | `GATE_HARD`, `value` is the currency |

`FR` intersects residence `EUROPE`. `US` does not, so a restricted US scope cuts even when `US` is an allowed office. An INR compensation does not write an `IN` office.

`rule` is the field name (`office`). `phrase` is the quote. `value` is the code. A gate signal uses `rule` `facts.<field>` and `phrase` the quote.

`Evaluate` takes the ready verdict as an argument. It does not query storage. The score activity loads verdicts and passes them in. A missing verdict passes a pending marker, which adds no fact signals.

Rescore updates `bucket`, `score`, `signals`, `run_id`, `updated_at`. It does not update `user_status`.

---

## HTTP — existing job payload

`GET /api/v1/profiles/{profile_id}/jobs` and the PATCH 200 body gain:

```json
{
  "facts_status": "ready",
  "signals": [
    {
      "code": "GATE_HARD",
      "rule": "facts.compensation",
      "value": "INR",
      "phrase": "25 LPA",
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

The activity re-reads title, description, raw location, and raw salary from storage. The payload does not carry the text.

`RescoreJobs` loads each profile that has a match for the job and calls the scoring service. It does not duplicate `Evaluate`.

---

## Config

| Env | Meaning | Default |
| --- | ------- | ------- |
| `JOBHOUND_ANTHROPIC_API_KEY` | batch API key | empty → enrich off |
| `JOBHOUND_ENRICH_MODEL` | model string stored on the row. Haiku. | required when the key is set; empty key ignores it |

Names and parsing in `internal/config` only. `PromptVersion` is not an env var.

---

## Eval

`make eval` compares `testdata/enrich/gold.jsonl` with `testdata/enrich/predictions.jsonl`. Both lines are `{ "id", "facts": ModelReply }`. The command prints the three metrics per field and exits 0. It does not call the network. It does not fail the build on a low score.

The fixture covers the concept table: a US office with scope `open`, a US authorization, a US-only scope, a France office with scope `open`, Bangalore → `IN`, LATAM → region `LATAM`, `25 LPA` → `INR` with no country added.

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Impl | `internal/enrich/impl/*_test.go` | quote coercion, ISO country and currency drop, currency does not fill office, `restricted` with no remaining code becomes `unknown`, authorization `required` with a valid quote and no country stays `required`, absence quotes |
| Storage | `internal/enrich/storage/*_test.go` | unique key, `ErrNotFound` |
| Scoring impl | `internal/scoring/impl/evaluate_test.go` | delta 0 and cuts ignored while unscored; scored `INR` gates; scored office `US` gates only when `US` is outside the profile allow; restricted `US` gates against residence `EUROPE`; restricted `FR` does not; `CONFLICT` only for a mapped phrase; seniority does not conflict with `role_function` |
| Public API | `internal/publicapi/handlers/get_profile_jobs_test.go` | `facts_status` and `value` on the JSON |
| Workflow | `internal/scoring/workflows/profile_run_test.go` | parent finishes without waiting; child id is stable |

Diacritic fold is covered through the existing scorer tests, not a standalone utils test.
