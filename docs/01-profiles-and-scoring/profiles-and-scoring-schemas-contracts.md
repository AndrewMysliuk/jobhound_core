# Search profiles and rule scoring (v2) — schemas and contracts

> Implementation contracts: types, HTTP, Temporal, storage, env, errors.
> Concept: [`profiles-and-scoring-concept.md`](./profiles-and-scoring-concept.md).

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/profiles` | — | — (YAML files) | — |
| `internal/scoring` | — | `profile_runs`, `profile_matches` | `ProfileRunWorkflow` |
| `internal/jobs` | — | `jobs` | `JobRetentionWorkflow` (window unchanged, column changes) |
| `internal/ingest` | — | `ingest_watermarks` | `IngestSourceWorkflow` (key scope changes; query comes from the profile) |
| `internal/publicapi` | `/api/v1/profiles/*` | — | — |

**Deleted modules:** `internal/llm`, `internal/profile`, `internal/slots`, `internal/manual`, `internal/pipeline/{impl,mock}`. `internal/scoring` is `internal/pipeline` renamed; the phrase-matching helpers in `pipeline/utils` move to `scoring/utils`. There is no `internal/dictionary`. Geo allow-lists (`EUROPE_HIRING_OK`) are not reused.

---

## Profile files

`config/profiles/<id>.yaml`, one file per profile. Unknown keys are an error (strict decode). This file is the only place queries, excludes, country phrases, and penalties exist.

```yaml
id: andrew
name: Andrew
domain: software

sources: [europe_remotely, working_nomads, himalayas, remotify_europe, we_work_remotely, wellfound, vue_jobs, golang_cafe, builtin]

queries: [vue, typescript, react, golang, node, "AI native", "AI engineer", LLM]
wellfound_roles: [frontend-engineer, full-stack-engineer, backend-engineer, artificial-intelligence-engineer]

exclude_title:
  - recruiter
  - junior
exclude_text:
  - php
  - "uk only"
penalties:
  - "work permit required"
  - "must be based in the us"
```

The three phrase lists in the sample show the shape. The real `andrew` file contains every phrase from `jobhound_frontend/app/utils/defaultStage2Exclude.ts`, classified once:

- A phrase that names a role or a title ("recruiter", "product manager", "junior", "staff engineer") goes to `exclude_title`.
- A phrase that locks a country or region other than a blanket USA ban ("uk only", "must be based in germany", "philippines") goes to `exclude_text`, together with stack and domain phrases ("php", "java", "kubernetes").
- A phrase about visa, work permit, work authorization, right to work, citizenship, "must be based", or "must reside" goes to `penalties`, not to either exclude list.
- A phrase that rejects the USA as a whole ("united states only", "us based applicants only", "eastern time", "overlap with us") is **not copied**. Europe and USA stay allowed because they are absent.

`queries` are the old `collectors/schema/queries.go` matrix **without** `frontend` and `full-stack`. `wellfound_roles` are the old Wellfound slugs, copied as URL segments. After `andrew.yaml` exists, delete `defaultStage2Exclude.ts`, the `EUROPE_HIRING_OK` rule in `app/utils/stagePayload.ts`, and `internal/collectors/schema/queries.go`.

`sources` entries must be known collector source ids. `queries` are ignored for catalog sources (`we_work_remotely`, `vue_jobs`, `golang_cafe`); Wellfound uses `wellfound_roles` and does not use `queries`. No `min_score` field. No dictionary keys: every entry is a literal phrase.

`config/profiles/architecture.yaml` is **not created in this slice**. Collectors and per-source slug lists live in [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).

Matching for every phrase list: case-insensitive, word boundaries. `java` does not match `javascript`.

---

## Types (`internal/profiles/schema`)

```go
type Domain string

const (
	DomainSoftware     Domain = "software"
	DomainArchitecture Domain = "architecture"
)

type Profile struct {
	ID             string
	Name           string
	Domain         Domain
	Sources        []string
	Queries        []string
	WellfoundRoles []string
	ExcludeTitle   []string
	ExcludeText    []string
	Penalties      []string
}
```

## Types (`internal/scoring/schema`)

```go
type Bucket string

const (
	BucketPassed   Bucket = "PASSED"
	BucketRejected Bucket = "REJECTED"
)

type UserStatus string

const (
	UserStatusNew    UserStatus = "NEW"
	UserStatusHidden UserStatus = "HIDDEN"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "RUNNING"
	RunStatusSucceeded RunStatus = "SUCCEEDED"
	RunStatusFailed    RunStatus = "FAILED"
)

// SignalCode identifies one scoring hit or one hard cutoff.
type SignalCode string

const (
	SignalQuery          SignalCode = "QUERY"
	SignalPenalty        SignalCode = "PENALTY"
	SignalCutExcludeTitle SignalCode = "CUT_EXCLUDE_TITLE"
	SignalCutExcludeText  SignalCode = "CUT_EXCLUDE_TEXT"
)

// Signal is one fired phrase with its contribution.
type Signal struct {
	Code   SignalCode `json:"code"`
	Points int        `json:"points"`
	Terms  []string   `json:"terms,omitempty"`
}

type Match struct {
	ProfileID  string
	JobID      string
	Bucket     Bucket
	Score      int
	Signals    []Signal
	UserStatus UserStatus
	RunID      int64
	UpdatedAt  time.Time
}
```

Every exported string enum above follows the Canonical Enum Pattern (`String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`).

**Deleted enums:** `RunJobStatus` (all six `*_STAGE_*` values), `Stage2Rule` with `RuleField` / `RuleOp` / `RuleAction` / `RuleWhen`, `manual/schema.RunKind`, everything in `llm/schema`. No `UNKNOWN` bucket.

Named constants in `internal/scoring/schema`, not literals in `impl/`:

- `QueryPoints` = 1
- `PenaltyPoints` = -10
- `ScoreWindowDays` = 10, measured on `first_seen_at` against `profile_runs.started_at`

---

## Contracts

```go
// internal/profiles/contract.go
type Store interface {
	List(ctx context.Context) ([]schema.Profile, error)
	Get(ctx context.Context, id string) (schema.Profile, error)
}

var (
	ErrProfileNotFound = errors.New("profiles: profile not found")
	ErrProfileInvalid  = errors.New("profiles: profile file invalid")
)
```

`impl/file_store.go` reads `config/profiles/*.yaml` on every call. A later `storage/` on Postgres implements the same interface with no change above it.

```go
// internal/scoring/contract.go
type API interface {
	StartRun(ctx context.Context, profileID string, idempotencyKey uuid.UUID) (schema.Run, error)
	LatestRun(ctx context.Context, profileID string) (schema.Run, error)
	ListJobs(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error)
	SetUserStatus(ctx context.Context, profileID, jobID string, s schema.UserStatus) (schema.ListedJob, error)
}

var (
	ErrRunAlreadyRunning = errors.New("scoring: run already in progress")
	ErrJobNotInScope     = errors.New("scoring: job not scored for this profile")
	ErrNoRun             = errors.New("scoring: no run for this profile")
)
```

---

## Storage

### `jobs` (changed)

| Column | Role |
| ------ | ---- |
| `id` | **the dedup key.** `company_key` + `\x1e` + normalized title + `\x1e` + normalized `location.raw`. One row per vacancy, not per board. `apply_url` is not part of the key |
| `sources` | `TEXT[]` — every source id that has shown this vacancy |
| `url`, `apply_url` | canonical listing URL and apply/ATS link; ATS-sourced values win over aggregator values |
| `company`, `company_key` | display name and the normalized key (lowercased, `Inc` / `GmbH` / `Ltd` / `LLC` / `SRL` stripped). `company_key` is the join target for the future `companies` table |
| `company_website` | when a board reports it; empty otherwise |
| `location` | `JSONB` `{ "type": "remote\|hybrid\|office", "regions": [], "countries": [], "timezone": "", "raw": "" }`. Collapses `is_remote`, `country_code`, `hiring_countries`, `hiring_regions`, `hiring_raw`, `timezone_offsets`. Display only. Country policy does not read this object. Only `raw` is copied into `jobs.id` |
| `first_seen_at`, `last_seen_at` | set on insert; only `last_seen_at` moves on re-ingest. Scoring filters on `first_seen_at`. Retention deletes on `last_seen_at` |

**Dropped columns:** `stage1_status` (every row in `jobs` is ingested by definition), `user_id` (unused, returns with auth), plus the six location columns listed above.

Indexes: `jobs (first_seen_at)` for the scoring window, `jobs (last_seen_at)` for retention, `jobs (company_key)` for the companies slice.

Title and `location.raw` go into `id` after the same string normalization: trim, collapse each run of whitespace to a single space, lowercase. An empty `raw` is an empty segment. No stemmer, no stop-word list, no geo coder. Two boards that share company key, normalized title, and normalized raw location are one row even when their `apply_url` values differ. On that upsert, ATS `url` / `apply_url` still win over aggregator values.

### `profile_runs`

| Column | Role |
| ------ | ---- |
| `id` | `BIGSERIAL` |
| `profile_id` | text, the YAML `id`; no FK, profiles are files |
| `status` | `RUNNING` / `SUCCEEDED` / `FAILED` |
| `started_at`, `finished_at` | `finished_at` null while running. `started_at` is the clock for the 10-day score window |
| `jobs_scored`, `sources_skipped` | counters shown in the UI |
| `idempotency_key` | UUID, unique |

### `profile_matches`

| Column | Role |
| ------ | ---- |
| `profile_id`, `job_id` | composite primary key; `job_id` FK to `jobs` `ON DELETE CASCADE` |
| `bucket` | `PASSED` / `REJECTED`, check constraint |
| `score` | int, may be negative |
| `signals` | `JSONB` array of `Signal` |
| `user_status` | `NEW` / `HIDDEN`, default `NEW`, check constraint |
| `run_id` | FK to `profile_runs` — the run that last wrote the score |
| `updated_at` | |

The scoring upsert writes `bucket`, `score`, `signals`, `run_id`, `updated_at` and **must not** include `user_status` in the update set. Index: `profile_matches (profile_id, bucket, user_status, score DESC)`.

### `ingest_watermarks` (changed)

Primary key loses `slot_id`: `(source_id)` only. The `slot_id` column and its FK are dropped.

**Dropped tables:** `slots`, `slot_jobs`, `slot_idempotency_keys`, `user_profile`, `pipeline_runs`, `pipeline_run_jobs`.

**Migration:** rewrite `migrations/000001_initial_schema.{up,down}.sql` for the v2 shape. The local database is dropped and re-created; no `000002` with a wall of `DROP`s.

---

## HTTP — `internal/publicapi`

Prefix `/api/v1`. Error envelope unchanged:

```json
{ "error": { "code": "DOMAIN.CODE", "message": "Human-readable explanation" } }
```

Bodies keep the current pattern: embedded JSON Schema in `handlers/json_schema/`, then `publicapi/utils.ReadValidatedJSON` into a typed struct with `DisallowUnknownFields()`.

**Removed routes:** `GET|PUT /api/v1/profile`, all of `/api/v1/slots/*` including `stages/2|3/run`, `stages/{stage}/jobs` and the stage patch. **Removed schemas:** `create_slot`, `profile_put`, `stage2_run`, `stage3_run`. **New schema:** `patch_user_status`. No request body carries queries, excludes, or country rules.

### `GET /api/v1/profiles`

Profiles from YAML, in file-name order.

**Response `200`** — `{ "profiles": [ { "id", "name", "domain", "sources": [], "queries": [] } ] }`. Enough for the picker. Exclude lists and penalties are not exposed.

### `POST /api/v1/profiles/{profile_id}/runs`

Starts a run. Header `Idempotency-Key` (non-nil UUID) required. No body.

**Response `202`** — `{ "run_id", "profile_id", "status": "RUNNING", "started_at" }`. Replaying the same key returns the same run.

**Errors:** `400` missing/invalid key, `404` unknown profile, `409` a run is already open or the key was reused with a different profile.

### `GET /api/v1/profiles/{profile_id}/runs/latest`

**Response `200`** — `{ "run_id", "status", "started_at", "finished_at", "jobs_scored", "sources_skipped" }`.

**Errors:** `404` unknown profile or no run yet.

### `GET /api/v1/profiles/{profile_id}/jobs`

Query: `bucket` (optional, `PASSED` or `REJECTED`; default `PASSED`), `user_status` (optional, `NEW` or `HIDDEN`; omitted means `NEW`), `page` (≥1), `limit` (1–100). Default sort `score DESC, first_seen_at DESC`. **No score threshold.** There is no query value that returns both statuses.

**Response `200`** — `{ "page", "limit", "total", "jobs": [ { "job_id", "title", "company", "url", "apply_url", "location", "sources": [], "posted_at", "first_seen_at", "last_seen_at", "bucket", "score", "signals": [], "user_status" } ] }`.

**Errors:** `400` bad query, `404` unknown profile.

### `PATCH /api/v1/profiles/{profile_id}/jobs/{job_id}`

**Body** — `{ "user_status": "HIDDEN" }`

**Response `200`** — the updated job entry. **Errors:** `400` invalid value, `404` profile unknown or job not scored for it.

---

## Temporal

| Kind | Name | Module |
|------|------|--------|
| Workflow | `ProfileRunWorkflow` | `internal/scoring/workflows` |
| Workflow | `IngestSourceWorkflow` | `internal/ingest/workflows` (unchanged except payload) |
| Workflow | `JobRetentionWorkflow` | `internal/jobs/workflows` (unchanged except the timestamp column) |
| Activity | `ScoreProfile` | `internal/scoring/workflows/activities` |
| Activity | `MarkRunFinished` | `internal/scoring/workflows/activities` |

**Deleted:** `ManualSlotRunWorkflow` and its run kinds, every stage-3 activity, `RunPipelineStage2` / `RunPipelineStage3`.

**Start / id / concurrency:** workflow id `profile-run-{profile_id}`, reuse policy `ALLOW_DUPLICATE` after close, so a second run while one is open is rejected by Temporal and surfaces as 409.

**Payloads:** `ProfileRunInput{ ProfileID string; RunID int64 }` — the resolved profile is **not** in the payload. Activities re-read the YAML, so a run carries no stale copy of the phrases. Ingest child input drops `SlotID` and carries `SourceID` + the query string taken from the profile (`queries` or one `wellfound_roles` slug).

Redis keys: `ingest:lock:{source_id}:{query_segment}` and `ingest:cooldown:{source_id}:{query_segment}`; empty query keeps the `catalog` segment. `ingest.ErrNilSlotID` is deleted.

A child that is locked, cooling down, rate-limited, or whose collector this worker process did not build is skipped. The workflow logs that source id and increments `sources_skipped`. The run still finishes `SUCCEEDED`.

---

## Config

| Env | Meaning | Default |
| --- | ------- | ------- |
| `JOBHOUND_PROFILES_DIR` | profile YAML directory | empty → `config/profiles` relative to cwd |
| `JOBHOUND_JOB_RETENTION_DAYS` | retention window on `last_seen_at` | 30 |

The 10-day score window is `ScoreWindowDays` in `internal/scoring/schema`, not an env key.

**Removed:** `JOBHOUND_ANTHROPIC_API_KEY`, `JOBHOUND_ANTHROPIC_MODEL`, `JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN`. **Removed `config.Config` fields** (declared but never loaded): `TelegramBotToken`, `TelegramChatID`, `HTTPUserAgent`, `IncludeKeywords`, `ExcludeKeywords`. No `JOBHOUND_DICTIONARIES_DIR`.

The profiles directory must be mounted into the `api` and `worker` containers — the API reads profiles for `GET /api/v1/profiles`, the worker reads them for ingest and scoring. Update `.env.example`, `docker-compose.yml` and the Dockerfile copy step together.

---

## Errors

| HTTP | `code` | When |
| ---- | ------ | ---- |
| `400` | `HTTP.INVALID_JSON` | body is not JSON |
| `400` | `HTTP.VALIDATION_FAILED` | JSON Schema failed |
| `400` | `HTTP.INVALID_QUERY` | bad `page` / `limit` / `bucket` / `user_status` |
| `400` | `HTTP.IDEMPOTENCY_KEY_REQUIRED` | header missing |
| `400` | `HTTP.INVALID_IDEMPOTENCY_KEY` | not a non-nil UUID |
| `409` | `HTTP.IDEMPOTENCY_KEY_CONFLICT` | key reused with a different request |
| `404` | `PROFILES.NOT_FOUND` | no YAML with that id |
| `400` | `PROFILES.INVALID_DEFINITION` | YAML malformed or unknown source id |
| `409` | `PROFILES.RUN_ALREADY_RUNNING` | a run is open for this profile |
| `404` | `PROFILES.NO_RUN` | `runs/latest` before the first run |
| `404` | `PROFILES.JOB_NOT_IN_SCOPE` | job not scored for this profile |
| `400` | `PROFILES.INVALID_USER_STATUS` | value outside `NEW` / `HIDDEN` |
| `405` | `HTTP.METHOD_NOT_ALLOWED` | unchanged |
| `500` | `INTERNAL.UNEXPECTED` | unchanged; cause logged, never in the body |

Idempotency codes move from the `SLOTS.` prefix to `HTTP.` — they are not entity-specific. **Removed codes:** `HTTP.INVALID_STAGE`, `SLOTS.LIMIT_REACHED`, `SLOTS.NOT_FOUND`, `SLOTS.STAGE_ALREADY_RUNNING`, `SLOTS.NO_PIPELINE_RUN`, `SLOTS.PROFILE_REQUIRED`, `PIPELINE.JOB_NOT_IN_SCOPE`, and the three `SLOTS.IDEMPOTENCY_*`. Regenerate `internal/publicapi/schema/generated/api-error-registry.json` and the frontend's `types/generated/apiErrorCodes.ts`.

The slot-cap special case (top-level `limit` next to `error`) disappears with the cap, so `SlotLimitReachedBody` is deleted and the envelope becomes uniform.

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Handlers | `internal/publicapi/handlers/*_test.go` | the five profile routes against a mocked `scoring.API` + `profiles.Store`: 202 start, 409 double start, query validation, patch of `user_status`. POST runs rejects a body |
| Impl | `internal/scoring/impl/*_test.go` | title exclude before text exclude; text exclude rejects; a penalty keeps `PASSED` and subtracts 10; each query phrase +1 once; case fold; `java` does not match `javascript`; the word `frontend` adds nothing unless it is itself a query phrase; score 0 is `PASSED` |
| Impl | `internal/profiles/impl/*_test.go` | YAML load: valid file, unknown key, unknown source id. No dictionary |
| Storage | `internal/scoring/storage/*_test.go` | the upsert preserves `user_status` while overwriting score and bucket |
| Storage | `internal/jobs/storage/*_test.go` | dedup-key upsert merges two sources into one row even when their apply URLs differ; `first_seen_at` stable, `last_seen_at` moves |
| Workflows | `internal/scoring/workflows/*_test.go` | SDK test env: ingest children from the profile YAML, then score; a skipped source is not a failed run; `user_status` survives a second pass |

No standalone tests for enum plumbing.
