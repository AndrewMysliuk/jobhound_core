# Contract: structured logging (field names & layering)

**Feature**: `010-observability`  
**Consumers**: All binaries and feature packages that emit application logs.

## Principles

- **Patterns** match omg-api: `EnrichWithContext(ctx, logger)` at boundaries; **handler** for HTTP and Temporal **activities**; **method** (+ **service** on the service struct) for `impl`; **workflow** for Temporal workflow methods.
- **No** `github.com/omgbank/go-common` — helpers live in **`internal/platform/logging`** with the **same JSON key names** as below where applicable.
- **Storage**: return **wrapped errors**; **`Warn`** only when returning partial success after skipping bad rows (document in code if used).

## Structured field keys (JSON / zerolog)

Use these **string values** consistently (constants in `internal/platform/logging`):

| Key | When |
|-----|------|
| `handler` | HTTP route handler name; Temporal **activity** name |
| `method` | `impl` service method name |
| `workflow` | Temporal **workflow** name |
| `service` | Logical service name on the service logger (constructor) |
| `request_id` | HTTP request correlation (from `X-Request-ID` or generated) |
| `workflow_id` | From Temporal activity info |
| `run_id` | Temporal run id |
| `slot_id` | UUID string when the operation is slot-scoped |
| `user_id` | When present (MVP may be empty / single-tenant) |
| `pipeline_run_id` | When a pipeline run header is in play |
| `source_id` | Collector / ingest source when relevant |

Add narrow keys for debugging only when stable (e.g. `stage`, `job_id`) — avoid dumping payloads.

## Context contract

- Package **`internal/platform/logging`** exposes `With*` functions that store values on `context.Context` (unexported keys).
- **First** code path that knows an id must attach it; downstream passes the same `ctx` into `impl` / activities.
- HTTP: middleware wraps the request with **`request_id`** before `ServeHTTP` reaches handlers; handlers add **`slot_id`** / **`user_id`** when parsed from path/body.
- Activities: at entry, merge **activity.GetInfo** ids and payload ids into logger via `EnrichWithContext` (and optionally attach to a derived ctx for nested calls).

## HTTP: `X-Request-ID`

- If the client sends **`X-Request-ID`**, use it (trim, reasonable max length).
- Else generate a new id (e.g. UUID).
- Attach to `r.Context()` for the rest of the request; echo header on response optional but recommended for tracing.

## Workflow code

- Use **`workflow` logger fields** from deterministic inputs (e.g. workflow name, `slot_id` from args).
- **Heavy / I/O detail** — log from **activities** only (Temporal determinism).

## Security

- Do **not** log secrets: API keys, cookies, session tokens, full LLM prompts/responses, raw HTML bodies.
- URLs and source names are fine; cap or hash if a source ever embeds credentials in query (should not).
