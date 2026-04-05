# Feature: Observability (structured logging)

**Feature Branch**: `010-observability`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-05  
**Status**: Draft

**Alignment**: Product narrative in [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md). This epic is **ops-facing**: **structured logs and correlation** only. It does **not** replace the product API (`009`) or any UI.

## Goal

**Structured logging** (queryable fields, stable keys), **correlation across boundaries** (HTTP, Temporal workflows and activities), and output suitable for **GCP Cloud Logging** (typically **JSON on stdout** in compose/prod). Operators must be able to filter **one hunt** without guesswork: logs on ingest, pipeline, and HTTP-handled paths carry **`slot_id`** and **`user_id`** when known.

**Reference style**: Same **patterns** as omg-api (`EnrichWithContext`, `handler` / `method` / `workflow` field discipline, error logging at boundaries). **Do not** add `github.com/omgbank/go-common` or other Omega-internal deps — reimplement the small helpers inside this repo (see Implementation layout).

## Scope

### Logging package

- Add **`internal/platform/logging`**: zerolog-based helpers, context enrichment, and string constants for structured field names aligned with the omg-api **naming** (e.g. `handler`, `method`, `workflow`, `service` where applicable).
- **`EnrichWithContext(ctx, logger)`** reads correlation and domain ids previously stored on `context.Context` and returns a child logger for the call site.

### Where to log (omg-api-style)

- **HTTP** (debug routes on `cmd/agent` **now**; public **`cmd/api`** when it exists): at the start of each handler, build a logger with **`FieldHandler`** (operation name) + `EnrichWithContext`. Log **errors** on validation and downstream failures; **Debug** for optional start/request detail where useful.
- **`impl` (services)**: **`FieldMethod`** (and **`FieldService`** on the service logger at construction, same idea as omg-api). `EnrichWithContext` at method entry when the request carries context.
- **Temporal activities**: **`FieldHandler`** + activity name; enrich with **`workflow_id`** / **`run_id`** from activity info (and domain ids from payload) at the start of the activity.
- **Temporal workflows**: **`FieldWorkflow`** + workflow name; structured workflow-level logging consistent with omg-api workflow guidelines (see project Temporal rules).
- **Storage**: prefer **return wrapped errors** without logging every DB failure; **`Warn`** only when intentionally **skipping** bad data but returning a partial/successful result (same exception pattern as omg-api storage in edge cases).

### Context contract

Whoever **first** knows an id attaches it to `context.Context` via small APIs in **`internal/platform/logging`** (e.g. `WithRequestID`, `WithSlotID`, `WithUserID` — exact names in implementation). Downstream code passes **that** ctx into services and storage so `EnrichWithContext` sees the same fields everywhere.

Minimum correlation set:

- HTTP: **`request_id`** — use header **`X-Request-ID`** when present; otherwise generate and attach to `r.Context()` for **all** debug HTTP handlers (and the same rule for **`009`** when implemented).
- Temporal activities: **`workflow_id`**, **`run_id`**, plus **`slot_id`** / **`user_id`** when the activity input carries them.
- Workflows: log identity fields that are **deterministic** from workflow input / known state; heavy or non-deterministic detail stays in activities.

### Format and config

- **Local default**: human-readable console output.
- **Compose / prod-style**: **JSON** logs via **`internal/config`** env knob (exact `JOBHOUND_*` names documented in this epic’s `contracts/environment.md` when added, and in `internal/config`).
- Log **level** from config; no scattered `os.Getenv` outside `internal/config`.

### GCP

- Rely on **structured JSON to stdout** for ingestion into **Cloud Logging**; no mandatory extra agents in this epic.

## Out of scope

- Metrics backends, trace vendors, OpenTelemetry rollout — **until** a later epic or need; optional note in `plan.md` is enough.
- Full SRE playbook, on-call, paging.
- Product analytics, CRUD, or workflow **control** UI (those stay **`009`** + separate UI).
- **Dashboards** and external ops-visualization stacks — not part of this epic; **Temporal Web** + **log queries** are enough.

## Dependencies

- **Phasing**: per draft §9, full rollout can follow the core vertical; **basic** worker/handler logging can start alongside **`003`** without blocking other epics.
- **Data**: **`slot_id`** / reserved **`user_id`** on domain paths assume **`002`**, pipeline/ingest specs (**`006`** / **`007`**, etc.) as today.
- **HTTP**: public API **`009`** extends the same **`X-Request-ID`** / context pattern; debug HTTP in **`cmd/agent`** adopts it **immediately** per this spec.

## Local / Docker

- Same binaries (`cmd/agent`, `cmd/worker`, future `cmd/api`); format and level from config.
- Temporal Web remains the workflow inspection tool for local/dev.

## Related

- [`plan.md`](./plan.md), [`tasks.md`](./tasks.md), [`checklists/requirements.md`](./checklists/requirements.md) — implementation breakdown.
- [`contracts/logging.md`](./contracts/logging.md), [`contracts/environment.md`](./contracts/environment.md) — field names and `JOBHOUND_LOG_*`.
- [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — §9 phasing (`010` in extensions; can overlap early).
- [`../000-epic-overview/spec.md`](../000-epic-overview/spec.md) — epic index.
- [`../002-postgres-gorm-migrations/spec.md`](../002-postgres-gorm-migrations/spec.md) — no `go-common`; patterns only.
