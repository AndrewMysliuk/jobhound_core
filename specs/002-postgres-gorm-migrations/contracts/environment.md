# Contract: environment variables (database & migrate)

**Feature**: `002-postgres-gorm-migrations`  
**Consumers**: `cmd/agent`, `cmd/migrate` (if present), local Compose

**Canonical names**: any code that opens PostgreSQL or runs migrations must use the variable names below exactly as written (Compose docs and README stay aligned with this file). Go code should read them via **`internal/config`** (`EnvDatabaseURL`, `LoadDatabaseFromEnv`, `MigrateDSNFromEnv`, pool env constants) — not ad-hoc `os.Getenv` in `internal/platform/pgsql` or other feature packages.

## Application DSN

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_DATABASE_URL` | Yes (for any DB-using command) | PostgreSQL connection URL, e.g. `postgres://USER:PASSWORD@localhost:5432/DBNAME?sslmode=disable` |

## Connection pool (optional)

If unset, **`internal/config`** applies defaults consumed by `internal/platform/pgsql`: max open **25**, max idle **5**, conn max lifetime **1h**. Invalid numeric values fall back to defaults.

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_DB_MAX_OPEN_CONNS` | No | `database/sql` `SetMaxOpenConns` (integer ≥ 1 recommended). |
| `JOBHOUND_DB_MAX_IDLE_CONNS` | No | `SetMaxIdleConns` (integer ≥ 0). |
| `JOBHOUND_DB_CONN_MAX_LIFETIME_SEC` | No | `SetConnMaxLifetime` in seconds (integer ≥ 1). |

## Migrate override (optional)

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_MIGRATE_DATABASE_URL` | No | If set, migrate entrypoint uses this instead of `JOBHOUND_DATABASE_URL` (useful when migrate runs in a different network context). |

## Compose alignment (local)

Docker Compose should set `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` so that a documented example `JOBHOUND_DATABASE_URL` matches the default service. Exact values are defined in `docker-compose.yml` and repeated in README.

## Secrets

Never commit real URLs or passwords. Document **names only** in this contract and README per constitution.
