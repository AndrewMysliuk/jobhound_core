# Implementation Plan: Manual search workflow (on-demand slot orchestration)

**Branch**: `008-manual-search-workflow`  
**Date**: 2026-04-03  
**Last Updated**: 2026-04-03  
**Spec**: `specs/008-manual-search-workflow/spec.md`  
**Input**: Feature specification + [`contracts/manual-workflow.md`](./contracts/manual-workflow.md) + [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Deliver **on-demand** Temporal orchestration for a **search slot** (`slot_id`): optional **stage-1 ingest** as **parallel** child workflows (or equivalent) **per `source_id`**, then **persisted stage 2** and **persisted stage 3** as **two separate** registered Temporal units (workflows or distinctly named activities)—**not** a single bundled step like legacy `RunPersistedPipelineStages`. Add **`slot_jobs`** (`slot_id` + `job_id`, unique pair) so the slot’s candidate set is **`slot_jobs` ∩ `jobs`**; ingest writes membership and respects **`006`** (Redis lock, cooldown, watermarks—manual does **not** bypass). Implement a **parent** “manual slot run” workflow that composes run kinds from the contract (`INGEST_SOURCES`, `PIPELINE_STAGE2`, `PIPELINE_STAGE3`, combinations, full `INGEST_THEN_PIPELINE`, incremental `DELTA_INGEST_THEN_PIPELINE`), creates **`pipeline_runs`** when stage 2 and/or 3 need a new run header (**`007`**), aggregates counts for the **stable response shape** in `contracts/manual-workflow.md`, and exposes **schema DTOs** for **`009`** to stay a thin HTTP layer. **Filter-change invalidation** (drop stage-3-only vs stage-2+3 snapshot data per root spec table) is normative; *where* it runs (on filter save vs first activity) may coordinate with **`009`** but **storage/helpers** belong here.

## Technical Context

**Language/Version**: Go 1.24  
**Orchestration**: Temporal (`003`) — deterministic workflows; I/O only in activities  
**Stages math**: `004`; persistence / caps / `pipeline_run_jobs`: `007` (stage-3 batch **up to 20** jobs per execution ordered by **`jobs.posted_at` DESC for selection**—per **`008`** spec and contract; align const/config with `internal/config` if **`007`** still uses a different default)  
**Ingest**: `005` / `006` — `IngestSourceWorkflow`, `RunIngestSource`, **`PASSED_STAGE_1`**  
**Database**: PostgreSQL + `migrations/` + GORM under feature `storage/` (`002`)  
**Testing**: Colocated unit tests; Temporal via in-memory env and/or `//go:build integration`; no mandatory network for default `go test ./...`  
**Module layout**: Per constitution — `contract.go`, `impl/`, `storage/`, `schema/`, `workflows/` + `activities/` where applicable; no Temporal imports under `internal/domain`

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| V. Temporal for orchestration | **PASS** (parent + split stage units + parallel ingest children) |
| IV. Postgres as system of record | **PASS** (`slot_jobs`, snapshots via `pipeline_run_jobs` / follow-on) |
| VI. Config in `internal/config` | **PASS** (no new env keys *required* solely for `008`; optional stage-3 batch size documented if promoted from const) |
| Testing: default `go test` without Docker | **PASS** (in-memory Temporal + unit tests) |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Contracts | Align [`contracts/manual-workflow.md`](./contracts/manual-workflow.md); add or refresh [`contracts/environment.md`](./contracts/environment.md) if any new `JOBHOUND_*` knobs ship with this epic |
| 1 Data model | Versioned SQL migration: **`slot_jobs`** (+ indexes); GORM models and **jobs** (or sibling) **storage** APIs: link on ingest, slot-scoped list queries joining **`slot_jobs`** + **`jobs`** |
| 2 Split pipeline Temporal units | Replace legacy **bundled** persisted activity with **stage-2-only** and **stage-3-only** units; worker registers both; **no** new features on the old bundled path |
| 3 Parent manual workflow | New workflow: input = `slot_id`, run kind, parameters per contract; parallel ingest children when needed; `CreateRun` when needed; order stage 2 before stage 3; aggregate response fields |
| 4 Invalidation | Implement delete/drop rules for stage-2 vs stage-3 snapshot data when filters change (callable from `009` or workflow entry—behavior frozen in spec table) |
| 5 Schema & client ergonomics | Module-local **`schema/`** DTOs for workflow input/output and API-facing aggregate; optional small Temporal client helper (tests / CLI / future `cmd`—same pattern as `internal/reference/workflows`) |
| 6 Quality gates | `make test`, `make vet`, `make fmt`; integration tests as needed |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **Home for parent workflow** | New thin **`internal/manual/`** (or agreed name) with **`workflows/`**, **`activities/`**, **`schema/`** for run payloads and aggregate response; **reuse** `internal/ingest/workflows` and **`internal/pipeline/workflows`** for child/split units—avoid duplicating ingest/pipeline logic. |
| D2 | **Table name for slot membership** | **`slot_jobs`** (or TBD alias) — unique **`(slot_id, job_id)`**; metadata such as **`first_seen_at`** optional per implementation. |
| D3 | **Legacy bundled activity** | **Deprecate / stop registering** `RunPersistedPipelineStages` (or equivalent) for product paths once split units ship; internal call graph uses only split units. |
| D4 | **Stage-3 batch size** | **Normative for product**: **≤ 20** per execution, **`posted_at` DESC** for selection (**`008`**); keep **one** source of truth in code (`internal/config` const or env per team follow-up from **`007`**). |
| D5 | **Run kind encoding** | Logical kinds from **`contracts/manual-workflow.md` §3**; Go type names are implementation details but must map 1:1 in docs/tests. |

## Engineering follow-ups (non-blocking)

- **Public REST** and slot CRUD — **`009`**.  
- **Scheduled** refresh — product backlog; reuse same workflows/activities.  
- **`cmd/agent` Temporal hook** — optional convenience; not required to close **`008`** if worker + tests + reference client suffice.

## Project structure (documentation)

```text
specs/008-manual-search-workflow/
├── spec.md
├── plan.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── manual-workflow.md
    └── environment.md          # add/refresh when env knobs are frozen
```

## Source structure (anticipated — implementation phase)

```text
migrations/                          # slot_jobs (+ indexes)
internal/jobs/storage/               # or agreed module: slot_jobs writes + slot-scoped queries
internal/ingest/workflows/           # existing IngestSourceWorkflow; ensure slot_jobs + 006 alignment
internal/pipeline/workflows/         # split stage-2 / stage-3 activities (or workflows)
internal/pipeline/storage/           # snapshot writes; invalidation helpers
internal/manual/                     # parent workflow, schema DTOs, registration (if D1 stands)
    schema/
    workflows/
    activities/
cmd/worker/main.go                   # register parent + split units + ingest children
internal/reference/workflows/        # pattern for ExecuteWorkflow in tests/CLI
```
