# Feature: Observability and operations

**Feature Branch**: `010-observability`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-03  
**Status**: Draft

**Alignment**: Product narrative and MVP phasing live in [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md). This epic is **ops-facing** only: it must not substitute the **API-first** product surface (`009`) or a separate UI. If anything here implied “build Grafana instead of API/UI,” that contradicts the draft—this epic stays **telemetry and operator visibility**.

## Goal

Structured **logging**, stable **correlation** across process boundaries (HTTP requests, Temporal workflows/activities), and export paths suitable for **GCP** (Cloud Logging; traces/metrics later if we adopt them). Logs should be **queryable** when debugging slot-scoped work: carry **`slot_id`** (and **`user_id`** when present) on pipeline, ingest, and API-handled paths so operators can filter one hunt without guessing.

## Scope

- **Conventions** for `cmd/worker`, future `cmd/api` (or equivalent), and any long-running binaries: field names, levels, and what context to attach before a log line.
- **Correlation IDs**: at minimum Temporal `workflow_id` / `run_id` (and activity context where useful); HTTP **request id** once **`009`** serves traffic; **`slot_id`** / **`user_id`** on domain operations (ingest, pipeline runs, manual workflows per **`008`**).
- **Local / Docker**: readable console logs by default; **JSON** (or another structured encoder) behind an env knob for compose and prod parity.
- **Operator dashboards (draft §7)**: **Grafana** (or similar) on **metrics / health / ad-hoc SQL or log-derived views** is **optional** and **read-only ops**—not the primary product UI. “Temporal Web + log queries” remains the **minimal** story until this epic ships richer tooling.

## Out of scope

- Full SRE playbook, on-call rotations, pager policies.
- **Product** analytics dashboards, CRUD, or workflow control surfaces (those stay **`009`** + UI).
- Choosing a single vendor for **metrics** and **traces** before there is a concrete need (document options when implementing; OpenTelemetry as a possible direction is fine to mention in plan/tasks later).

## Dependencies

- **Product phasing**: per draft §9, richer observability lands in the **extensions** phase **after** a working core vertical; implementation may **start small** alongside **`003`** (worker logs) without blocking core epics.
- **Technical**: most value for HTTP correlation once **`009`** exists; slot-aware fields assume **`002`** / pipeline storage already carry **`slot_id`** (and reserved **`user_id`**) as in **`006`** / **`007`**.

## Local / Docker

- Same binaries as today; pretty console locally, structured logs optional via config (`internal/config` when wired).
- Temporal Web remains a **dev convenience** for workflow inspection; this epic does not require replacing it.

## Related

- [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — §7 UI vs ops; §9 phasing (`010` after core).
- [`../000-epic-overview/spec.md`](../000-epic-overview/spec.md) — epic index and suggested order.
