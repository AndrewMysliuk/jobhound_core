# Implementation Plan: PostgreSQL, GORM, migrations

**Branch**: `002-postgres-gorm-migrations`  
**Date**: 2026-03-29  
**Last Updated**: 2026-04-03  
**Spec**: `specs/002-postgres-gorm-migrations/spec.md`  
**Input**: Feature specification + `research.md` + [`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Add **PostgreSQL** via **GORM**, **versioned SQL migrations** with **golang-migrate**, and a **storage layer** for `jobs` with bidirectional mapping to **`domain.Job`** (no GORM in `internal/domain`). Provide **Docker Compose** for local Postgres, document **env vars** in Makefile/README, and a **dedicated migrate entrypoint** so the agent binary does not auto-migrate in production. **No** `go-common` or Omega internal libraries. **SQLite** is out of scope. Optional run/event tables are **deferred** to **`008` / `009`** unless a concrete blocker appears (see Resolved decisions).

**MVP narrative (2026-04-02)**: **`jobs`** is the **canonical** vacancy table (one PK per stable id). **Slot-scoped pools** attach via **downstream** schema (`007` / `006` / `009`), not by duplicating `jobs` rows per slot—see spec “Alignment with MVP”.

## Technical Context

**Language/Version**: Go 1.24  
**Database**: PostgreSQL (pin major version in Compose, e.g. **16**)  
**ORM**: GORM with Postgres driver; single lifecycle for `*gorm.DB`  
**Migrations**: `github.com/golang-migrate/migrate/v4` + Postgres driver; SQL files under repo-root **`migrations/`** (see Resolved decisions)  
**Testing**: `go test ./...`; add tests for migration application (or schema assertions) and DB ping / storage mapping where practical  
**Layout**: `internal/jobs/storage/` with `model.go`, `repository.go`, optional `contract.go`; shared **`internal/platform/pgsql`** for `Open` / `OpenFromEnv` + `GormGetter`

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| IV. Postgres as system of record | **PASS** |
| VI. Config without secrets in repo | **PASS** (document names only; `.env` gitignored) |
| Layering: domain without storage imports | **PASS** (GORM models and tags only under storage) |
| No SQLite in target architecture | **PASS** |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Research | `research.md` |
| 1 Contracts | `contracts/environment.md`, `contracts/jobs-schema.md` |
| 2 Tooling & Compose | `docker-compose.yml` (or `compose.yaml`), volume + healthcheck, aligned `POSTGRES_*` |
| 3 Dependencies | `go.mod`: GORM, migrate, drivers; optional `cmd/migrate` or Makefile-only migrate |
| 4 Migrations | `migrations/*.up.sql` / `*.down.sql` for `jobs` table per spec |
| 5 GORM + getter | Open DB from env; export `GormGetter` / constructor pattern for repos |
| 6 Storage package | `model` + `NewModel` / `ToDomain` (and reverse); `WithContext` on queries |
| 7 Wire & docs | Makefile targets (`migrate-up` / `migrate-down` or `db-migrate`), README env section |
| 8 Tests | Migration + connection + mapping tests as agreed in tasks |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **Migrations directory** | **`migrations/`** at repo root; filenames follow golang-migrate convention (`000001_....up.sql`). |
| D2 | **Postgres version (local)** | **16** in Compose and docs unless CI forces another LTS. |
| D3 | **Run / event stub tables** | **Defer** to Temporal / **`008`–`009`** specs; ship **`jobs` only** in `002` to keep v0 tight (spec “optional v0” → choose minimal). |
| D4 | **Migrate invocation** | **Dedicated** command: prefer **`cmd/migrate`** *or* documented **`make migrate-up`** wrapping `migrate` CLI / small Go main — **not** inside `cmd/agent` default startup for prod. |
| D5 | **DSN env var** | Single primary: **`JOBHOUND_DATABASE_URL`** (Postgres URL form). If migrate tool needs a separate var, document **`JOBHOUND_MIGRATE_DATABASE_URL`** as optional override; default to same as D1 in `contracts/environment.md`. |
| D6 | **GORM AutoMigrate** | **Not used** for schema authority; SQL migrations only. |
| D7 | **`posted_at` zero value** | Domain zero `time.Time` ↔ SQL **NULL** in mapping (spec). |
| D8 | **`user_id`** | Nullable text; domain `*string` nil/empty ↔ NULL. |

## Engineering follow-ups (non-blocking)

- Index tuning (`source`, `posted_at`) when `006` ingest query patterns are fixed.
- Temporal + run history schema in `003` / `008` once workflow IDs and payloads are stable.
- When **`slot_id`** (and related) land on **`pipeline_runs`** or sibling tables, ensure **ON DELETE** behavior matches product draft §2 (slot delete → no orphans referencing that slot).

## Project structure (documentation)

```text
specs/002-postgres-gorm-migrations/
├── spec.md
├── plan.md
├── research.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── environment.md
    └── jobs-schema.md
```

## Source structure (anticipated — implementation phase)

```text
migrations/
├── 000001_init_jobs.up.sql
├── 000001_init_jobs.down.sql
└── ...
internal/
├── platform/pgsql/        # Open, pool from config, GormGetter (infrastructure)
└── jobs/storage/
    ├── model.go
    ├── repository.go
    └── …                  # JobRepository impl; optional extra files
cmd/
├── agent/
└── migrate/               # optional thin CLI for golang-migrate
docker-compose.yml         # postgres service
```
