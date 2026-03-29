# jobhound_core

Backend for a **personal job agent**: collect vacancies, narrow them in **three stages** (role/time → keywords → CV-aware LLM), persist in **PostgreSQL**, run **manual** and **scheduled** flows via **Temporal**, and optionally notify via **Telegram** (short messages). A **web UI** is planned in a separate repository; this repo will expose an HTTP API. Auth is out of scope for now; the data model stays open for a future user id.

## Stack (target)

Go 1.24, PostgreSQL + GORM, Temporal, Claude API for scoring, Telegram Bot API for delivery. Local development is meant to use **Docker Compose** for Postgres and Temporal (see epic `002` / `003` in `specs/`).

## Repo layout

- `cmd/agent` — agent binary (wiring)
- `cmd/migrate` — SQL migrations CLI
- `internal/domain` — shared domain types (e.g. Job)
- `internal/db` — Postgres open, pool; integration-tagged migration tests
- `internal/jobs` — jobs module: contract, GORM storage
- `internal/pipeline` — collect / filter / score pipeline: contract, `impl/`, `mock/`
- `internal/config` — configuration shape (secrets from `.env` only)
- `tests/integration/` — optional cross-module integration suites (`integration` build tag)
- `migrations/` — SQL migration files
- `specs/` — feature specs (`000` overview, `001`–`012` index in `specs/000-epic-overview/spec.md`)

Conventions: `.cursor/rules/specify-rules.mdc` and `.specify/memory/constitution.md`.

## Quick start

```bash
# .env in repo root is gitignored — edit it with your keys/paths
make build
make run
make test
```

`make help` lists targets and **database-related environment variable names** (values stay in `.env`, never committed).

## Database environment (contract)

PostgreSQL DSN and optional migrate override are defined in **[`specs/002-postgres-gorm-migrations/contracts/environment.md`](specs/002-postgres-gorm-migrations/contracts/environment.md)** (`JOBHOUND_DATABASE_URL`, `JOBHOUND_MIGRATE_DATABASE_URL`). Implementations must match those names; `make help` echoes the same identifiers for quick reference.

### Connecting from the host (default Compose)

From the repo root, start Postgres with `docker compose up -d` (see [`docker-compose.yml`](docker-compose.yml): user, password, and database are all `jobhound`, port `5432` on the host). Applications and migrate tools on your machine should use:

```bash
export JOBHOUND_DATABASE_URL='postgres://jobhound:jobhound@localhost:5432/jobhound?sslmode=disable'
```

You can put the same value in a local `.env` file (gitignored). Credentials match `POSTGRES_*` in Compose only for local dev; do not reuse this pattern in production.

### Migrations and idempotence

Use `make migrate-up` (or `bin/migrate up` after `make build-migrate`) with `JOBHOUND_DATABASE_URL` set. **Applying `up` when the database is already at the latest migration version exits successfully** (exit code 0): `cmd/migrate` treats golang-migrate’s “no change” result as success, so deploy scripts and local workflows can run `migrate up` repeatedly without failing on the second run.

## Documentation

- [Epic overview and feature index](specs/000-epic-overview/spec.md)
- Constitution and principles: `.specify/memory/constitution.md`
