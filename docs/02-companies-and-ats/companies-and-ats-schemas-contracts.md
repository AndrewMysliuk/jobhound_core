# Company database and direct ATS polling — schemas and contracts

> Implementation contracts for the slice described in [`companies-and-ats-concept.md`](companies-and-ats-concept.md). Numbers behind every decision: [`rnd-findings.md`](rnd-findings.md).
> Single-domain slice (`domain: software`). No `domains` column, no `tech_tags`, no applied-company highlight.
> Frontend: no changes required. The job list keeps its existing shape; closed vacancies are filtered server-side.

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/companies` | — | `companies` | `ResolveCompanyWorkflow`, `PollCompaniesWorkflow` |
| `internal/collectors/ats` | — | — | — (called from company activities) |
| `internal/jobs` | — | `jobs` (+ `company_key` fk, `closed_at`, `closed_by`) | — |
| `internal/scoring` | — | `profile_runs` (+ new counters) | `ProfileRunWorkflow` (extended) |
| `internal/publicapi` | `/api/v1/companies` (debug, read-only) | — | — |

`internal/enrich` — **outside this slice**. It already skips short descriptions (`MinDescriptionChars`), and ATS vacancies pass that threshold on their own. No change.

`internal/ingest` — **outside this slice** except for one thing: the job upsert it performs must respect the close-protection rule below.

---

## Types (`internal/companies/schema`)

```go
// ATS is the board provider. Only the three with a working fetcher are pollable.
type ATS string

const (
	ATSGreenhouse ATS = "GREENHOUSE"
	ATSLever      ATS = "LEVER"
	ATSAshby      ATS = "ASHBY"
	// Recognised but not pollable. Stored so the company is not re-resolved forever
	// and so the bucket size says when a fourth fetcher pays.
	ATSWorkday        ATS = "WORKDAY"
	ATSTeamtailor     ATS = "TEAMTAILOR"
	ATSWorkable       ATS = "WORKABLE"
	ATSRecruitee      ATS = "RECRUITEE"
	ATSPersonio       ATS = "PERSONIO"
	ATSRippling       ATS = "RIPPLING"
	ATSSmartRecruiters ATS = "SMARTRECRUITERS"
	ATSOther          ATS = "OTHER"
)

// Pollable reports whether a fetcher exists for this ATS in this slice.
func (a ATS) Pollable() bool

type ResolutionStatus string

const (
	ResolutionStatusPending        ResolutionStatus = "PENDING"
	ResolutionStatusResolved       ResolutionStatus = "RESOLVED"
	ResolutionStatusUnresolved     ResolutionStatus = "UNRESOLVED"
	ResolutionStatusUnsupportedATS ResolutionStatus = "UNSUPPORTED_ATS"
)

type ResolutionMethod string

const (
	ResolutionMethodSeed           ResolutionMethod = "SEED"
	ResolutionMethodApplyURL       ResolutionMethod = "APPLY_URL"
	ResolutionMethodAggregatorPage ResolutionMethod = "AGGREGATOR_PAGE"
	ResolutionMethodCareersPage    ResolutionMethod = "CAREERS_PAGE"
)

type PollState string

const (
	PollStateActive   PollState = "ACTIVE"
	PollStateInactive PollState = "INACTIVE"
)

// PollOutcome distinguishes the five answers a board can give. "complete" is the
// only one that may close vacancies; see rnd-findings.md M3.
type PollOutcome string

const (
	PollOutcomeComplete     PollOutcome = "COMPLETE"      // reached end of pagination; may close
	PollOutcomeNotModified  PollOutcome = "NOT_MODIFIED"  // 304; closes nothing
	PollOutcomePartial      PollOutcome = "PARTIAL"       // 200s but truncated; upserts, closes nothing
	PollOutcomeEmpty        PollOutcome = "EMPTY"         // 200 + empty list; success, slug stays verified
	PollOutcomeDeadSlug     PollOutcome = "DEAD_SLUG"     // 404
)

type FailureStage string

const (
	FailureStageDiscover FailureStage = "DISCOVER"
	FailureStageResolve  FailureStage = "RESOLVE"
	FailureStagePoll     FailureStage = "POLL"
	FailureStageUpsert   FailureStage = "UPSERT"
	FailureStageScore    FailureStage = "SCORE"
)

type Company struct {
	Key              string // immutable; domain if known at birth, else normalized name
	Name             string
	Aliases          []string
	Website          string
	ATS              ATS
	ATSSlug          string // verbatim; NEVER normalized
	BoardURL         string
	BoardETag        string
	ResolutionStatus ResolutionStatus
	ResolutionMethod ResolutionMethod
	PollState        PollState
	NextResolveAt    *time.Time
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	LastPolledAt     *time.Time
	PollFailCount    int
	EmptyPollStreak  int
	NeedsReview      bool // blocklist `review` pattern hit, or an outsized board
}

type UpsertCompanyParams struct {
	Name     string
	Website  string
	ApplyURL string
}

type UpsertCompanyResult struct {
	Key     string
	Created bool
	Merged  bool
	Blocked bool
}

// BoardResult is what a fetcher returns. Outcome is as load-bearing as Jobs.
type BoardResult struct {
	Jobs    []domainschema.Job
	Outcome PollOutcome
	ETag    string
}

type ResolveCompanyPayload struct {
	CompanyKey string
}

type PollCompaniesPayload struct {
	RunID string
}
```

Every exported string enum above follows the Canonical Enum Pattern (`String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`) — same as `scoring/schema.Bucket` and `enrich/schema.FactsStatus`.

### Blocklist (`internal/companies/schema`)

```go
type Blocklist struct {
	Version   int      `yaml:"version"`
	Exact     []string `yaml:"exact"`
	Anonymous []string `yaml:"anonymous"`
	Review    []string `yaml:"review"`
}
```

Loaded from `config/companies/blocklist.yaml`. `Exact` and `Anonymous` block creation; `Review` only sets `NeedsReview`. Matching is exact on the normalized key for `Exact`/`Anonymous`, case-insensitive substring for `Review`. **No fuzzy matching anywhere** — `lemon.io` is blocked, `lemon.markets` is not; `cloudbees` and `cloudbeds` are two companies (M5).

### Vacancy close (`internal/jobs/schema`)

```go
type ClosedBy string

const (
	ClosedByATS        ClosedBy = "ATS"
	ClosedByAggregator ClosedBy = "AGGREGATOR"
)
```

---

## Contract (`internal/companies`)

`contract.go` — interfaces and sentinel errors only.

```go
type API interface {
	// UpsertFromAggregator is the discover path: blocklist check, then
	// merge-or-insert. Never creates a second row for a known company.
	UpsertFromAggregator(ctx context.Context, p schema.UpsertCompanyParams) (schema.UpsertCompanyResult, error)

	// Resolve finds ats + slug for one PENDING company and commits the outcome.
	Resolve(ctx context.Context, companyKey string) (schema.Company, error)

	// DuePollable returns RESOLVED + ACTIVE companies past their interval,
	// oldest LastPolledAt first, capped.
	DuePollable(ctx context.Context, now time.Time, limit int) ([]schema.Company, error)

	// Poll fetches one board and applies the result. Closing happens only on
	// PollOutcomeComplete.
	Poll(ctx context.Context, companyKey string) (schema.PollOutcome, error)

	Get(ctx context.Context, companyKey string) (schema.Company, error)
	List(ctx context.Context, f schema.ListFilter) ([]schema.Company, error)
}

// CompanyBoardFetcher is implemented once per pollable ATS.
type CompanyBoardFetcher interface {
	ATS() schema.ATS
	// FetchBoard returns the board plus the outcome. etag is the stored
	// BoardETag ("" on first poll); an empty-but-successful board is
	// PollOutcomeEmpty, a 404 is PollOutcomeDeadSlug — never an error.
	FetchBoard(ctx context.Context, slug, etag string) (schema.BoardResult, error)
}

var (
	ErrCompanyNotFound   = errors.New("company not found")
	ErrCompanyBlocked    = errors.New("company is blocklisted")
	ErrResolveInProgress = errors.New("resolve already running for this company")
	ErrNotPollable       = errors.New("company ats has no fetcher")
	ErrBoardFormat       = errors.New("board response does not match the expected format")
)
```

`ErrBoardFormat` is terminal for that company this run — a changed board format means a fetcher needs updating, not a retry.

---

## Storage

### `companies` (new)

| Column | Type | Role |
| ------ | ---- | ---- |
| `key` | TEXT PK | immutable identity; domain-form or name-form |
| `name` | TEXT NOT NULL | display name as last seen |
| `aliases` | TEXT[] NOT NULL DEFAULT `'{}'` | other forms this company was found under |
| `website` | TEXT NOT NULL DEFAULT `''` | discovered later than the key; never promoted to key |
| `ats` | TEXT NOT NULL DEFAULT `''` | `ATS` enum; set for `RESOLVED` and `UNSUPPORTED_ATS` |
| `ats_slug` | TEXT NOT NULL DEFAULT `''` | **verbatim**, case and dots preserved |
| `board_url` | TEXT NOT NULL DEFAULT `''` | human-facing board page |
| `board_etag` | TEXT NOT NULL DEFAULT `''` | last `ETag`, for `If-None-Match` |
| `resolution_status` | TEXT NOT NULL | `PENDING` \| `RESOLVED` \| `UNRESOLVED` \| `UNSUPPORTED_ATS` |
| `resolution_method` | TEXT NOT NULL DEFAULT `''` | how the slug was obtained |
| `poll_state` | TEXT NOT NULL DEFAULT `'ACTIVE'` | `ACTIVE` \| `INACTIVE` |
| `next_resolve_at` | TIMESTAMPTZ NULL | set for `UNRESOLVED` (+14d). **NULL for `UNSUPPORTED_ATS`** — never retried |
| `first_seen_at` / `last_seen_at` | TIMESTAMPTZ NOT NULL | discovery window |
| `last_polled_at` | TIMESTAMPTZ NULL | drives the due interval |
| `poll_fail_count` | INT NOT NULL DEFAULT 0 | transport + dead-slug failures |
| `empty_poll_streak` | INT NOT NULL DEFAULT 0 | 5 consecutive `EMPTY` → `INACTIVE` |
| `needs_review` | BOOLEAN NOT NULL DEFAULT false | blocklist `review` hit or outsized board |
| `created_at` / `updated_at` | TIMESTAMPTZ NOT NULL | |

Indexes: `(resolution_status, poll_state, last_polled_at)` for `DuePollable`; `(website)` and a GIN index on `aliases` for merge lookup; `(resolution_status, next_resolve_at)` for the resolve queue.

Companies are **never deleted by retention** — the row outlives its vacancies.

### `jobs` (altered)

| Column | Type | Role |
| ------ | ---- | ---- |
| `company_key_ref` | TEXT NULL REFERENCES `companies(key)` ON DELETE SET NULL | the resolved company; NULL for blocklisted/anonymous |
| `closed_at` | TIMESTAMPTZ NULL | set when a complete poll no longer lists the vacancy |
| `closed_by` | TEXT NOT NULL DEFAULT `''` | `ATS` \| `AGGREGATOR` |

The existing `company_key` TEXT column stays as the content key that builds `jobs.id` — it is **not** replaced. `company_key_ref` is the foreign key to the new table, nullable because a blocklisted company has no row.

Index: `(closed_at)` partial `WHERE closed_at IS NULL` for the default list.

**Close protection (the rule that makes closing work at all).** An aggregator upsert may refresh `last_seen_at`, union `openings`, and fill an empty `company_website`. It **must not** clear `closed_at` when `closed_by = 'ATS'`. Only a complete poll of that company's board may set `closed_at = NULL` and `closed_by = ''`. Without this, the content-derived `jobs.id` plus a lagging aggregator reverts every close on the next run.

### `profile_runs` (altered)

New counters, all `INT NOT NULL DEFAULT 0`: `companies_discovered`, `companies_merged`, `companies_blocked`, `companies_resolved`, `companies_unsupported`, `companies_polled`, `polls_not_modified`, `polls_partial`, `poll_failures`, `jobs_upserted_ats`, `jobs_closed`.

`polls_partial` is its own counter on purpose: a chronically truncating board closes nothing and would otherwise look healthy.

**Migration:** `migrations/000002_companies_and_ats.{up,down}.sql`. The existing single revision `000001_initial_schema` is not edited. GORM models only in `internal/companies/storage` and `internal/jobs/storage`.

`make db-dump` / `db-restore` must be verified **before** this migration runs against a database holding data worth keeping.

---

## HTTP — `internal/publicapi`

Prefix `/api/v1`. One new read-only route. Envelope on non-2xx is the existing `{"error":{"code","message"}}`.

### `GET /api/v1/companies`

Debug / inspection only. Not consumed by the frontend in this slice.

**Query:** `resolution_status` (enum), `poll_state` (enum), `ats` (enum), `needs_review` (bool), `page` (≥1), `limit` (1–100). Sort `last_polled_at ASC NULLS FIRST, key ASC`.

**Response `200`**

```json
{
  "items": [
    {
      "key": "storyblok",
      "name": "Storyblok",
      "website": "https://www.storyblok.com",
      "ats": "GREENHOUSE",
      "ats_slug": "storyblok",
      "board_url": "https://boards.greenhouse.io/storyblok",
      "resolution_status": "RESOLVED",
      "resolution_method": "SEED",
      "poll_state": "ACTIVE",
      "last_polled_at": "2026-10-25T10:00:00Z",
      "poll_fail_count": 0,
      "needs_review": false
    }
  ],
  "page": 1,
  "limit": 50,
  "total": 857
}
```

`board_etag` is **not** on the wire — internal polling state.

**Errors:** `HTTP.INVALID_QUERY` (`400`) on a bad enum or page.

### Existing `GET /api/v1/profiles/{profile_id}/jobs` — changed

- Closed vacancies are **excluded by default**, the same way `user_status=HIDDEN` is.
- New optional query `closed` (`true` | `false`, default `false`) to look at them deliberately.
- Each item gains `closed_at` (nullable string) so the UI can render it later; existing fields unchanged.

No new request body schemas — both routes are `GET`. The `patch_user_status` schema is untouched.

---

## Temporal

| Kind | Name | Module |
|------|------|--------|
| Workflow | `ResolveCompanyWorkflow` | `internal/companies/workflows` |
| Workflow | `PollCompaniesWorkflow` | `internal/companies/workflows` |
| Activity | `UpsertCompaniesFromJobs` | `internal/companies/workflows/activities` |
| Activity | `ListPendingResolve` | same |
| Activity | `ResolveCompany` | same |
| Activity | `ListDuePollable` | same |
| Activity | `PollCompany` | same |

Mapping Temporal types → domain lives in `workflows/mappers.go`. **`impl/` must not import `go.temporal.io/sdk/...`** (enforced by `depguard`).

**Start / id / concurrency**

- `ResolveCompanyWorkflow` — id `resolve-{company_key}`, reuse `ALLOW_DUPLICATE` after close. One active resolve per company; two profiles running at once must not race. Started as a child of `ProfileRunWorkflow` with `ParentClosePolicy` **abandon**, like `EnrichBatchWorkflow`.
- `PollCompaniesWorkflow` — id `poll-companies-{run_id}`. Concurrency cap **5 per ATS**. Hard cap on companies per run (`JOBHOUND_COMPANIES_POLL_MAX_PER_RUN`), oldest-first, so a growing table lengthens the cycle rather than the run.
- A failed resolve or poll child **does not fail the profile run** — same contract as enrich today.

**Payloads:** `ResolveCompanyPayload{CompanyKey}` and `PollCompaniesPayload{RunID}` only. Activities re-read the company row; the payload never carries board data or company fields.

**Placement inside `ProfileRunWorkflow`** — three stages inserted between ingest and score:

```
ingest children (unchanged)
  → UpsertCompaniesFromJobs          (discover)
  → ListPendingResolve → ResolveCompanyWorkflow children
  → PollCompaniesWorkflow            (upsert + close)
  → score (unchanged rules, more text to score)
  → enrich (unchanged; ATS text passes MinDescriptionChars on its own)
  → run SUCCEEDED
```

Score must run **after** the poll stage, or the first pass scores the aggregator's truncated text and the ATS text waits a full run.

---

## Config

| Env | Meaning | Default |
| --- | ------- | ------- |
| `JOBHOUND_COMPANIES_DIR` | directory holding `blocklist.yaml` and the seed file | empty → `config/companies` |
| `JOBHOUND_COMPANIES_POLL_MAX_PER_RUN` | hard cap on board polls in one profile run | 50 |
| `JOBHOUND_COMPANIES_POLL_INTERVAL_MIN_HOURS` | lower bound of the per-company due interval | 6 |
| `JOBHOUND_COMPANIES_POLL_INTERVAL_MAX_HOURS` | upper bound | 24 |
| `JOBHOUND_COMPANIES_ATS_CONCURRENCY` | parallel polls per ATS | 5 |
| `JOBHOUND_COMPANIES_RESOLVE_RETRY_DAYS` | `UNRESOLVED` → `next_resolve_at` offset | 14 |

Names and parsing **only** in `internal/config` (new `companies.go`). Feature packages must not `os.Getenv`. Add every row to `.env.example` with the default in a comment.

The poll cap is a **memory** limit as much as a politeness one: M3 measured 2.0 MB for `ashby`, ~9 MB per 30-company cycle. Fetchers decode streaming and do not hold whole bodies.

---

## Errors

| HTTP / sentinel | `code` | When |
| --------------- | ------ | ---- |
| `400` | `HTTP.INVALID_QUERY` | bad enum / page / limit on `GET /companies` or `closed` |
| `404` | `COMPANIES.NOT_FOUND` | company key not in the table |
| `500` | `INTERNAL.UNEXPECTED` | anything else; cause logged, never copied into the body |

New registry entries in `internal/publicapi/schema`: `COMPANIES.NOT_FOUND`. `impl/` does not import the registry; handlers go through `WriteError` and never pick a status or message.

Sentinel → wire mapping happens in the handler layer: `ErrCompanyNotFound` → `COMPANIES.NOT_FOUND`. `ErrCompanyBlocked`, `ErrResolveInProgress`, `ErrNotPollable` and `ErrBoardFormat` never reach HTTP — they are internal outcomes of the worker path.

---

## Fetchers (`internal/collectors/ats`)

Three packages, `net/http` only, **no rod** — these are JSON APIs. All three verified live (M3).

| ATS | Request | Description field |
|---|---|---|
| `greenhouse` | `GET https://boards-api.greenhouse.io/v1/boards/{slug}/jobs?content=true` | `content` (HTML) |
| `ashby` | `GET https://api.ashbyhq.com/posting-api/job-board/{slug}` | `descriptionPlain` |
| `lever` | `GET https://api.lever.co/v0/postings/{slug}?mode=json` | `descriptionPlain` |

Shared rules, each one a measured fact and not a guess:

1. **`?content=true` on Greenhouse is mandatory.** Without it the response is titles only (8.2 KB vs 185 KB for the same 14 jobs) and nothing scores.
2. **Check the status code before decoding.** Ashby answers an unknown slug with plain-text `Not Found`, not JSON. Decoding first turns a dead slug into a parse error and blames the wrong failure stage.
3. **`404` → `PollOutcomeDeadSlug`, not an error.** `200` + empty list → `PollOutcomeEmpty`, a success that keeps the slug verified.
4. **Send `If-None-Match` when `board_etag` is set.** All three return `304`.
5. **Pass the slug verbatim.** Never lowercase, never strip dots — 4 of 30 real slugs break otherwise (`541`, `primer.io`, `ecosia.org`, `Coder`/`Deepnote`/`Mastra`).
6. **Report completeness honestly.** `PollOutcomeComplete` only when pagination is exhausted; anything truncated is `PollOutcomePartial` and closes nothing.
7. Map the structured fields the boards already give — `isRemote` / `workplaceType` / `secondaryLocations[]` / `categories.allLocations[]` / `offices[]`, plus `applyUrl` and publish timestamps — into `domain/schema.Job`. Using them to **replace** enrich facts is a later slice; do not discard them in the fetcher's own schema.

Fixtures for each fetcher go under `internal/collectors/ats/{provider}/testdata/`, including the `404` and empty-board bodies, because those two paths carry the load-bearing logic.

---

## Seed import (`cmd/companies-import`)

One-shot CLI, outside the run button. Reads `config/companies/companies_template.xlsx` (64 rows today, all `Fit=FIT`).

| Seed row | Behavior |
| --- | --- |
| `Fit=SKIP` or the example row | skipped |
| ATS is Greenhouse / Lever / Ashby with a slug | fetch the board; `200` → `RESOLVED` (**an empty list still counts** — the slug is verified, the company just has nothing open); `404` → `PENDING`, slug discarded |
| ATS recognised but not pollable | `UNSUPPORTED_ATS`, store the `ats`, no slug verify, `next_resolve_at` NULL |
| Custom / UNKNOWN / NONE | `PENDING`, no slug |
| Name on the blocklist | skipped, counted |

The import is idempotent — re-running it merges rather than duplicating.

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Fetchers | `internal/collectors/ats/{greenhouse,lever,ashby}/*_test.go` | parse fixtures; **`404` → `DEAD_SLUG`**, **`200` + empty → `EMPTY`**, `304` → `NOT_MODIFIED`, truncated → `PARTIAL`; slug passed verbatim; Greenhouse request carries `content=true` |
| Impl | `internal/companies/impl/*_test.go` | merge-or-insert (name then domain → one row + alias); blocklist exact / anonymous / review; resolve step order with an `UNSUPPORTED_ATS` short-circuit; `DuePollable` ordering and cap; `EMPTY` streak → `INACTIVE`; dead-slug streak of 3 → `PENDING` |
| Storage | `internal/companies/storage/*_test.go` | company upsert, alias lookup, due query (sqlite like the existing storage tests) |
| Close protection | `internal/jobs/storage/*_test.go` | aggregator upsert does **not** clear `closed_at` when `closed_by='ATS'`; a complete poll does |
| Handlers | `internal/publicapi/handlers/*_test.go` | `GET /companies` filters + `400` on a bad enum; job list excludes closed by default and includes them with `closed=true` |
| Workflows | `internal/companies/workflows/*_test.go` | child id scheme; a failed child does not fail the parent |

Do not add tests whose only job is enum plumbing. The three fetcher outcome paths and the close-protection rule are where the bugs live.
