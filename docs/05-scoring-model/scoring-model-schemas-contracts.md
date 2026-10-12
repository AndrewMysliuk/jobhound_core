# Gated scoring model — schemas and contracts

> Concept: [`scoring-model-concept.md`](./scoring-model-concept.md).
> Frontend side of this slice is one type file and one page; it is listed under *Frontend wire types* instead of a separate document.

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/profiles` | — | — (YAML on disk) | — |
| `internal/scoring` | — | `profile_matches` | `ProfileRunWorkflow` (unchanged) |
| `internal/publicapi` | `/api/v1/profiles/{profile_id}/jobs*` | — | — |

`internal/ingest` and `internal/collectors` — **outside this slice**. They keep reading `Profile.Sources`, `Profile.Queries` and `Profile.WellfoundRoles`, which this slice does not touch. The plus-signal terms are a separate block and are never passed to a collector.

---

## Types (`internal/profiles/schema`)

The profile is the only source of weights, caps, thresholds and the currency rate. Nothing in this list has a code-level default.

```go
type Profile struct {
	ID             string
	Name           string
	Domain         Domain
	Sources        []string
	Queries        []string
	WellfoundRoles []string
	GateRole       []string
	GateHard       []string
	GateLocation   GateLocation
	SignalsPlus    SignalsPlus
	PenaltiesSoft  []PenaltyGroup
	Buckets        Buckets
}

type GateLocation struct {
	AllowCountries  []string
	AllowRegions    []string
	RejectTimezones []string
}

type SignalsPlus struct {
	Title     map[string]int
	Text      TextSignals
	Salary    SalarySignals
	Freshness FreshnessSignals
}

type TextSignals struct {
	Terms    map[string]int
	MaxTotal int
}

type SalarySignals struct {
	MinEURYear     int
	MinUSDYear     int
	USDPerEUR      float64
	BonusIfAbove   int
	PenaltyIfBelow int
}

type FreshnessSignals struct {
	Days3 int
	Days7 int
}

type PenaltyGroup struct {
	Name           string
	Weight         int
	MinOccurrences int
	CheckNegation  bool
	Terms          []string
}

type Buckets struct {
	Top int
}
```

Removed from `Profile`: `LocationCountries`, `LocationRegions` (moved into `GateLocation`), `ExcludeTitle`, `ExcludeText`, `Penalties`.

**`PenaltiesSoft` is a slice, not a map.** The YAML spells the groups as a mapping so each one is named at its definition site; the loader flattens it into a slice sorted by `Name`. That gives `impl/` a deterministic visit order, so two runs over the same vacancy produce an identical signal list, and it carries the group name into the signal without a second lookup.

**`CheckNegation` is per group, not a hardcoded group name.** The negation check is needed by `ambiguous_us` and `part_time`, whose phrases are the ones that appear inside denials. Which groups need it is profile data, so `impl/` never matches on a group name.

`MinOccurrences` of `0` or `1` both mean "one hit is enough".

### YAML mapping (`config/profiles/*.yaml`)

snake_case tags, `KnownFields(true)` stays on, so an unknown key is a load error.

| YAML key | Field |
| -------- | ----- |
| `gate_role` | `GateRole` |
| `gate_hard` | `GateHard` |
| `gate_location.allow_countries` / `.allow_regions` / `.reject_timezones` | `GateLocation` |
| `signals_plus.title` | `SignalsPlus.Title` |
| `signals_plus.text.terms` / `.max_total` | `SignalsPlus.Text` |
| `signals_plus.salary.*` | `SignalsPlus.Salary` |
| `signals_plus.freshness.days_3` / `.days_7` | `SignalsPlus.Freshness` |
| `penalties_soft.<group>.weight` / `.terms` / `.min_occurrences` / `.check_negation` | `PenaltiesSoft[i]` |
| `buckets.top` | `Buckets.Top` |

`signals_plus.text.max_total` sits beside `terms`, not inside it — a term weight and a cap cannot share one mapping. `gate_location` carries no numeric UTC offsets: the vacancy's timezone is free text, so the offset range would be unevaluable configuration.

---

## Types (`internal/scoring/schema`)

```go
type Bucket string

const (
	BucketTop      Bucket = "TOP"
	BucketReview   Bucket = "REVIEW"
	BucketRejected Bucket = "REJECTED"
)

type SignalCode string

const (
	SignalGateRole     SignalCode = "GATE_ROLE"
	SignalGateHard     SignalCode = "GATE_HARD"
	SignalCutLocation  SignalCode = "CUT_LOCATION"
	SignalCutTimezone  SignalCode = "CUT_TIMEZONE"
	SignalCutPostedAt  SignalCode = "CUT_POSTED_AT"
	SignalPlus         SignalCode = "PLUS"
	SignalPenalty      SignalCode = "PENALTY"
)

// Signal is one fired rule with its contribution. Delta is 0 for gates.
type Signal struct {
	Code        SignalCode `json:"code"`
	Rule        string     `json:"rule"`
	Phrase      string     `json:"phrase,omitempty"`
	Delta       int        `json:"delta"`
	Occurrences int        `json:"occurrences,omitempty"`
}
```

`BucketPassed`, `SignalQuery`, `SignalCutExcludeTitle` and `SignalCutExcludeText` are **deleted**, not deprecated. Both enums keep the Canonical Enum Pattern (`String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`).

`Rule` names the rule that fired, so a signal is readable without the profile at hand:

| Code | `Rule` | `Phrase` | `Delta` | `Occurrences` |
| ---- | ------ | -------- | ------- | ------------- |
| `GATE_ROLE` | `gate_role` | matched title phrase | `0` | hits in title |
| `GATE_HARD` | `gate_hard` | matched phrase | `0` | non-negated hits |
| `CUT_LOCATION` | `gate_location.allow_countries` or `.allow_regions` | the vacancy's country or region code | `0` | — |
| `CUT_TIMEZONE` | `gate_location.reject_timezones` | matched phrase | `0` | non-negated hits |
| `CUT_POSTED_AT` | `posted_at` | RFC3339 `posted_at` | `0` | — |
| `PLUS` | `signals_plus.title`, `.text`, `.salary`, `.freshness` | matched term; empty for salary and freshness | awarded points | hits for term signals |
| `PENALTY` | the group name, e.g. `foreign_stack` | the term that triggered the group | the group weight | hits counted for the threshold |

The body cap is visible in the signal list: term signals under `signals_plus.text` are emitted with the points actually awarded after the cap, so the deltas always sum to `Score`.

`ListJobsParams`, `ListedJob` and `JobPage` keep their shapes; only the `Bucket` and `Signal` types inside them change.

### Constants (`internal/scoring/schema/constants.go`)

| Constant | Fate |
| -------- | ---- |
| `ScoreWindowDays` | unchanged |
| `PostedAtRejectDays` | unchanged — the staleness gate stays a module constant, not profile data |
| `QueryPoints` | deleted; points come from `signals_plus` |
| `PenaltyPoints` | deleted; weights come from `penalties_soft` |

---

## Contract

`internal/profiles/contract.go` and `internal/scoring/contract.go` are **unchanged**: `Store`, `API`, `RunStarter` and all four sentinel errors keep their signatures. A malformed profile keeps surfacing as `profiles.ErrProfileInvalid`.

`internal/scoring/impl` exported surface stays `Evaluate(profile, job) Outcome` plus the service methods. `Outcome` keeps `{Bucket, Score, Signals}`. New unexported helpers in `impl/` — negation window, occurrence counting, salary parsing, bucket selection — are not part of the contract.

---

## Storage

| Table / column | Role |
| -------------- | ---- |
| `profile_matches` | one row per profile and job; unchanged shape |
| `profile_matches.bucket` | `TOP` \| `REVIEW` \| `REJECTED`; check constraint rewritten |
| `profile_matches.signals` | JSONB array of the new signal object |
| `profile_matches.score` | now meaningful for rejected rows too |
| `profile_matches_list` index | unchanged — `(profile_id, bucket, user_status, score DESC)` already serves the audit ordering |

**Migration:** the bucket check constraint in `migrations/000001_initial_schema.up.sql` is **edited in place**, from `CHECK (bucket IN ('PASSED', 'REJECTED'))` to `CHECK (bucket IN ('TOP', 'REVIEW', 'REJECTED'))`. No `000002`. The database is recreated from scratch between runs, so there is no deployed row to migrate and no reason to carry a dead `PASSED` value forward. `signals` is schemaless JSONB, so the new object needs no DDL.

**Query change in `internal/scoring/storage`:** `List` drops its "a `PASSED` list omits score 0 and below" rule. A score of zero is a legitimate `REVIEW` row, and a rejected row's score is the audit value. Default bucket inside `List` becomes `TOP`. Ordering stays `score DESC, jobs.first_seen_at DESC`.

---

## HTTP — `internal/publicapi`

No new route, no new JSON Schema file, no change to `registerRoutes()`. Two existing responses change shape, and one query default changes.

### `GET /api/v1/profiles/{profile_id}/jobs`

**Query:** `bucket` now accepts `TOP` | `REVIEW` | `REJECTED`; an unknown value stays `400 HTTP.INVALID_QUERY`. **The default changes from `PASSED` to `TOP`.** `user_status`, `page` and `limit` are unchanged (`DefaultJobListLimit` 50, `MaxJobListLimit` 100).

**Response `200`** — unchanged except `bucket` and every entry of `signals`:

```json
{
  "page": 1,
  "limit": 50,
  "total": 3,
  "jobs": [
    {
      "job_id": "…",
      "bucket": "REJECTED",
      "score": 14,
      "signals": [
        { "code": "GATE_ROLE", "rule": "gate_role", "phrase": "analyst", "delta": 0, "occurrences": 1 },
        { "code": "PLUS", "rule": "signals_plus.title", "phrase": "vue", "delta": 5, "occurrences": 1 },
        { "code": "PENALTY", "rule": "foreign_stack", "phrase": "ruby", "delta": -3, "occurrences": 2 }
      ]
    }
  ]
}
```

`ProfileJobSignal` in `internal/publicapi/schema` becomes `{code, rule, phrase, delta, occurrences}`; `points` and `terms` are removed. All other `ProfileJob` fields keep their names and types.

**Audit use:** `?bucket=REJECTED` with the default ordering returns the highest-scoring rejected vacancies first, which is the false-rejection list. No `random` parameter and no `/jobs/audit` route is added.

### `PATCH /api/v1/profiles/{profile_id}/jobs/{job_id}`

Request body is unchanged. The `200` body is one `ProfileJob`, so it inherits the same `bucket` and `signals` change.

---

## Temporal

| Kind | Name | Module |
|------|------|--------|
| Workflow | `ProfileRunWorkflow` | `internal/scoring/workflows` |

Unchanged: registration, payloads, child ingest workflows, run statuses. The scoring step calls the same service method; only what that method writes into `bucket` and `signals` differs. `impl/` must not import `go.temporal.io/sdk/...`, which the new helpers have no reason to.

---

## Config

No new environment variable. The currency rate, every weight, the body cap and the `TOP` threshold are profile data, so nothing in this slice belongs in `internal/config`.

---

## Errors

| HTTP / sentinel | `code` | When |
| --------------- | ------ | ---- |
| `400` | `HTTP.INVALID_QUERY` | `bucket` outside `TOP` \| `REVIEW` \| `REJECTED`; unchanged handling, new value set |
| `400` | `PROFILES.INVALID_DEFINITION` | YAML has an unknown key, a leftover `exclude_title` / `exclude_text` / `penalties` block, or a block that fails to decode — surfaced as `profiles.ErrProfileInvalid` |
| `404` | `PROFILES.NOT_FOUND` | unchanged |
| `500` | internal | unchanged |

A missing or malformed salary, an empty location, an absent `posted_at` and an unparseable timezone are **not** errors anywhere in this slice. They contribute zero and the pass continues.

---

## Frontend wire types (`jobhound_frontend`)

| Type | Change |
| ---- | ------ |
| `IProfileJobBucket` | `"TOP" \| "REVIEW" \| "REJECTED"` |
| `IProfileSignalCode` | `"GATE_ROLE" \| "GATE_HARD" \| "CUT_LOCATION" \| "CUT_TIMEZONE" \| "CUT_POSTED_AT" \| "PLUS" \| "PENALTY"` |
| `IProfileSignal` | `{ code, rule, phrase?, delta, occurrences? }` |
| `app/pages/profiles/[id].vue` | the list view gains a third bucket tab; the default view sends `bucket=TOP` explicitly |

An unknown hiring location renders as a dash. There is no badge, no extra bucket and no extra signal for it.

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Impl | `internal/scoring/impl/evaluate_test.go` | the negation window, including the sentence-boundary clamp: `not part-time`, `no US authorization required`, `you won't need to relocate`, `non-hybrid` must not count, while `We are not a huge company. Security clearance required.` must still reject |
| Impl | `internal/scoring/impl/evaluate_test.go` | occurrence threshold: one `ruby` in the body scores nothing, two apply the group weight once, `ruby` in the title applies it regardless of count |
| Impl | `internal/scoring/impl/evaluate_test.go` | a vacancy that trips `gate_role` and still scores high is `REJECTED` **with** its score and both signal kinds present |
| Impl | `internal/scoring/impl/evaluate_test.go` | empty countries and regions land in a score-decided bucket, never `REJECTED`; a stale `posted_at` is `REJECTED` with `CUT_POSTED_AT` |
| Impl | `internal/scoring/impl/evaluate_test.go` | the body cap: term deltas sum to `Score`, and salary absent contributes zero rather than the below-minimum penalty |
| Impl | `internal/profiles/impl/store_test.go` | the five blocks decode from YAML; `penalties_soft` arrives sorted by name; a leftover `exclude_title` key fails with `ErrProfileInvalid` |
| Storage | `internal/scoring/storage/repository_test.go` | a `REVIEW` row with score `0` and a negative-score row are both listed |
| Handlers | `internal/publicapi/handlers/get_profile_jobs_test.go` | the default bucket is `TOP`; `bucket=REVIEW` is accepted; `bucket=PASSED` is `400`; the signal projection carries the new fields |

No dedicated tests for the two enums or for the negation helper in isolation — they are covered through `Evaluate` and the handler.
