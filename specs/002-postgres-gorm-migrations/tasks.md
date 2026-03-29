# Tasks: PostgreSQL, GORM, migrations

**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/*`  
**Tests**: REQUIRED for migrations/connection/mapping per acceptance criteria (extend `go test ./...`).

## A. Contracts & documentation

1. [ ] **Freeze env contract** — Definition of done: `contracts/environment.md` matches implemented var names; Makefile `help` lists DSN + migrate-related names; README section points to contracts.
2. [ ] **Freeze jobs table contract** — Definition of done: `contracts/jobs-schema.md` matches first migration and `domain.Job` mapping rules.

## B. Local stack

3. [ ] **Docker Compose for Postgres** — Definition of done: service with pinned image (e.g. `postgres:16`), named volume, port, `healthcheck`, `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` aligned with documented DSN example.
4. [ ] **Document connection from host** — Definition of done: README or spec cross-link: example `JOBHOUND_DATABASE_URL` for Compose default credentials.

## C. Dependencies & DB bootstrap

5. [ ] **Add Go modules** — Definition of done: `go.mod` includes GORM, postgres driver, golang-migrate (and any CLI wrapper deps); `go mod tidy` clean.
6. [ ] **Implement DB open package** — Definition of done: single place opens `*gorm.DB` from env; exposes `GormGetter` (or equivalent) for storage; uses `WithContext` pattern; sensible pool defaults or env-tuned values documented.
7. [ ] **Dedicated migrate path** — Definition of done: `cmd/migrate` and/or Makefile targets apply `migrations/` up/down; **agent binary does not** run migrations on ordinary start (prod-safe default).

## D. Migrations

8. [ ] **Initial SQL migration: `jobs`** — Definition of done: up creates table per `spec.md` + `contracts/jobs-schema.md` (PK `id` text, columns, `created_at`/`updated_at`); down drops table (or safe downgrade per team preference).
9. [ ] **Migration idempotence** — Definition of done: second `up` at latest version does not error (document behaviour in README if non-obvious).

## E. Storage layer

10. [ ] **GORM model + `TableName()`** — Definition of done: lives under `internal/.../storage/...`; no GORM imports under `internal/domain`.
11. [ ] **Mapping** — Definition of done: `NewModel(domain.Job)` (or symmetric name) and `ToDomain()` (or `(*Model).ToDomain()`) cover all fields including `PostedAt` NULL ↔ zero time, `UserID` nil ↔ NULL.
12. [ ] **Repository stub** — Definition of done: minimal iface + implementation: e.g. `Upsert` or `Save` and/or `GetByID` — enough to prove wiring; exact methods as needed for `006` can stay TODO with clear comments **or** smallest viable API per plan.

## F. Tests

13. [ ] **Tests: migrations + schema** — Definition of done: integration-style test against Compose Postgres (CI script or `testcontainers` / `-short` skip documented) **or** documented manual step + unit tests for SQL parsing — **minimum**: automated test that runs migrate up and checks `jobs` exists with expected columns (choose one approach in implementation).
14. [ ] **Tests: mapping** — Definition of done: table-driven tests for `ToDomain` / `NewModel` round-trip and NULL/zero edge cases.

## G. Optional / deferred (do not block `002` closure)

15. [ ] **Stub tables for runs/events** — **Deferred** by plan D3; create tasks under `008` when needed.
16. [ ] **Wire Dedup port to real storage** — Belongs to `006` unless spec overlap; leave noop until then.
