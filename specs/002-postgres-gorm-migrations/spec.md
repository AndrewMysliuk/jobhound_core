# Feature: PostgreSQL, GORM, migrations

**Feature Branch**: `002-postgres-gorm-migrations`  
**Created**: 2026-03-29  
**Last Updated**: 2026-03-29  
**Status**: Draft

## Goal

Connect to **PostgreSQL** with **GORM**, add **versioned SQL migrations** (same approach as omg-api), and place persistence in a **storage layer** with a clear split between **domain** and **GORM models**. Provide local **Docker Compose** for Postgres and documented env vars. The v0 schema covers **jobs** and, if needed, **minimal stubs** for runs/events ahead of `006` / `008` / `011`. **SQLite is not part of the target architecture** (see constitution).

## Style reference (omg-api)

Patterns follow the omg-api reference (dedicated storage packages, `GormGetter`, golang-migrate). **Do not add dependencies on `github.com/omgbank/go-common`, omg-api source, or other Omega internal libraries** — implement the same **patterns** inside `jobhound_core`.

## Connection and GORM lifecycle

- DSN and pool settings come from **environment**; secrets and local `.env` stay out of git (variable names documented in Makefile / README as they land).
- GORM is initialized in **one place** (`cmd` and/or a small `internal/...` package, e.g. `internal/storage/pgsql` or `internal/db`).
- Repositories obtain `*gorm.DB` **like omg-api**: a type alias or **`GormGetter`** `func() *gorm.DB` passed into storage constructors and used as `dbGetter().WithContext(ctx)…`. This keeps a single connection lifecycle and simplifies tests.
- Optional GORM open options in the same spirit as omg-api where useful: e.g. `TranslateError`, pool limits, timeouts — the exact list is fixed in the implementation plan.

## Migrations

- Tool: **`github.com/golang-migrate/migrate/v4`** with the Postgres driver (as in omg-api).
- Schema is defined by **SQL files** in the repo (e.g. `migrations/` or a name agreed in the plan).
- Up/down runs via a **dedicated command** (Makefile target and/or small `cmd/migrate`, and/or a Compose step — per plan). The **agent binary must not** silently run migrations on every prod start (a dev-only helper is allowed if called out in the plan).
- **GORM AutoMigrate** is not the source of truth for schema; use SQL migrations only.

## Storage layer and domain separation

- Packages like `internal/.../storage/<entity>/` (or equivalent): **`model.go`** (GORM struct + `TableName()`), **`pgsql.go`** (implementation), and optionally **`contract.go`** (storage interface).
- **`internal/domain`** stays free of GORM tags and GORM imports.
- Required mapping: **`NewModel(domain…)`** (or a symmetric name) and **`ToSchema()`** / **`ToDomain()`** on the storage model for both directions. Future complex fields may use `jsonb` with explicit marshal/unmarshal in these functions.
- Use **`WithContext(ctx)`** on every query path. Use `Transaction` on `*gorm.DB` where atomicity is required.

## Initial schema (v0)

### Table `jobs` (required)

Purpose: normalized vacancy row and dedup by **stable id** from `001`.

| Logical field   | Notes |
|-----------------|--------|
| Primary key     | **`id`** = stable job id from `domain` (`StableJobID` / `Job.ID`), **text**; uniqueness enforced by PK. |
| `source`        | text, not null |
| `title`, `company` | text |
| `url`           | text (listing); extra indexes for query patterns optional per plan |
| `apply_url`     | text, nullable |
| `description`   | text |
| `posted_at`     | `timestamptz`, nullable (zero in domain → NULL) |
| `user_id`       | text, nullable (reserved for multi-user from `001`) |
| `created_at` / `updated_at` | `timestamptz`, for audit and upsert (details in migration) |

Indexes: at least PK on `id`; others (e.g. `source`, time) as needed for ingest spec `006`.

### Tables for runs / events (optional v0)

If `002` includes stubs for `008` / Temporal `003`:

- Table names and columns are **owned by migrations** so later specs can extend them; allow **one minimal** run-history table (id, timestamps, status, nullable `temporal_workflow_run_id`, nullable `payload` jsonb) and/or a **minimal** event row — exact columns in the plan, or defer to `008` if `002` ships **only** `jobs`.

**Rule**: keep v0 small; prefer a tight `jobs` table plus an explicit follow-up in `008` over speculative columns.

## Docker Compose (local)

- **PostgreSQL** service for development (image and major version per plan; pin one version, e.g. 16, in compose and docs).
- Data volume, port, healthcheck; `POSTGRES_*` vars aligned with app DSN and the migrate command.
- **Temporal** may land in `003`; it is **not required** for spec `002` if milestones match `000`.

## Integration with ports and cmd

- Port implementations in `internal/ports` (e.g. future `Dedup` / job store) wire in **`cmd/agent`** once contracts exist in `006`. For **`002`**, deliver: working DB connection, applied migrations, and a **storage package stub** for `jobs` (plus migration / ping tests) if the plan splits work that way.
- Layering from `001` holds: domain must not import storage.

## Out of scope

- Final cache/watermark schema, full events/schedules model, HTTP API — see `006`, `008`, `011`.
- `go-common`, Debezium, CDC handlers.
- Production GCP setup / secrets beyond documenting env var names.

## Dependencies

- **`001-agent-skeleton-and-domain`**: `Job`, stable id, fields and meaning of `UserID` / URLs.
- **`000-epic-overview`**: local stack with Postgres via Compose.

## Acceptance criteria

1. `docker compose` (or a documented equivalent) starts Postgres; the app can open a connection using env DSN.
2. A migrate command/target applies SQL through the latest version; re-runs behave per golang-migrate semantics.
3. The **`jobs`** table exists and matches the “Initial schema” section above (allowable plan-level tweaks).
4. Code includes a **separate** GORM model for `jobs` and mapping to/from **`domain.Job`** with no GORM tags in `internal/domain`.
5. Env var names for DSN (and for migrate, if different) are documented.

## Related

- `specs/000-epic-overview/spec.md`
- `specs/001-agent-skeleton-and-domain/spec.md`
- `.specify/memory/constitution.md`

## Planning artifacts

- `plan.md` — phases, constitution check, resolved decisions
- `research.md` — repo inventory and pattern notes
- `tasks.md` — implementation checklist
- `checklists/requirements.md` — spec quality checklist
- `contracts/environment.md` — env vars for DSN / migrate
- `contracts/jobs-schema.md` — `jobs` columns and domain mapping
