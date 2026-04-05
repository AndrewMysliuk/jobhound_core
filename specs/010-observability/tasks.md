# Tasks: Observability (structured logging)

**Input**: [`spec.md`](./spec.md), [`plan.md`](./plan.md), [`contracts/logging.md`](./contracts/logging.md), [`contracts/environment.md`](./contracts/environment.md)  
**Depends on**: Existing tree (`002`–`009`); adds **zerolog** and platform helpers only.  
**Tests**: `go test ./...` without mandatory Docker; logger in tests via `Nop()` or discard.

## Implementation order

| Order | Section | Rationale |
|-------|---------|-----------|
| 1 | [A](#a-contracts--config) | Freeze env names and field contract before wiring. |
| 2 | [B](#b-internalplatformlogging) | All call sites depend on helpers. |
| 3 | [C](#c-cmd-bootstrap) | Root logger + level/format for every binary. |
| 4 | [D](#d-http-middleware-x-request-id) | Context available before handlers run. |
| 5 | [E](#e-debug-http-handlers-cmdagent) | Dev-facing routes get correlation first. |
| 6 | [F](#f-public-api-handlers-cmdapi) | Product API matches same HTTP contract. |
| 7 | [G](#g-domain-impl-slots-profile-pipeline) | Slot/user ids flow into storage calls. |
| 8 | [H](#h-temporal-activities) | I/O and domain errors visible per activity. |
| 9 | [I](#i-temporal-workflows) | Deterministic workflow-level lines. |
| 10 | [J](#j-collectors-multi--misc-cmd) | Remove unstructured `log.Printf` hotspots. |
| 11 | [K](#k-docs--quality-gates) | README, Compose note, CI commands. |

---

## A. Contracts & config

1. [x] **Environment contract** — [`contracts/environment.md`](./contracts/environment.md) matches `internal/config`: `JOBHOUND_LOG_LEVEL`, `JOBHOUND_LOG_FORMAT` parsed in one place; defaults documented.  
2. [x] **Logging field contract** — [`contracts/logging.md`](./contracts/logging.md) matches constants in `internal/platform/logging` (keys: `handler`, `method`, `workflow`, `service`, `request_id`, `workflow_id`, `run_id`, `slot_id`, `user_id`, etc.).  
3. [x] **Root `internal/config` comment** — Epilogue in `config.go` (or adjacent) references **`010`** env keys (same style as `008` one-liner).

## B. `internal/platform/logging`

1. [x] **Dependency** — Add `github.com/rs/zerolog` to `go.mod`; `go mod tidy`.  
2. [x] **Field constants** — Exported `const` or typed strings for JSON keys per [`contracts/logging.md`](./contracts/logging.md).  
3. [x] **Context API** — Unexported ctx keys; exported `WithRequestID`, `WithSlotID`, `WithUserID`, `WithPipelineRunID`, … as needed; `EnrichWithContext(ctx, logger) zerolog.Logger`.  
4. [x] **Activity helper** — Small helper that takes `context.Context` + `*zerolog.Logger` + activity name and merges `workflow_id` / `run_id` from `go.temporal.io/sdk/activity.GetInfo` (handle non-activity ctx safely for tests).  
5. [x] **Root logger factory** — `NewRoot(level, format, binaryName string)` (or equivalent) used by `cmd/*` — **binary** as `service` or dedicated `binary` field for filtering in GCP.

## C. Cmd bootstrap

1. [x] **`cmd/api/main.go`** — Replace `log.Printf` with structured logger; log listen, shutdown, fatal config/db/temporal errors with `.Err()`.  
2. [x] **`cmd/worker/main.go`** — Same; include queue, namespace, schedule registration warnings.  
3. [x] **`cmd/agent/main.go`** — Same for debug HTTP listen/stop.  
4. [x] **`cmd/retention/main.go`** — Structured log for deleted count and errors; align level with config if retention loads full `Config` or minimal env.  
5. [x] **`cmd/migrate/main.go`** — Keep `fmt` for CLI human output; documented in package comment (spec C.5).

## D. HTTP middleware (`X-Request-ID`)

1. [x] **`internal/collectors/handlers/debughttp`** — Wrap mux or per-route chain: ensure every request gets id in context; optional response header `X-Request-ID`.  
2. [x] **`internal/publicapi/handlers`** — Same middleware applied in `NewHTTPHandler` (inside CORS chain order documented: request id first vs CORS — pick consistent order).

## E. Debug HTTP handlers (`cmd/agent`)

1. [x] Instrument **each** handler with `logH := logging.EnrichWithContext(r.Context(), logger.With().Str(FieldHandler, "<Name>").Logger())`; **Error** on failures; **Debug** optional start/payload summary (no secrets).

| File | Handlers / notes |
|------|------------------|
| [`handler.go`](../../internal/collectors/handlers/debughttp/handler.go) | Router registration + middleware wiring |
| [`health.go`](../../internal/collectors/handlers/debughttp/health.go) | Health |
| [`europe_remotely.go`](../../internal/collectors/handlers/debughttp/europe_remotely.go) | Collector route |
| [`working_nomads.go`](../../internal/collectors/handlers/debughttp/working_nomads.go) | Collector route |
| [`run_collector.go`](../../internal/collectors/handlers/debughttp/run_collector.go) | Generic collector run |
| [`helpers.go`](../../internal/collectors/handlers/debughttp/helpers.go) | Add logger param if shared decode paths need **Error** lines |

[`response.go`](../../internal/collectors/handlers/debughttp/response.go): only touch if error paths need logging (prefer logging at call site).

## F. Public API handlers (`cmd/api`)

1. [x] Extend [`handler.go`](../../internal/publicapi/handlers/handler.go) **`Deps`** with `zerolog.Logger`; pass into handlers via `routeLog` / `ReadJSON`. **Every** route:

| File | Route / handler |
|------|-----------------|
| [`health.go`](../../internal/publicapi/handlers/health.go) | `GET /api/v1/health` |
| [`get_profile.go`](../../internal/publicapi/handlers/get_profile.go) | `GET /api/v1/profile` |
| [`put_profile.go`](../../internal/publicapi/handlers/put_profile.go) | `PUT /api/v1/profile` |
| [`get_slots.go`](../../internal/publicapi/handlers/get_slots.go) | `GET /api/v1/slots` |
| [`post_slots.go`](../../internal/publicapi/handlers/post_slots.go) | `POST /api/v1/slots` |
| [`get_slot.go`](../../internal/publicapi/handlers/get_slot.go) | `GET /api/v1/slots/{slot_id}` |
| [`delete_slot.go`](../../internal/publicapi/handlers/delete_slot.go) | `DELETE ...` |
| [`post_stage2_run.go`](../../internal/publicapi/handlers/post_stage2_run.go) | Stage 2 run |
| [`post_stage3_run.go`](../../internal/publicapi/handlers/post_stage3_run.go) | Stage 3 run |
| [`get_stage_jobs.go`](../../internal/publicapi/handlers/get_stage_jobs.go) | Stage jobs list |
| [`patch_job_bucket.go`](../../internal/publicapi/handlers/patch_job_bucket.go) | Patch bucket |

After parsing **`slot_id`** from path, call `logging.WithSlotID` on ctx before **`deps.Slots` / `deps.Profile`** calls. **`user_id`** when API exposes it.

[`cors.go`](../../internal/publicapi/handlers/cors.go): no business logs unless error path.

## G. Domain `impl` (`slots`, `profile`, `pipeline`)

1. [x] **`internal/slots/impl`** — Constructor: `logger.With().Str(FieldService, "slots").Logger()`; each exported method: `EnrichWithContext` + `FieldMethod`; **Error** on returned errors at service boundary where not already logged below (avoid duplicate — **either** handler **or** service for same failure; prefer **handler** for HTTP + **service** for workflow-only entrypoints).  
2. [x] **`internal/profile/impl`** — Same pattern (`service` = `profile`).  
3. [x] **`internal/pipeline/impl`** — Same (`service` = `pipeline`); include **`pipeline_run_id`** in ctx/logs when present.

**Rule of thumb**: HTTP handler logs request validation errors; **service** logs when invoked from **Temporal activities** without HTTP, or logs **Debug** business milestones. Adjust in implementation so no double-**Error** on same failure.

## H. Temporal — activities

1. [x] At **start** of each activity: build logger with activity helper + **`FieldHandler`** = registered activity name; attach **`slot_id`** / **`pipeline_run_id`** from inputs where applicable (`user_id` when present on DTOs).

| Package | Methods |
|---------|---------|
| [`internal/ingest/workflows/activities`](../../internal/ingest/workflows/activities/ingest.go) | `RunIngestSource` |
| [`internal/pipeline/workflows/activities`](../../internal/pipeline/workflows/activities/stages.go) | `RunPipelineStages`, `RunPersistPipelineStage2`, `RunPersistPipelineStage3` |
| [`internal/manual/workflows/activities`](../../internal/manual/workflows/activities/slot_run.go) | Slot run activity(ies) |
| [`internal/jobs/workflows/activities`](../../internal/jobs/workflows/activities/retention.go) | `RunJobRetention` |

**Error** on downstream failures; **Debug** for high-signal milestones (e.g. ingest source id, job counts) without large payloads.

## I. Temporal — workflows

1. [x] **`workflow.GetLogger`**: **`FieldWorkflow`** + workflow name; deterministic fields from input; activity / ingest-child failures → **Error**.

| File | Workflow |
|------|----------|
| [`internal/ingest/workflows/ingest_workflow.go`](../../internal/ingest/workflows/ingest_workflow.go) | `IngestSourceWorkflow` |
| [`internal/manual/workflows/manual_slot_run.go`](../../internal/manual/workflows/manual_slot_run.go) | Manual slot run parent |
| [`internal/jobs/workflows/retention_workflow.go`](../../internal/jobs/workflows/retention_workflow.go) | Retention workflow |

(`internal/pipeline/workflows` today registers **activities only** — no separate workflow func; parent orchestration lives under **`manual`** / callers.)

## J. Collectors multi & misc `cmd`

1. [x] [`internal/collectors/multi/multi.go`](../../internal/collectors/multi/multi.go) — Replace `log.Printf` with injected or package logger; **Warn**/**Error** per wrapped collector failure with **`source_id`** if available.  
2. [x] [`cmd/agent/temporal_manual.go`](../../cmd/agent/temporal_manual.go) — Structured log for workflow start/result/errors (stdout JSON still ok for CLI aggregate; logs go to stderr/logger per binary convention).  
3. [x] **Worker registration** — [`cmd/worker/main.go`](../../cmd/worker/main.go): pass logger into activity structs constructors if they need it (`NewActivities(logger, ...)` pattern).

## K. Docs & quality gates

1. [x] **README** — Document `JOBHOUND_LOG_LEVEL`, `JOBHOUND_LOG_FORMAT`; example `json` for Compose/GCP.  
2. [x] **`make test` / `go test ./...`** — Pass.  
3. [x] **`make vet` / `make fmt`** — Clean.  
4. [x] **Spot check** — `grep -R "log\\.Printf\|log\\.Println" --include='*.go' cmd internal` — only justified leftovers (e.g. migrate CLI) documented.
