# jobhound_core

Backend for a **personal job agent**: collect vacancies, narrow them in **three stages** (role/time → keywords → CV-aware LLM), persist in **PostgreSQL**, run **manual** and **scheduled** flows via **Temporal**, and optionally notify via **Telegram** (short messages). A **web UI** is planned in a separate repository; this repo will expose an HTTP API. Auth is out of scope for now; the data model stays open for a future user id.

## Stack (target)

Go 1.24, PostgreSQL + GORM, Temporal, Claude API for scoring, Telegram Bot API for delivery. Local development is meant to use **Docker Compose** for Postgres and Temporal (see epic `002` / `003` in `specs/`).

## Repo layout

- `cmd/agent` — agent binary (wiring)
- `cmd/worker` — Temporal worker (reference workflow + activities)
- `cmd/migrate` — SQL migrations CLI
- `internal/domain` — shared domain types (e.g. Job)
- `internal/platform/pgsql` — GORM Postgres open + pool from config; integration-tagged migration tests (infrastructure, not a domain module)
- `internal/jobs` — jobs module: contract, GORM storage
- `internal/pipeline` — collect / filter / score pipeline: contract, `impl/`, `mock/`
- `internal/config` — configuration shape (secrets from `.env` only)
- `tests/integration/` — optional cross-module integration suites (`integration` build tag)
- `migrations/` — SQL migration files
- `internal/reference/workflows` — v0 reference Temporal workflow + `activities/` (orchestration; no DB on demo path); future features add their own `internal/<feature>/workflows/`
- `specs/` — feature specs (`000` overview, `001`–`012` index in `specs/000-epic-overview/spec.md`)

Conventions: `.cursor/rules/specify-rules.mdc` and `.specify/memory/constitution.md`.

## Quick start

```bash
# .env in repo root is gitignored — edit it with your keys/paths
make build
make run
make test
```

`make help` lists targets and documented **environment variable names** for PostgreSQL and Temporal (values stay in `.env`, never committed).

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

## Temporal (local Compose)

Contracts and naming: **[`specs/003-temporal-orchestration/contracts/environment.md`](specs/003-temporal-orchestration/contracts/environment.md)** (connection env vars) and **[`specs/003-temporal-orchestration/contracts/reference-workflow.md`](specs/003-temporal-orchestration/contracts/reference-workflow.md)** (reference workflow/activity, queue, namespace).

From the repo root, `docker compose up -d` starts **application Postgres** (port **5432**), **Temporal** (gRPC on host **7233**), and **Temporal Web UI** at **http://localhost:8088** (UI container listens on 8080; Compose maps it to 8088). Temporal persistence uses a **separate** Postgres service (`temporal-postgresql`) on the Compose network; it does not replace the `jobhound` database.

Workers and clients on your machine should point at the Temporal frontend:

```bash
export JOBHOUND_TEMPORAL_ADDRESS='localhost:7233'
# Optional overrides (defaults match spec 003):
# export JOBHOUND_TEMPORAL_NAMESPACE='default'
# export JOBHOUND_TEMPORAL_TASK_QUEUE='jobhound'
```

Build and run the worker (after `docker compose up -d` for Temporal):

```bash
make build          # produces bin/agent and bin/worker
export JOBHOUND_TEMPORAL_ADDRESS='localhost:7233'
./bin/worker        # or: make run-worker
```

### Manual check: Temporal Web UI

With **Temporal** and **`bin/worker`** running (same `JOBHOUND_TEMPORAL_*` defaults as above):

1. Open **http://localhost:8088** (Temporal Web UI).
2. Run the integration test (starts one workflow run on queue `jobhound`, namespace `default`):

   ```bash
   export JOBHOUND_TEMPORAL_ADDRESS='localhost:7233'
   go test -tags=integration ./internal/reference/workflows/ -run TestReferenceDemoWorkflow_againstServer
   ```

3. In the UI, open **Workflows** and find the run whose id starts with `integration-reference-`.
4. Open that execution and confirm **at least one completed activity** (`ReferenceGreetActivity`) appears in the history.

Default `go test ./...` also runs **in-memory** Temporal tests under `internal/reference/workflows` (no Docker).

### Worker in production (GCP sketch)

For a later GCP deployment, the worker is a **long-running process** (e.g. Cloud Run **with min instances**, GKE, or a VM) that uses the same binary as local dev (`bin/worker`). Point it at your Temporal frontend with **`JOBHOUND_TEMPORAL_ADDRESS`** (and, if non-default, **`JOBHOUND_TEMPORAL_NAMESPACE`** / **`JOBHOUND_TEMPORAL_TASK_QUEUE`**) supplied by your environment or secret manager — never commit real endpoints or tokens. Network topology (VPC, mTLS, separate namespaces per env) is out of scope for spec `003`; see `.specify/memory/constitution.md` and `specs/000-epic-overview` as the architecture evolves.

## Documentation

- [Epic overview and feature index](specs/000-epic-overview/spec.md)
- Constitution and principles: `.specify/memory/constitution.md`
