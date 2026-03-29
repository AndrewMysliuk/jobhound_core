# Tasks: PostgreSQL, GORM, migrations

**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/*`  
**Tests**: REQUIRED for migrations/connection/mapping per acceptance criteria (extend `go test ./...`).

## A. Contracts & documentation

1. [x] **Freeze env contract** — Definition of done: `contracts/environment.md` matches implemented var names; Makefile `help` lists DSN + migrate-related names; README section points to contracts.
2. [x] **Freeze jobs table contract** — Definition of done: `contracts/jobs-schema.md` matches first migration and `domain.Job` mapping rules.

## B. Local stack

1. [x] **Docker Compose for Postgres** — Definition of done: service with pinned image (e.g. `postgres:16`), named volume, port, `healthcheck`, `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` aligned with documented DSN example.
2. [x] **Document connection from host** — Definition of done: README or spec cross-link: example `JOBHOUND_DATABASE_URL` for Compose default credentials.

## C. Dependencies & DB bootstrap

1. [x] **Add Go modules** — Definition of done: `go.mod` includes GORM, postgres driver, golang-migrate (and any CLI wrapper deps); `go mod tidy` clean.
2. [x] **Implement DB open package** — Definition of done: single place opens `*gorm.DB` from env; exposes `GormGetter` (or equivalent) for storage; uses `WithContext` pattern; sensible pool defaults or env-tuned values documented.
3. [x] **Dedicated migrate path** — Definition of done: `cmd/migrate` and/or Makefile targets apply `migrations/` up/down; **agent binary does not** run migrations on ordinary start (prod-safe default).

## D. Migrations

1. [x] **Initial SQL migration: `jobs`** — Definition of done: up creates table per `spec.md` + `contracts/jobs-schema.md` (PK `id` text, columns, `created_at`/`updated_at`); down drops table (or safe downgrade per team preference).
2. [x] **Migration idempotence** — Definition of done: second `up` at latest version does not error (document behaviour in README if non-obvious).

## E. Storage layer

1. [x] **GORM model + `TableName()`** — Definition of done: lives under `internal/.../storage/...`; no GORM imports under `internal/domain`.
2. [x] **Mapping** — Definition of done: `NewModel(domain.Job)` (or symmetric name) and `ToDomain()` (or `(*Model).ToDomain()`) cover all fields including `PostedAt` NULL ↔ zero time, `UserID` nil ↔ NULL.
3. [x] **Repository stub** — Definition of done: minimal iface + implementation: e.g. `Upsert` or `Save` and/or `GetByID` — enough to prove wiring; exact methods as needed for `006` can stay TODO with clear comments **or** smallest viable API per plan.

## F. Tests

1. [x] **Testтогs: migrations + schema** — Definition of done: integration-style test against Compose Postgres (CI script or `testcontainers` / `-short` skip documented) **or** documented manual step + unit tests for SQL parsing — **minimum**: automated test that runs migrate up and checks `jobs` exists with expected columns (choose one approach in implementation).
2. [x] **Tests: mapping** — Definition of done: table-driven tests for `ToDomain` / `NewModel` round-trip and NULL/zero edge cases.

## G. Optional / deferred (do not block `002` closure)

1. [x] **Stub tables for runs/events** — **Deferred** by plan D3; follow-up tasks live in `specs/008-events-and-run-history/tasks.md`.
2. [x] **Wire Dedup port to real storage** — **Closed for `002`**: intentionally deferred to `006` (cache/ingest); no separate dedup schema in `002` (`jobs` only per plan D3). Agent keeps `mock.Dedup` in `cmd/agent` until `006` adds persistence (see `specs/006-cache-and-ingest/spec.md`).

