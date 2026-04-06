# Contract: environment variables (cache & ingest)

**Feature**: `006-cache-and-ingest`  
**Consumers**: `cmd/agent`, `cmd/worker`, ingest activities, `internal/config`.

**Canonical names**: load only via **`internal/config`** — exported env key constants, `LoadIngestFromEnv()`, and **`Config.Ingest`** populated by **`config.Load()`** — no ad-hoc `os.Getenv` in feature packages.

## Redis

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_REDIS_URL` | Yes for ingest coordination in deployments that run ingest | Redis connection URL, e.g. `redis://localhost:6379/0`. |

**Local Compose**: `docker-compose.yml` exposes Redis on **localhost:6379** (service `redis`). Example: `JOBHOUND_REDIS_URL=redis://localhost:6379/0`.

## Explicit refresh (bypass cooldown)

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_INGEST_EXPLICIT_REFRESH` | No | If **`true`**, ingest may **bypass cooldown** (still takes **lock** — see `redis-ingest-coordination.md`). If unset or **`false`**, cooldown is enforced. Default: **disabled**. |

Workflow-level or invocation flags may also enable refresh for manual runs; they must be **explicitly** documented next to the activity and should align with the same semantics (bypass cooldown only, not lock).

## Job retention (hard delete)

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT` | No | If **`true`** (default when unset), `cmd/worker` attempts once at startup to create the Temporal schedule **`jobhound-job-retention`** (weekly **Sunday 05:00 UTC** → `JobRetentionWorkflow`). If **`false`**, no schedule upsert (use Temporal UI/CLI or ops tooling). |

**Manual retention** (same delete semantics as the workflow): run `bin/retention run` after `make build-retention`; requires **`JOBHOUND_DATABASE_URL`**. Deletes `jobs` with `created_at` before now(UTC) minus **7 days**; dependent **`pipeline_run_jobs`** rows follow **`007`** **ON DELETE CASCADE**.

## Lock / cooldown durations (optional overrides)

Defaults match **`redis-ingest-coordination.md`** (lock **600** s, cooldown **3600** s). Optional overrides (positive integers, seconds); invalid or empty values fall back to defaults.

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_INGEST_LOCK_TTL_SEC` | No | Redis `ingest:lock:{slot_id}:{source_id}` TTL. Default **600**. |
| `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` | No | Redis `ingest:cooldown:{slot_id}:{source_id}` TTL after successful ingest. Default **3600**. |

`cmd/worker` passes these into `ingest.NewRedisCoordinatorWithTTL` when Redis is configured.

## Related

- **Database**: `specs/002-postgres-gorm-migrations/contracts/environment.md` — `JOBHOUND_DATABASE_URL`.  
- **Temporal**: `specs/003-temporal-orchestration/contracts/environment.md` — worker wiring for scheduled retention / ingest activities.  
- **Stage-1 column / FKs**: `specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md`.
