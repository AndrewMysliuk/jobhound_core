# Contract: environment variables (HTTP public API)

**Feature**: `009-http-public-api`  
**Consumers**: `cmd/api`, Docker Compose overrides, deployment config.  
**Single source of parsing**: [`internal/config`](../../../internal/config) — handlers and feature code must not read raw `os.Getenv` for these keys.

## `009`-scoped variables

| Variable | Required | Default (local dev) | Semantics |
|----------|----------|---------------------|-----------|
| **`JOBHOUND_API_LISTEN`** | No | **`127.0.0.1:8081`** (or team-agreed port; document in `cmd/api` help) | TCP address for the public API HTTP server. **Alternative**: split into **`JOBHOUND_API_HOST`** + **`JOBHOUND_API_PORT`** if the codebase standard prefers two keys — **pick one pair** in `internal/config` and list only that here. |
| **`JOBHOUND_API_CORS_ORIGINS`** | No | **`http://localhost:8080`** | Comma-separated list of allowed **Origin** values for CORS. Empty → **no** browser CORS (or deny all origins) — document chosen behavior for production. |

## Shared variables (already defined elsewhere)

| Variable | Epic | Notes |
|----------|------|--------|
| **`JOBHOUND_DATABASE_URL`** | `002` | Postgres DSN for API process. |
| **`JOBHOUND_TEMPORAL_ADDRESS`** | `003` | Temporal frontend for workflow client. |
| Namespace / task queue | `003` | Use same values as worker so **`ManualSlotRunWorkflow`** starts on the correct queue (see [`../../003-temporal-orchestration/contracts/environment.md`](../../003-temporal-orchestration/contracts/environment.md)). |

## Implementation checklist

- [ ] All names above parsed in **`internal/config`** and passed into **`cmd/api`** as structs.  
- [ ] Defaults documented in this file match **`Load*`** behavior in code.  
- [ ] No new secret knobs in repo; secrets stay out of git per project rules.
