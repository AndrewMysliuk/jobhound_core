# Contract: environment variables (cache & ingest)

**Feature**: `006-cache-and-ingest`  
**Consumers**: `cmd/agent`, `cmd/worker`, ingest activities, `internal/config`.

**Canonical names**: load only via **`internal/config`** — no ad-hoc `os.Getenv` in feature packages.

## Redis

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_REDIS_URL` | Yes for ingest coordination in deployments that run ingest | Redis connection URL, e.g. `redis://localhost:6379/0`. |

Local Compose must document a default URL aligned with the Redis service when this feature is implemented.

## Explicit refresh (bypass cooldown)

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_INGEST_EXPLICIT_REFRESH` | No | If **`true`**, ingest may **bypass cooldown** (still takes **lock** — see `redis-ingest-coordination.md`). If unset or **`false`**, cooldown is enforced. Default: **disabled**. |

Workflow-level or invocation flags may also enable refresh for manual runs; they must be **explicitly** documented next to the activity and should align with the same semantics (bypass cooldown only, not lock).

## Lock / cooldown durations

**Not** environment variables in v1 — see **`redis-ingest-coordination.md`** (code constants: lock **600** s, cooldown **3600** s defaults).

## Related

- **Database**: `specs/002-postgres-gorm-migrations/contracts/environment.md` — `JOBHOUND_DATABASE_URL`.  
- **Temporal**: `specs/003-temporal-orchestration/contracts/environment.md` — worker wiring for scheduled retention / ingest activities.  
- **Stage-1 column / FKs**: `specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md`.
