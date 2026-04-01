# Tasks: Cache and ingest

**Input**: `spec.md`, `plan.md`, `contracts/*`  
**Depends on**: `001`, `002`, `005`, **`007`** (`pipeline_runs`, `jobs.stage1_status` / per `pipeline-run-job-status.md`, `pipeline_run_jobs` CASCADE), `003` (worker / schedules).  
**Tests**: Unit tests without mandatory network; `integration` tag optional for DB/Redis.

## A. Contracts & docs

1. [x] **Contracts match intent** — Definition of done: `contracts/*.md` align with `spec.md` and `plan.md` resolved decisions; stage-1 column references **`007`** contract only.

## B. Config & Redis

1. [x] **`internal/config`** — Definition of done: `JOBHOUND_REDIS_URL`, `JOBHOUND_INGEST_EXPLICIT_REFRESH` loaded and documented in `contracts/environment.md`; no raw `os.Getenv` in feature packages.

2. [x] **Redis coordination** — Definition of done: lock `ingest:lock:{source_id}`, cooldown `ingest:cooldown:{source_id}` per `redis-ingest-coordination.md`; TTL defaults **600** / **3600** s; **fail closed** if Redis unavailable.

3. [x] **Explicit refresh** — Definition of done: when enabled, bypass **cooldown** only; **lock** still acquired before work.

## C. Migrations & schema

1. [x] **`ingest_watermarks`** — Definition of done: table per `ingest-watermark-and-filter-key.md`; `up`/`down` under `migrations/`.

2. [x] **`pipeline_runs.broad_filter_key_hash`** — Definition of done: nullable `TEXT` (hex) added **after** `007` creates `pipeline_runs`, per contract; safe `down` for dev.

3. [x] **Indexes** — Definition of done: stage-1 read paths and retention (`jobs.created_at` if needed) per `spec.md` / `retention-jobs.md`.

## D. Ingest logic

1. [x] **Upsert & skip** — Definition of done: upsert by stable id; equality excludes `description`, excludes `created_at`/`updated_at`; description-only change still updates row.

2. [x] **Broad filter key** — Definition of done: canonical JSON + SHA-256 hex per `ingest-watermark-and-filter-key.md`; stored on `pipeline_runs` for the run.

3. [x] **`PASSED_STAGE_1`** — Definition of done: set `jobs` stage-1 column when broad stage 1 completes — values/names per **`007`** contract.

4. [x] **Watermark** — Definition of done: read/write `ingest_watermarks` when collector (`005`) supports incremental; otherwise full fetch, cursor unused.

5. [x] **Downstream on description-only change** — Definition of done: ingest does **not** reset `pipeline_run_jobs` / stage 2–3 state (per `plan.md` D7).

## E. Collectors & orchestration

1. [x] **Collector integration** — Definition of done: ingest activities call `005` collectors; respect lock/cooldown/refresh.

2. [x] **Temporal** — Definition of done: workflows/activities registered per `003` patterns; ingest entrypoints wired in `cmd/worker` as needed.

## F. Retention

1. [x] **Scheduled cleanup** — Definition of done: cron **every 7 days UTC** runs retention delete per `retention-jobs.md`.

2. [x] **Manual cleanup** — Definition of done: same delete semantics available via documented manual trigger (CLI or workflow).

3. [x] **FK safety** — Definition of done: deleting `jobs` removes `pipeline_run_jobs` rows (**CASCADE** or same transaction) — aligned with **`007`** migrations.

## G. Local / Docker

1. [x] **Compose** — Definition of done: Redis service + documented `JOBHOUND_REDIS_URL` for local dev.

## H. Quality gates

1. [x] **`make test` / `go test ./...`** — Definition of done: passes without mandatory network for default tests.

2. [x] **`make vet` / `make fmt`** — Definition of done: clean for touched packages.

## I. Optional / deferred

1. [x] **Integration tests** — Definition of done: `//go:build integration` applies migrations and/or Redis mini test if adopted.

2. [x] **Env TTL overrides** — Definition of done: **Implemented** — `JOBHOUND_INGEST_LOCK_TTL_SEC` / `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` in `internal/config/ingest.go` + `contracts/environment.md`; worker passes TTLs into `ingest.NewRedisCoordinatorWithTTL`.
