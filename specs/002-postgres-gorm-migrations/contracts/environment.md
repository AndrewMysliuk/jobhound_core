# Contract: environment variables (database & migrate)

**Feature**: `002-postgres-gorm-migrations`  
**Consumers**: `cmd/agent`, `cmd/migrate` (if present), local Compose

## Application DSN

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_DATABASE_URL` | Yes (for any DB-using command) | PostgreSQL connection URL, e.g. `postgres://USER:PASSWORD@localhost:5432/DBNAME?sslmode=disable` |

## Migrate override (optional)

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_MIGRATE_DATABASE_URL` | No | If set, migrate entrypoint uses this instead of `JOBHOUND_DATABASE_URL` (useful when migrate runs in a different network context). |

## Compose alignment (local)

Docker Compose should set `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` so that a documented example `JOBHOUND_DATABASE_URL` matches the default service. Exact values are defined in `docker-compose.yml` and repeated in README.

## Secrets

Never commit real URLs or passwords. Document **names only** in Makefile / README per constitution.
