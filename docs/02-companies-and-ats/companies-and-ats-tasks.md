# Company database and direct ATS polling — tasks

> Agent implementation plan for [`companies-and-ats-concept.md`](companies-and-ats-concept.md) / [`companies-and-ats-schemas-contracts.md`](companies-and-ats-schemas-contracts.md). Measured facts behind the decisions: [`rnd-findings.md`](rnd-findings.md).
> Every checkbox is finishable in code. No manual QA, no E2E scripts, no "run the product".
> Frontend: **no tasks**. The job list keeps its shape and closed vacancies are filtered server-side.

---

## Scope

**In this slice**

1. A `companies` table with an immutable key, merge-or-insert, aliases, and a blocklist on the discover path.
2. Resolve `PENDING` companies to ATS + verified slug, primarily from the `apply_url` aggregators already return.
3. Three board fetchers (Greenhouse, Lever, Ashby) and a capped, conditional poll loop.
4. Upsert vacancies from boards and **close** the ones a complete poll no longer lists — with aggregator re-ingest unable to revert a close.
5. Run counters that show whether the flip works.
6. Seed import from the existing xlsx.
7. Read-only `GET /api/v1/companies` for inspection.

**Paused (do not implement here)**

- `domains` column, any per-profile company filter — the slice is single-domain (`software`).
- `tech_tags`, `UpdateCompanyTags`, `internal/dictionary`.
- `applied_companies`, applied-company UI highlight, applied statuses.
- Fetchers beyond the three (Workday, Teamtailor, Workable, Recruitee, Personio, Rippling) — recorded as `UNSUPPORTED_ATS` instead.
- Resolve step "guess + verify" from the company name.
- Parsing hand-rolled careers HTML as a vacancy source.
- Mapping board `work_mode` / geography into `facts` to replace enrich answers.
- "Known company + matching stack" score signal.
- Any LLM call inside this slice's own logic.

**Two ordering constraints that are not negotiable**

- `S4` (close protection) lands in the same change as `S3` (`closed_at` / `closed_by`). A close column without the protection rule is a feature that silently reverts itself every run.
- `W5` (poll stage) is wired **before** the score step inside `ProfileRunWorkflow`, never after.

---

## 0. Schema / contract

- [ ] **T1** — `internal/companies/schema/company.go`: `Company`, `UpsertCompanyParams`, `UpsertCompanyResult`, `ListFilter`, `BoardResult`, workflow payloads
- [ ] **T2** — `internal/companies/schema/enums.go`: `ATS` (incl. non-pollable values + `Pollable()`), `ResolutionStatus`, `ResolutionMethod`, `PollState`, `PollOutcome`, `FailureStage` — each with the full Canonical Enum Pattern
- [ ] **T3** — `internal/companies/schema/blocklist.go`: `Blocklist` struct with `exact` / `anonymous` / `review`
- [ ] **T4** — `internal/companies/contract.go`: `API` + `CompanyBoardFetcher` interfaces, sentinel errors (`ErrCompanyNotFound`, `ErrCompanyBlocked`, `ErrResolveInProgress`, `ErrNotPollable`, `ErrBoardFormat`)
- [ ] **T5** — `internal/jobs/schema`: `ClosedBy` enum (`ATS` | `AGGREGATOR`) with Canonical Enum methods
- [ ] **T6** — `internal/publicapi/schema`: company list DTOs (`board_etag` **not** on the wire); add `closed_at` to the job item; register `COMPANIES.NOT_FOUND` in the error registry

---

## 1. Config / migration

- [ ] **G1** — `internal/config/companies.go`: `JOBHOUND_COMPANIES_DIR`, `_POLL_MAX_PER_RUN` (50), `_POLL_INTERVAL_MIN_HOURS` (6), `_POLL_INTERVAL_MAX_HOURS` (24), `_ATS_CONCURRENCY` (5), `_RESOLVE_RETRY_DAYS` (14). Reject non-positive values at load. No `os.Getenv` outside this package
- [ ] **G2** — add every key to `.env.example` with the default in a comment
- [ ] **M1** — `migrations/000002_companies_and_ats.up.sql`: `companies` table with all columns and the three indexes from the contracts file. Do **not** edit `000001`
- [ ] **M2** — same migration: `jobs.company_key_ref` (nullable FK `ON DELETE SET NULL`), `jobs.closed_at`, `jobs.closed_by`, partial index `WHERE closed_at IS NULL`
- [ ] **M3** — same migration: the eleven `profile_runs` counters, `INT NOT NULL DEFAULT 0`
- [ ] **M4** — `migrations/000002_companies_and_ats.down.sql` that fully reverses M1–M3

---

## 2. Storage

- [ ] **S1** — `internal/companies/storage`: GORM model + `Upsert`, `GetByKey`, `FindByWebsite`, `FindByAlias`, `List`, `ListPendingResolve`, `ListDuePollable` (ordered `last_polled_at ASC NULLS FIRST`, limited), `MarkPolled`, `MarkResolution`
- [ ] **S2** — storage tests (sqlite, as the existing `scoring/storage` and `enrich/storage` tests do): upsert, alias lookup, due-query ordering and cap
- [ ] **S3** — `internal/jobs/storage`: `closed_at` / `closed_by` / `company_key_ref` on the model; `CloseAbsent(companyKey string, seenIDs []string)`; `ReopenPresent`
- [ ] **S4** — **close protection in the existing job upsert**: an aggregator upsert refreshes `last_seen_at` / unions `openings` / fills empty `company_website`, and **never clears `closed_at` when `closed_by='ATS'`**. Only a complete poll clears it. Ships together with S3
- [ ] **S5** — storage tests for S4 specifically: aggregator upsert over an ATS-closed row leaves `closed_at` intact; a complete poll clears it
- [ ] **S6** — `internal/scoring/storage`: increment the new `profile_runs` counters

---

## 3. Impl

- [ ] **I1** — `internal/companies/impl/blocklist.go`: load `blocklist.yaml` from `JOBHOUND_COMPANIES_DIR`; exact match on the normalized key for `exact`/`anonymous`, case-insensitive substring for `review` → `NeedsReview`. **No fuzzy matching**
- [ ] **I2** — `internal/companies/impl/identity.go`: key normalization (lowercase, strip `Inc`/`GmbH`/`Ltd`/`LLC`/`SRL`; domains drop `www.` / `careers.`). The **slug is never normalized** — add a comment saying so, with the four breaking slugs as the reason
- [ ] **I3** — `internal/companies/impl/upsert.go`: `UpsertFromAggregator` — blocklist check, then look up by key, website and aliases; on a hit merge and record the other form in `aliases`; never a second row. Key is fixed at birth and never promoted
- [ ] **I4** — `internal/companies/impl/resolve.go`: step 1 apply URL on the raw host → `APPLY_URL`; recognised non-pollable host → `UNSUPPORTED_ATS` with `next_resolve_at` NULL; unknown host → follow redirects; nothing → `UNRESOLVED` with `next_resolve_at = now + retryDays`
- [ ] **I5** — `internal/companies/impl/poll.go`: `DuePollable` (interval from config, oldest-first, capped) and `Poll` — dispatch to the fetcher by `ATS`, apply the `PollOutcome`: `COMPLETE` upserts **and closes**; `PARTIAL` upserts and closes nothing; `EMPTY` is success, bumps `empty_poll_streak`, keeps the slug; `NOT_MODIFIED` touches only `last_polled_at`; `DEAD_SLUG` bumps `poll_fail_count`
- [ ] **I6** — state transitions: 5 consecutive `EMPTY` → `poll_state=INACTIVE` (slug kept); a new aggregator sighting of an `INACTIVE` company → back to `ACTIVE`; 3 consecutive `DEAD_SLUG` → `resolution_status=PENDING`, previous slug kept in `resolution_method` history
- [ ] **I7** — fallback rule: a `RESOLVED` company whose complete poll returned zero while the aggregator still shows jobs for it keeps the aggregator path, and the discrepancy is logged (`board_jobs=0, aggregator_jobs=N`)
- [ ] **I8** — sentinel error mapping; `ErrBoardFormat` is terminal for that company this run and logged loudly
- [ ] **I9** — impl tests with a fake fetcher and fake storage: merge paths, blocklist, resolve order incl. the `UNSUPPORTED_ATS` short-circuit, every `PollOutcome` branch, both streak transitions

---

## 4. Fetchers

- [ ] **F1** — `internal/collectors/ats/greenhouse`: `GET /v1/boards/{slug}/jobs?content=true`. **`content=true` is mandatory** — without it the response is titles only
- [ ] **F2** — `internal/collectors/ats/ashby`: `GET /posting-api/job-board/{slug}`, `descriptionPlain`
- [ ] **F3** — `internal/collectors/ats/lever`: `GET /v0/postings/{slug}?mode=json`, `descriptionPlain`
- [ ] **F4** — shared rules in all three: **status code checked before decoding** (Ashby's 404 body is plain text, not JSON); `404` → `DEAD_SLUG`, not an error; `200` + empty → `EMPTY`; `If-None-Match` from `board_etag` → `304` → `NOT_MODIFIED`; slug sent verbatim; honest `COMPLETE` vs `PARTIAL`; streaming decode (payloads reach 2.0 MB)
- [ ] **F5** — map the structured fields into `domain/schema.Job`: `isRemote` / `workplaceType` / `secondaryLocations[]` / `categories.allLocations[]` / `offices[]`, `applyUrl`, publish timestamps. Keep them in the fetcher schema even though `facts` mapping is out of scope
- [ ] **F6** — `AssignStableID` on every mapped job so ATS and aggregator rows collapse to one `jobs` row
- [ ] **F7** — fixtures per provider under `testdata/`, **including the `404` body and the empty-board body**; tests over every outcome path and over `content=true` being present on the Greenhouse request

---

## 5. Temporal

- [ ] **W1** — `internal/companies/workflows`: `ResolveCompanyWorkflow` (id `resolve-{company_key}`, one active per company) and `PollCompaniesWorkflow` (id `poll-companies-{run_id}`, per-ATS concurrency 5, per-run cap). `RegisterWorkflow` listed in `New...`
- [ ] **W2** — activities `UpsertCompaniesFromJobs`, `ListPendingResolve`, `ResolveCompany`, `ListDuePollable`, `PollCompany`; `workflows/mappers.go` for Temporal → domain. **No `go.temporal.io/sdk` under `impl/`**
- [ ] **W3** — failure semantics: transport failure → Temporal retry, on exhaustion `poll_fail_count++` and **nothing closed**; business skips (blocklist, not due, `UNSUPPORTED_ATS`) are never failures; `failure_stage` on every log line
- [ ] **W4** — `internal/scoring/workflows/profile_run.go`: insert the discover stage (`UpsertCompaniesFromJobs`) and the resolve children after ingest. Children use `ParentClosePolicy` abandon and a failed child **does not fail the run**, matching the existing enrich contract
- [ ] **W5** — same file: run `PollCompaniesWorkflow` **before** the score step, so the pass scores ATS text rather than the aggregator's truncated version
- [ ] **W6** — update the run counters as children finish, the way `jobs_scored` / `sources_skipped` already do
- [ ] **W7** — workflow tests: child id scheme, a failed child leaving the parent `SUCCEEDED`

---

## 6. HTTP

- [ ] **H1** — `internal/publicapi/handlers/companies_list.go`: `GET /api/v1/companies` with `resolution_status` / `poll_state` / `ats` / `needs_review` / `page` / `limit`; register in `registerRoutes()`. One file per route, no package-level helpers
- [ ] **H2** — job list: exclude `closed_at IS NOT NULL` by default; new optional `closed` query (default `false`); add `closed_at` to the response item
- [ ] **H3** — errors through `WriteError` only: `HTTP.INVALID_QUERY` on a bad enum, `COMPANIES.NOT_FOUND` where applicable, nothing leaked on `500`
- [ ] **H4** — handler tests (`httptest`, mocked impl): filters, `400` on a bad enum, job list excluding closed by default and including them with `closed=true`

---

## 7. Seed import

- [ ] **P1** — `cmd/companies-import`: read `config/companies/companies_template.xlsx`, skip the example row and `Fit=SKIP`
- [ ] **P2** — board verify per row: `200` → `RESOLVED` **including an empty list** (the slug is verified); `404` → `PENDING` with the slug discarded; recognised non-pollable ATS → `UNSUPPORTED_ATS`; Custom/UNKNOWN/NONE → `PENDING`; blocklisted name → skipped and counted
- [ ] **P3** — idempotent: re-running merges instead of duplicating
- [ ] **P4** — `make companies-import` target
- [ ] **P5** — import tests over a small fixture sheet covering each row class

---

## 8. Wiring

- [ ] **C1** — `cmd/worker`: construct the blocklist, the three fetchers, the companies service, register both workflows and all activities. Composition only
- [ ] **C2** — `cmd/api`: wire the companies repository for the read-only route and the job-list `closed` filter. Composition only
- [ ] **C3** — `Dockerfile` copies `config/companies/` next to `config/profiles/`; Compose passes `JOBHOUND_COMPANIES_DIR=/app/config/companies` to `api` and `worker`

---

## 9. Tests (extra coverage)

- [ ] **Q1** — table-driven impl case: company discovered by name in run 1 and by domain in run 2 → **one row**, second form in `aliases`
- [ ] **Q2** — table-driven: `cloudbees` / `cloudbeds`, `lemon.io` / `lemon.markets`, `insider` / `insider one` stay **separate** companies; `lemon.io` is blocked while `lemon.markets` is not
- [ ] **Q3** — the close-protection sequence end to end at storage level: complete poll closes → aggregator upsert leaves it closed → complete poll listing it again reopens it
- [ ] **Q4** — `PARTIAL` outcome closes nothing even though every HTTP call returned `200`

---

## Dependencies (brief)

```
T* → G*/M* → S* → I* → F* → W* → H*/P* → C*
Q* after I* + S* + H*
S3 and S4 ship together. W5 ordering is fixed.
```

**M1:** T1–T6, G1–G2, M1–M4, S1–S6
**M2:** I1–I9, F1–F7
**M3:** W1–W7, H1–H4, P1–P5
**M4:** C1–C3, Q1–Q4

Early check (after M2 + P1–P4, no Temporal needed): import the seed, poll one seeded slug through the impl, confirm vacancies upserted and counters moved; poll again and confirm `304` / `NOT_MODIFIED` with nothing re-scored.
