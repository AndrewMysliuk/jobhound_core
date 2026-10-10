# Search profiles and rule scoring (v2) — tasks

> Agent implementation plan. One checkbox is one agent. Do that checkbox only, then stop.
> Behavior that is already written in [`profiles-and-scoring-concept.md`](./profiles-and-scoring-concept.md) and [`profiles-and-scoring-schemas-contracts.md`](./profiles-and-scoring-schemas-contracts.md) is fixed. Do not invent rules, phrase lists, routes, or columns.
> Each task leaves the tree compiling: `make test` for the packages it touches, and `make build` when it changes `cmd/`. Frontend tasks run `pnpm typecheck` and, where the **Done** line says so, `pnpm test:unit` in `jobhound_frontend`. Mark only your own checkbox `[x]` when its **Done** line is true.
> No manual QA, DoD, E2E, or browser walkthroughs in this file.
> Nuxt work is in section 6 of this file. Do not create `jobhound_frontend/docs/01-profiles-and-scoring/`.

---

## Scope

**In this slice**

1. Remove stage 3, LLM, the global profile text, slots, and the slot-scoped pipeline run path.
2. Load search profiles from YAML. Ship `andrew` only. Queries, both exclude lists, country phrases, and penalties live only in that file.
3. Stage 1 keeps today's child-workflow shape and reads those phrases from the profile. Stage 2 scores them. The browser does not send them.
4. One `jobs` row per vacancy (`id` is the dedup key), with `user_status` kept across recomputes.
5. `ProfileRunWorkflow`, `/api/v1/profiles/*`, and the Nuxt screens that call those routes.

**Paused (do not implement here)**

Later slices, in order: [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md), [`docs/03-vc-portfolio-boards/`](../03-vc-portfolio-boards/vc-portfolio-boards-concept.md), [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).

- `config/profiles/architecture.yaml` and any architecture collector. See [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).
- A company-tag signal. No `SignalCode` for it.
- `min_score` on the profile, on the query string, or as a list filter.
- Companies and ATS (`docs/04-companies-and-ats/`), VC portfolio boards (`docs/03-vc-portfolio-boards/`).
- LLM, CV parsing, profile create/update/delete over HTTP, auth, multi-user, scheduled auto-refresh, GCP.
- A second migration. Rewrite `000001` only (**M1**). Do not add `000002`.
- `internal/dictionary`, alias YAML, `EUROPE_HIRING_OK`, and any other hardcoded search list.

---

## 0. Cleanup

Old product path comes out before the new modules land. Ingest may still carry `slot_id` until **J3**. Do not add profiles, scoring, or job-identity columns in this section. Do not delete `queries.go` or `defaultStage2Exclude.ts` here; **P2** still reads them as the seed.

- [x] **K1** — Delete stage 3 and `internal/llm`, including every importer (`cmd/worker`, `cmd/agent`, `internal/pipeline`, `internal/manual`). Remove stage-3 HTTP (`post_stage3_run.go`, `stage3_run.schema.json`, stage-3 branches of stage-job routes and tests), stage-3 activities and utils (`pipeline/utils/scoring.go`, `stage3_outcome.go`, `stage3_cap.go`), Anthropic config (`internal/config/anthropic.go`, `Config.Anthropic*`, `Config.Pipeline`), and `JOBHOUND_ANTHROPIC_*` / `JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN` from `.env.example` and `docker-compose.yml`. Delete `domain/schema.ScoredJob` if nothing left references it. Stage 2, slots, and profile stay.
  **Done:** `make test` passes and no Go file imports `internal/llm`.

- [x] **K2** — Delete unloaded `Config` fields `TelegramBotToken`, `TelegramChatID`, `HTTPUserAgent`, `IncludeKeywords`, `ExcludeKeywords`. Make `github.com/go-rod/rod` a direct `require` (`go mod tidy`). Do not change browser fetch behavior.
  **Done:** `make test` and `make build` pass; `go.mod` has a direct rod require.

- [x] **K3** — Delete `internal/profile` and `GET|PUT /api/v1/profile` (handlers, `publicapi/schema/profile.go`, `profile_put` schema, tests). Drop the profile dependency from `handlers.Deps`, `cmd/api`, and the slots service. Remove the `SLOTS.PROFILE_REQUIRED` check. Leave the error constant itself until **E1**.
  **Done:** `make test` passes; `internal/profile` is gone; slot stage-2 routes still register.

- [x] **K4** — Remove slot HTTP only. Delete slot route files, slot tests, `publicapi/schema/slots.go`, `runs.go`, `stage.go`, `bucket.go`, `create_slot` / `stage2_run` / `patch_job_bucket` schemas, and the slot fields on `handlers.Deps`. `cmd/api` wires health only. Do not delete `internal/slots` yet (the worker still starts the manual workflow).
  **Done:** `make test` and `make build` pass; `/api/v1` registers `GET /api/v1/health` and nothing else.

- [x] **K5** — Unregister the v1 run path. `cmd/worker` stops registering manual and pipeline workflows. `cmd/agent` drops the noop pipeline path and `temporal_manual.go`. Worker still registers ingest and job retention.
  **Done:** `make build` passes; `cmd/worker` and `cmd/agent` do not import `internal/manual`, `internal/pipeline/workflows`, or `internal/llm`.

- [x] **K6** — Delete `internal/slots`, `internal/manual`, and the pipeline execution surface: `pipeline/impl`, `pipeline/mock`, `pipeline/workflows`, `pipeline/storage`, stage-2 files (`stage2_*.go`, `stage_rules.go`, `effective_run_job_status.go`), and `pipeline/contract.go` / `errors.go` once nothing imports them. Keep `internal/pipeline/utils` phrase-match helpers that ingest still calls, and the schema types those helpers need. Geo allow-list helpers are not a scoring input. Remove slot and pipeline-run methods from `jobs.JobRepository` and their tests. Delete `slots/utils.DefaultIngestSourceIDs`; the worker collector-map test asserts the map's own keys (himalayas omitted when nil, golang_cafe omitted when the rod fetcher is nil).
  **Done:** `make test` and `make build` pass; `internal/slots` and `internal/manual` are gone.

---

## 1. Config / profiles

- [x] **G1** — In `internal/config` only, add `JOBHOUND_PROFILES_DIR` (empty → `config/profiles`) and `JOBHOUND_JOB_RETENTION_DAYS` (default 30, reject non-positive). Document both keys in `.env.example`. No feature package calls `os.Getenv`. No dictionaries dir. Retention behavior still uses the old cutoff until **J2**.
  **Done:** the load path covers the defaults; `make test` passes.

- [x] **P1** — `internal/profiles/schema`: `Domain` and `Profile` from the contracts (`Queries`, `WellfoundRoles`, `ExcludeTitle`, `ExcludeText`, `Penalties`). `Domain` is a Canonical Enum (`String`, `Equals`, `Pointer`, `FromValue`, `ValuesDomain`, `FromStringDomain`). `internal/profiles/contract.go`: `Store` with `List` and `Get`, sentinels `ErrProfileNotFound` and `ErrProfileInvalid`. No HTTP, no file IO, no dictionary.
  **Done:** package builds. No enum-only test file.

- [x] **P2** — `internal/profiles/impl` file store and `config/profiles/andrew.yaml`. The store reads the directory on every `List`/`Get` (no cache). Strict decode. `sources` must be in an injected known-id set (the nine ids in the contracts); do not revive `DefaultIngestSourceIDs`. Unknown id → not found; malformed YAML or unknown source id → invalid. Fill the phrase lists by classifying `defaultStage2Exclude.ts` with the concept rules, and set `queries` / `wellfound_roles` from `queries.go` with `frontend` and `full-stack` removed from `queries`. Do not invent phrases. Do not delete the seed files in this task. Tests in `impl/*_test.go`: valid load, unknown key, unknown source id, and a second `Get` after the file changes.
  **Done:** those tests pass. `architecture.yaml` does not exist. `andrew.yaml` is the long classified list, not the short sample in the contracts.

- [x] **P3** — Delete the hardcoded copies now that YAML holds them. Remove `jobhound_frontend/app/utils/defaultStage2Exclude.ts`, the `EUROPE_HIRING_OK` geo rule and `stage2RulesFromKeywords` path in `app/utils/stagePayload.ts`, and every frontend call site that defaulted include, exclude, or country rules (including `slots-details.vue`). The old screen must compile without posting search phrases. Do not add a profile picker. Delete `internal/collectors/schema/queries.go` only if nothing in the still-live ingest path imports it; otherwise leave that delete to **W1**.
  **Done:** `rg` finds no `DEFAULT_STAGE2_EXCLUDE`, no `EUROPE_HIRING_OK`, and no client stage-2 rule builder. Frontend unit tests that remain still pass.

- [x] **U1** — Move the surviving phrase-match helpers from `internal/pipeline/utils` to `internal/scoring/utils`. Repoint ingest at the new path. Delete `internal/pipeline`. No scoring yet. Do not move a geo allow-list into scoring. Broad-filter code moves only if ingest still calls it; **J3** removes that call.
  **Done:** `make test` passes; `internal/pipeline` is gone.

---

## 2. Job identity

`jobs.id` is the dedup key. There is no `dedup_key` column. Unit tests keep building their own SQL. **M1**, after **S1**, is the only edit to `migrations/000001_*`.

- [x] **J1** — Replace the vacancy shape. `domain/schema.Job` loses `UserID`, `Stage1Status`, and the six location fields (`Remote`, `CountryCode`, `HiringCountries`, `HiringRegions`, `HiringRaw`, `TimezoneOffsets`). Add `Sources []string`, `CompanyKey`, `CompanyWebsite`, `Location` (`type`, `regions`, `countries`, `timezone`, `raw`), `FirstSeenAt`, `LastSeenAt`. `domain/utils`: company key is lowercased with `Inc` / `GmbH` / `Ltd` / `LLC` / `SRL` stripped. `id` is `company_key + "\x1e" + normalized title + "\x1e" + normalized location.raw`. Normalization is trim, collapse each whitespace run to one space, then lowercase. Empty raw is an empty segment. `apply_url` is not part of `id`. No stemmer, no stop-word list, no geo coder. Update collectors only enough to fill the new struct from data they already parse. `jobs` storage upsert: same id merges `sources`, ATS `url`/`apply_url` win over aggregator values, `first_seen_at` stays, `last_seen_at` moves. Storage tests: two sources with different apply URLs and the same company, title, and raw location become one row; `first_seen_at` stable; `last_seen_at` moves.
  **Done:** `make test` passes, including those storage cases. Do not audit boards for missing fields (**J4**).

- [x] **J2** — Retention uses `last_seen_at`, not `created_at`. `jobs/utils` cutoff reads `JOBHOUND_JOB_RETENTION_DAYS` from the config struct passed in (default 30). Update the retention activity, `cmd/retention` text, and retention tests. This is not the 10-day score window.
  **Done:** a row last seen 31 days ago is deleted; a row last seen inside the window is not. `make test` passes.

- [x] **J3** — Ingest loses the slot. Redis keys become `ingest:lock:{source_id}:{query_segment}` and `ingest:cooldown:{source_id}:{query_segment}`; empty query keeps the `catalog` segment. `ingest_watermarks` primary key is `(source_id)` only. `IngestSourceInput` drops `SlotID` and keeps `SourceID` + query. Delete `ingest.ErrNilSlotID` and slot fields on the broad-filter key. Stop applying the slot broad filter inside ingest; delete that helper once unused. Update coordinator, watermark, and activity tests.
  **Done:** `make test` passes; no ingest symbol mentions `slot`.

- [x] **J4** — Audit the nine collectors (`europe_remotely`, `working_nomads`, `himalayas`, `remotify_europe`, `we_work_remotely`, `wellfound`, `vue_jobs`, `golang_cafe`, `builtin`). Where the board already exposes an apply/ATS link or a company website, map it to `ApplyURL` and `CompanyWebsite`. Do not add sources, fetches, or parsers for fields the board does not show.
  **Done:** each collector test that already has a fixture asserts the mapped fields; `make test` passes.

---

## 3. Scoring

Pure function first, then rows, then the use case, then Temporal. No Temporal SDK under `impl/`.

- [x] **T1** — `internal/scoring/schema`: `Bucket` (`PASSED`, `REJECTED` only), `UserStatus`, `RunStatus`, `SignalCode` (`QUERY`, `PENALTY`, `CUT_EXCLUDE_TITLE`, `CUT_EXCLUDE_TEXT`), `Signal`, `Match`. Constants `QueryPoints` = 1, `PenaltyPoints` = -10, `ScoreWindowDays` = 10. Every exported string enum is a Canonical Enum. No company-tag code, no freshness code, no role code.
  **Done:** package builds. No enum-only test file.

- [x] **T2** — `internal/scoring/contract.go`: `API` (`StartRun`, `LatestRun`, `ListJobs`, `SetUserStatus`), sentinels `ErrRunAlreadyRunning`, `ErrJobNotInScope`, `ErrNoRun`. Add a `RunStarter` interface with one method that takes `profileID` and `runID` and returns `error`. The impl calls it; workflows implement it. No Temporal types in this file.
  **Done:** package builds.

- [x] **I1** — `internal/scoring/impl` pure evaluation of one job, no DB and no network. Order is the concept's core loop: `exclude_title` against the title, then `exclude_text` against title plus description, then +1 once per `queries` phrase in that same text, then −10 once per `penalties` phrase. A reject stops the pass and is stored as a signal. Otherwise the bucket is `PASSED`, including score 0 and negative scores. Case-insensitive, word boundaries. Table tests in `impl/*_test.go` cover the contract test list: title exclude first, text exclude, penalty keeps `PASSED`, each query phrase once, case fold, `java` versus `javascript`, `frontend` adds nothing, score 0 is `PASSED`.
  **Done:** that table passes. No `min_score`, no section headings, no dictionary.

- [x] **S1** — `internal/scoring/storage`: GORM models and repository for `profile_runs` (`RUNNING` / `SUCCEEDED` / `FAILED`, counters, unique `idempotency_key`) and `profile_matches`. Upsert writes `bucket`, `score`, `signals`, `run_id`, `updated_at` and does not put `user_status` in the update set. `List` filters optional `bucket` (default `PASSED`) and `user_status` (omitted means `NEW`), pages `page`/`limit`, sorts `score DESC, first_seen_at DESC`. Storage test: upsert overwrites score and bucket and leaves `HIDDEN` in place.
  **Done:** that test passes against the repository.

- [x] **M1** — Rewrite `migrations/000001_initial_schema.{up,down}.sql` to the v2 shape in the contracts: `jobs` (id is the dedup key, `sources`, location jsonb, `company_key`, `company_website`, `first_seen_at`, `last_seen_at`, indexes on `first_seen_at`, `last_seen_at`, and `company_key`), `profile_runs`, `profile_matches` (composite PK, `PASSED`/`REJECTED` check, upsert must be able to omit `user_status`, index `(profile_id, bucket, user_status, score DESC)`), `ingest_watermarks` without `slot_id`. Drop `slots`, `slot_jobs`, `slot_idempotency_keys`, `user_profile`, `pipeline_runs`, `pipeline_run_jobs`. Fix tests that execute this file. No `000002`.
  **Done:** up and down SQL match the GORM models from **J1** and **S1**; `make test` passes.

- [x] **I2** — `internal/scoring/impl` use cases on top of **I1**, **S1**, `profiles.Store`, and the jobs repository. `StartRun` rejects a second open run and returns the same run when the idempotency key repeats; the same key with a different profile is a conflict. `LatestRun` returns `ErrNoRun` when the profile has none. Scoring loads jobs with `first_seen_at` inside `ScoreWindowDays` before that run's `started_at`, evaluates each one (including rejected), and upserts. A failed score attempt does not delete existing matches. `SetUserStatus` returns `ErrJobNotInScope` when this profile has no match. `ListJobs` defaults to `PASSED` and `NEW`. Tests use fakes, not Postgres.
  **Done:** table tests cover the conflict, the idempotency replay, a row older than 10 days not being scored, a rejected row being stored, and a hidden row staying hidden after rescore. `impl/` does not import the Temporal SDK.

- [x] **W1** — `internal/scoring/workflows`: `ProfileRunWorkflow` and activities `ScoreProfile`, `MarkRunFinished`. `New...` calls `RegisterWorkflow`. Workflow id `profile-run-{profile_id}`, reuse `ALLOW_DUPLICATE` after close. Input is `ProfileRunInput{ProfileID, RunID}` only; activities re-read the YAML. The run expands `sources` × `queries` into existing `IngestSourceWorkflow` children (keyword children, `wellfound_roles` children, one catalog child for `we_work_remotely` / `vue_jobs` / `golang_cafe` with `queries` ignored). Wellfound slugs are not scored. A locked, cooling-down, rate-limited, or not-built collector child is skipped, logged with its source id, and counted in `sources_skipped`. It is not a failed run. Then score, then `SUCCEEDED` with counters. Temporal-to-domain mapping stays in `workflows/mappers.go`. Delete `internal/collectors/schema/queries.go` if **P3** left it. SDK test: stub collector, two jobs, buckets asserted, and `user_status` still `HIDDEN` on a second pass. A listed source with no collector in the worker map increments `sources_skipped` and the run still succeeds.
  **Done:** that workflow test passes. No Go file defines the old query matrix.

- [x] **W2** — `cmd/worker` constructs the scoring workflows and registers them on the existing worker. Pass the profiles directory from config. No business rules in `main`.
  **Done:** `make build` passes; the worker binary registers `ProfileRunWorkflow`.

---

## 4. HTTP

Handlers call `scoring.API` and `profiles.Store`. They do not score, and they do not accept search phrases.

- [x] **E1** — Replace the slot/stage/pipeline error codes with the table in the contracts. Idempotency codes move to the `HTTP.` prefix. Add `PROFILES.NOT_FOUND`, `PROFILES.INVALID_DEFINITION`, `PROFILES.RUN_ALREADY_RUNNING`, `PROFILES.NO_RUN`, `PROFILES.JOB_NOT_IN_SCOPE`, `PROFILES.INVALID_USER_STATUS`. Delete `SlotLimitReachedBody`; every error body is `{ "error": { "code", "message" } }`. Run `go generate` in `internal/publicapi/schema` and `scripts/gen-api-error-codes.ts` in `jobhound_frontend` so `api-error-registry.json` and `app/types/generated/apiErrorCodes.ts` match.
  **Done:** `handlers/api_error_test.go` passes against the new registry.

- [x] **T3** — `internal/publicapi/schema` DTOs for the five profile routes: profile list item (`id`, `name`, `domain`, `sources`, `queries`), run (`run_id`, `profile_id`, `status`, timestamps, `jobs_scored`, `sources_skipped`), job page (`page`, `limit`, `total`, job entry with location, sources, bucket, score, signals, `user_status`), and the patch body `{ "user_status" }`. `DisallowUnknownFields` on decode. No exclude or penalty block on the profile list item. No run request body.
  **Done:** package builds.

- [x] **H1** — `handlers/json_schema/patch_user_status.schema.json` (enum `NEW` | `HIDDEN`) embedded from `schemas_embed.go`. Delete the slot, profile, and stage schema files if **K4** left any. POST runs has no body schema.
  **Done:** the embed builds; old schema files are gone.

- [x] **H2** — `GET /api/v1/profiles` in its own route file, registered in `registerRoutes`. File-name order. `200` body is the list DTO. Handler test with a fake `profiles.Store`.
  **Done:** the handler test passes.

- [x] **H3** — `POST /api/v1/profiles/{profile_id}/runs`. Require `Idempotency-Key` (non-nil UUID), no body. A JSON body is `400`. `202` with the run DTO. Map sentinels: `400` missing or invalid key, `404` unknown profile, `400` invalid definition, `409` run already open or key reused for another profile. Replay of the same key returns the same run. Handler test covers 202, 409, the two 400 key cases, and a rejected body.
  **Done:** those handler tests pass.

- [x] **H4** — `GET /api/v1/profiles/{profile_id}/runs/latest`. `200` run DTO. `404` for an unknown profile and for `ErrNoRun`. Handler test covers both.
  **Done:** those handler tests pass.

- [x] **H5** — `GET /api/v1/profiles/{profile_id}/jobs`. Query `bucket` (`PASSED` or `REJECTED`, default `PASSED`), `user_status` (`NEW` or `HIDDEN`; omitted means `NEW`), `page` (≥1), `limit` (1–100). `400` `HTTP.INVALID_QUERY` on a bad query. `404` unknown profile. Response is the job page. No score threshold. There is no query value that returns both statuses. Handler test covers the bad query, the default of `PASSED` and `NEW`, and an explicit `REJECTED` page.
  **Done:** those handler tests pass.

- [x] **H6** — `PATCH /api/v1/profiles/{profile_id}/jobs/{job_id}` via `ReadValidatedJSON` into the patch DTO. `200` returns the updated job entry. `400` invalid status, `404` unknown profile or `ErrJobNotInScope`. Handler test covers 200 and both 404 causes.
  **Done:** those handler tests pass.

---

## 5. Wiring

- [x] **C1** — `cmd/api` composition only: load config, construct the profile file store (known source ids = the nine collector ids) and the scoring service, and pass them as `publicapi` handler deps. No scoring or YAML rules in `main`. No dictionary.
  **Done:** `make build` passes; `cmd/api` does not import `internal/slots`, `internal/profile`, `internal/llm`, or `internal/dictionary`.

- [x] **C2** — Mount profiles into the `api` and `worker` images. `Dockerfile` copies `config/profiles` next to the binaries. `docker-compose.yml` sets `JOBHOUND_PROFILES_DIR` for `api` and `worker`. `.env.example` already lists the keys from **G1**; do not add a second env list to the Makefile or README. No dictionaries directory.
  **Done:** both services receive the profiles directory; `make build` passes.

- [x] **C3** — Rewrite `docs/REPO_SNAPSHOT.md` so it describes this slice (profiles from YAML, stage 1 unchanged in shape, stage 2 phrase score, job identity, the five routes, the Nuxt profile list and profile page) and no longer documents slots, stage 3, Anthropic, the global profile text, or client-side keyword defaults as current behavior.
  **Done:** the snapshot's current-behavior sections match the concept. Do not create `jobhound_frontend/docs/01-profiles-and-scoring/`.

---

## 6. Frontend

Nuxt tasks in `jobhound_frontend`. Pages call `app/api`. No Pinia store. No keyword editor and no profile-text page. Reuse the existing page shell and `PaginatedDataTable`. Do not create `jobhound_frontend/docs/01-profiles-and-scoring/`.

- [x] **F1** — Add the profile client beside the current one in `app/types/` and `app/api/publicApi.ts`: `getProfiles`, `postProfileRun`, `getLatestProfileRun`, `getProfileJobs`, `patchProfileJob`. DTOs: profile list item (`id`, `name`, `domain`, `sources`, `queries`), run, job page, patch body `{ "user_status" }`. `postProfileRun` sends `Idempotency-Key` and no body. `getProfileJobs` sends `bucket`, `user_status`, `page`, and `limit` only when the caller set them. Keep the current `requestJson` shape: named exports, `Promise<T>`, `catch (e: unknown)` then `toApiError`. Do not delete the slot or profile-text functions in this task; existing pages still call them.
  **Done:** `pnpm typecheck` passes. Those five functions exist.

- [x] **F2** — Replace `app/pages/index.vue` with the profile list from `getProfiles`. Show `name`. No create, no delete, no three-slot cap. Link each row to `/profiles/{id}`. Delete `app/pages/profile.vue`. In `app/components/TheHeader.vue`, the only nav target is `/`. Delete `getProfile`, `putProfile`, `getSlots`, `postSlots`, and `deleteSlot`, and delete the types that then have no caller, including `app/types/IProfile.ts`.
  **Done:** `pnpm typecheck` passes. `rg` finds no `pages/profile.vue` and no `postSlots`.

- [x] **F3** — Add `app/pages/profiles/[id].vue` and delete `app/pages/slots-details.vue`. Load the latest run and the job list. The run button calls `postProfileRun` with a new UUID and an empty body. While status is `RUNNING`, poll `getLatestProfileRun`. Show status, `jobs_scored`, and `sources_skipped` when the run payload has them. The default list omits `bucket` and `user_status`. One control requests `bucket=REJECTED`. One control requests `user_status=HIDDEN` and can patch a row back to `NEW`. Hide calls `patchProfileJob` with `HIDDEN`. Render `error.message` on failure, including an already-open run. Do not render exclude or penalty lists. Delete the remaining slot and stage client functions and types, `app/components/KeywordLineTextarea.vue`, and the stage-2 keyword helpers in `app/utils/stagePayload.ts` once nothing imports them.
  **Done:** `pnpm typecheck` and `pnpm test:unit` pass. `rg` finds no `slots-details`, no `KeywordLineTextarea`, and no `stage2RulesFromKeywords`.

---

## Dependencies

```
K1 → K2 → K3 → K4 → K5 → K6
K6 → G1
K6 → P1 → P2 → P3
K6 → U1
K6 → J1 → J2
G1 → J2
J1 → J3 → J4
J1 → T1 → T2 → I1
U1 → I1
P2 → I1
T1 → S1 → M1
I1 + S1 + P2 + J1 → I2
I2 + J3 + P3 → W1 → W2
J1 + J2 + J3 → M1
K6 → E1 → T3 → H1
T2 + T3 + H1 → H2, H3, H4, H5, H6
W2 + H2..H6 + M1 → C1 → C2 → C3
E1 → F1
P3 → F2
F1 → F2 → F3
F3 → C3
```

**Milestone 1:** K1–K6. v1 product path is gone; ingest and retention still run.
**Milestone 2:** G1, P1, P2, P3, U1. YAML is the only search-phrase source; `internal/pipeline` is gone.
**Milestone 3:** J1, J2, J3, J4. Job identity and slot-free ingest.
**Milestone 4:** T1, T2, I1, S1, M1, I2, W1, W2. One profile run scores and persists.
**Milestone 5:** E1, T3, H1–H6, F1–F3, C1, C2, C3. HTTP, the Nuxt screens, and compose match the contracts.

H2–H6 do not depend on each other. J4 does not block scoring.
