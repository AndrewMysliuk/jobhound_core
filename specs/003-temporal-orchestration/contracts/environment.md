# Contract: environment variables (Temporal)

**Feature**: `003-temporal-orchestration`  
**Consumers**: `cmd/worker`, Temporal **client** code (tests, optional dev entrypoint)

**Canonical names**: any code that dials Temporal or runs the worker must use the variable names below exactly as written (Compose docs and README stay aligned with this file). Go code should read them via **`internal/config`** (`EnvTemporalAddress`, `LoadTemporalFromEnv`, defaults `DefaultTemporalNamespace` / `DefaultTemporalTaskQueue`) — not ad-hoc `os.Getenv` in feature packages.

## Connection

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_TEMPORAL_ADDRESS` | Yes (for worker and non-test client) | gRPC **frontend** address, host:port as seen from the process (e.g. `localhost:7233` when Temporal from Compose is mapped to the host). |

## Namespace and task queue

Defaults apply when variables are unset; **code must use these defaults** so worker and clients stay aligned with the spec.

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_TEMPORAL_NAMESPACE` | No | Temporal namespace. **Default**: `default`. |
| `JOBHOUND_TEMPORAL_TASK_QUEUE` | No | Task queue for worker polling and workflow starts. **Default**: `jobhound`. |

## TLS / cloud (future)

Local Compose typically uses **no mTLS**. If production or cloud adds TLS or API keys, extend this contract in the spec that introduces prod worker deployment — **do not** commit secrets; document variable **names** only.

## Compose alignment (local)

`docker-compose.yml` maps:

| Host | Service | Purpose |
|------|---------|---------|
| `localhost:7233` | `temporal` | gRPC frontend — use for `JOBHOUND_TEMPORAL_ADDRESS` |
| `http://localhost:8088` | `temporal-ui` | Temporal Web UI (container listens on 8080, published as 8088) |

Example for processes on the host:

- `JOBHOUND_TEMPORAL_ADDRESS=localhost:7233`

Application Postgres remains on `localhost:5432` (`postgres` service); Temporal uses a **separate** database service (`temporal-postgresql`) on the Docker network only.

## Relationship to database env

The **reference v0** workflow does **not** require `JOBHOUND_DATABASE_URL`. If a future worker binary loads DB and Temporal, both contracts apply: see `specs/002-postgres-gorm-migrations/contracts/environment.md`. Product workflows (`006`–`007`, `009`, `011`) will use DB-backed activities while preserving **idempotent** side effects under retries—see [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) §4 and epic **`007`**.
