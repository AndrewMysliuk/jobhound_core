# Contract: manual search workflow (on-demand orchestration)

**Feature**: `008-manual-search-workflow`  
**Status**: **Draft** — aligns with root [`spec.md`](../spec.md) (snapshots, **`slot_jobs`**, **separate** stage-2 / stage-3 Temporal units, filter invalidation). Code may still use legacy **`RunPersistedPipelineStages`** until split.

## 1. Purpose

Define the **on-demand** path for a **search slot** (`slot_id` UUID): optional **stage-1 ingest** (parallel per `source_id`), then **persisted stage 2** and **persisted stage 3** as **separate** Temporal workflows or activities, plus response shapes for **`009`**. Triggers are explicit (API, CLI, tests)—**not** cron. **Public HTTP MVP** behavior is normative in **`009`** (stage 1 only on slot create; no re-ingest on existing slot; stages 2 and 3 are **separate** API calls).

DTOs live under module **`schema/`** (e.g. `internal/manual/schema` or split across `ingest/schema`, `pipeline/schema`).

## 2. Non-goals

- **Public REST** for starting runs → **`009`**.
- **Slot CRUD** and auth → **`009`** / **`002`** as applicable.
- **Scheduled** refresh → product backlog (reuse the same primitives when added).

## 3. Run kinds (product semantics)

Logical **run kind** selects which steps execute. Go names are implementation details.

| Run kind (logical) | Stage-1 ingest | Stage 2 (snapshot) | Stage 3 (snapshot) | Notes |
|--------------------|----------------|--------------------|---------------------|--------|
| `INGEST_SOURCES` | Yes, **parallel** per `source_id` | No | No | **`006`** lock / cooldown / watermarks; **`IngestSourceInput.SlotID`** required; writes **`slot_jobs`** + **`PASSED_STAGE_1`** where applicable. Used when **`009`** creates a slot (HTTP does **not** expose re-ingest on the same `slot_id`). |
| `PIPELINE_STAGE2` | No | Yes | No | Pool = **`slot_jobs`** ∩ **`jobs`** with **`stage1_status = PASSED_STAGE_1`**. Produces stage-2 pass/fail snapshot. **`009`**: maps to **`POST …/stages/2/run`**; wipes stages **2+3** first; does **not** start stage 3. |
| `PIPELINE_STAGE3` | No | No | Yes | Pool = **`PASSED_STAGE_2`** for the active **`pipeline_run_id`**. LLM batch: up to **`max_jobs`** (HTTP: **1–100** from **`009`**; **`007`** may cap further—use **min** of policy and request). Order: **`posted_at`** DESC, **`job_id`** ASC. |
| `PIPELINE_STAGE2_THEN_STAGE3` | No | Yes | Yes | After stage 2 completes, run stage 3 on the new stage-2 snapshot. For **CLI/tests** or future product; **not** the default **`009`** flow (client calls stage 2 and 3 separately). |
| `INGEST_THEN_PIPELINE` | Yes (parallel per source) | Yes | Yes | Parent “full run”: ingest → stage 2 → stage 3. **Not** the **`009`** browser MVP (no single HTTP action for 1→2→3). |
| `DELTA_INGEST_THEN_PIPELINE` | Yes (**incremental** per **`006`**) | Yes | Yes | “Pull new” + pipeline. **Not** exposed in public HTTP MVP **`009`** (no repeat ingest on existing slot until product adds it). |

**Invalidation** (when user changes slot filters)—see root **`spec.md`** and **[`filter-invalidation.md`](./filter-invalidation.md)**:

- **Stage-3 rules only** → delete **stage-3** snapshot data only; then use **`PIPELINE_STAGE3`** (HTTP: **`POST …/stages/3/run`** with **`max_jobs`**).
- **Stage-2 rules** → delete **stage-2 and stage-3** snapshots; **`009`** uses **`PIPELINE_STAGE2`** only for the next run (client triggers **`PIPELINE_STAGE3`** separately). Composite kinds may still run **2 then 3** in one workflow for non-HTTP callers.

**Manual** never bypasses **`006`** for ingest.

## 4. Temporal mapping (target architecture)

### 4.1 Existing primitives (reuse semantics; split implementation)

| Piece | Registered name / symbol (today) | Package | **`008` expectation** |
|-------|-----------------------------------|---------|------------------------|
| Ingest workflow | `IngestSourceWorkflow` | `internal/ingest/workflows` | Keep; parent runs **N children in parallel** for N sources. |
| Ingest activity | `RunIngestSourceActivity` | `internal/ingest/workflows/activities` | Unchanged contract for per-source ingest. |
| Persisted pipeline (legacy) | `RunPersistedPipelineStagesActivity` | `internal/pipeline/workflows/activities` | **Split** into **two** activities (or workflows): one for **stage-2 snapshot only**, one for **stage-3 snapshot only**. Do not add new features to the bundled form. |

**Timeouts / retries**: use **`internal/platform/temporalopts`** (or equivalent); ingest workflow keeps conservative ingest timeouts.

### 4.2 Parent “manual slot run” workflow (to implement)

1. Input: **`slot_id`**, optional **`user_id`**, **run kind**, parameters (sources, `ExplicitRefresh`, **`004`** rules **`include`/`exclude`** for stage 2, profile text, **`max_jobs`** for stage 3 from HTTP, optional **`pipeline_run_id`** for stage-3-only).
2. **Ingest kinds**: start **child** `IngestSourceWorkflow` per `source_id` **in parallel**; aggregate per-source outputs.
3. When stage 2 or 3 runs: **`CreateRun(ctx, &slotID)`** when a new **`pipeline_runs`** row is required; pass **`pipeline_run_id`** into stage-2 and/or stage-3 units as defined by **`007`**.
4. **Order**: stage 2 **before** stage 3 when both run in one parent execution.
5. Aggregate counts/errors (§5); return Temporal **workflow id** / **run id** and **`pipeline_run_id`** when created.

**Determinism**: workflow code deterministic; DB/Redis/HTTP only in activities.

### 4.3 Frozen Go identifiers (`internal/manual/schema`)

Temporal **registration strings** and **run kinds** below match `github.com/andrewmysliuk/jobhound_core/internal/manual/schema` (same PR as this table). Parent workflow and split activities are registered under these names when §F lands; ingest names are already live in the worker.

| Role | Registered Temporal name | Go constant |
|------|---------------------------|-------------|
| Parent manual slot run (planned) | `ManualSlotRunWorkflow` | `ManualSlotRunWorkflowName` |
| Persisted stage 2 only (planned) | `PersistPipelineStage2Activity` | `PersistPipelineStage2ActivityName` |
| Persisted stage 3 only (planned) | `PersistPipelineStage3Activity` | `PersistPipelineStage3ActivityName` |
| Per-source ingest workflow | `IngestSourceWorkflow` | `ingest_workflows.IngestSourceWorkflowName` (`internal/ingest/workflows`) |
| Per-source ingest activity | `RunIngestSourceActivity` | `ingest_activities.RunIngestSourceActivityName` (`internal/ingest/workflows/activities`) |
| Legacy bundled 2+3 persistence | `RunPersistedPipelineStagesActivity` | `pipeline_workflows.RunPersistedPipelineStagesActivityName` (`internal/pipeline/workflows`) |

**Run kinds** (JSON / workflow input): `RunKind` string constants — `INGEST_SOURCES`, `PIPELINE_STAGE2`, `PIPELINE_STAGE3`, `PIPELINE_STAGE2_THEN_STAGE3`, `INGEST_THEN_PIPELINE`, `DELTA_INGEST_THEN_PIPELINE`.

**Aggregate response DTO**: `ManualSlotRunAggregate` (+ `Stage2Aggregate`, `Stage3Aggregate`) — field JSON tags align with §5 column “Field” names (`snake_case`).

## 5. Response shape (API-facing aggregate)

| Field | Type | Notes |
|-------|------|------|
| `temporal_workflow_id` | string | Client or system id. |
| `temporal_run_id` | string | Temporal run id. |
| `pipeline_run_id` | int64, optional | When stage 2 and/or 3 persisted under a run. |
| `ingest` | map or array per `source_id` | **`ingest/schema.IngestSourceOutput`** where ingest ran. |
| `stage2` | object, optional | Counts: passed vs rejected (snapshot sizes). |
| `stage3` | object, optional | Counts: scored / cap (**`max_jobs`** effective batch per **`009`** / **`007`**), passed vs rejected at stage 3. |
| `error_summary` | string, optional | User-safe; no raw stacks. |

JSON tags are defined on `internal/manual/schema.ManualSlotRunAggregate` and nested types.

## 6. Job input set (frozen for `008`)

- **After stage 1**: jobs linked via **`slot_jobs`** with **`jobs.stage1_status = PASSED_STAGE_1`** (see root **`spec.md`**).
- **Stage 3 eligible set**: **`PASSED_STAGE_2`** rows for the relevant **`pipeline_run_id`** (**`007`**), ordered by **`posted_at`** DESC, **`job_id`** ASC; take up to the effective **`max_jobs`** for that execution (**`009`** + **`007`**).

**`JobRepository`** gains (or a sibling repo gains) **slot-scoped** queries joining **`slot_jobs`** and **`jobs`**.

## 7. Implementation snapshot vs this contract

| Item | Status |
|------|--------|
| **`IngestSourceWorkflow`** + **`RunIngestSource`** | **Implemented** — per-job **`slot_jobs`** row after **`SaveIngest`**; **`jobs.stage1_status = PASSED_STAGE_1`** set only via **`SaveIngest`** (006/007). |
| **`slot_jobs` table + GORM model** | **Implemented** — migration **`000002`**, **`internal/jobs/storage.SlotJob`**, **`ListSlotJobsPassedStage1`** / **`UpsertSlotJob`** (§6). |
| **Separate stage-2 and stage-3 Temporal units** | **Implemented** — **`PersistPipelineStage2Activity`** / **`PersistPipelineStage3Activity`**. |
| **Parent manual slot workflow** | **Implemented** — **`ManualSlotRunWorkflow`** (`internal/manual/workflows`). |
| **HTTP / stable start payload** | **`009`** |
| **Filter invalidation (slot snapshots)** | **Implemented** — see [`filter-invalidation.md`](./filter-invalidation.md) |
| **`cmd/agent` Temporal hook** | **Not implemented** |

## 8. Related specs & contracts

- [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)
- [`../../003-temporal-orchestration/contracts/environment.md`](../../003-temporal-orchestration/contracts/environment.md)
- [`../../004-pipeline-stages/contracts/pipeline-stages.md`](../../004-pipeline-stages/contracts/pipeline-stages.md)
- [`../../006-cache-and-ingest/contracts/redis-ingest-coordination.md`](../../006-cache-and-ingest/contracts/redis-ingest-coordination.md)
- [`../../007-llm-policy-and-caps/contracts/pipeline-run-job-status.md`](../../007-llm-policy-and-caps/contracts/pipeline-run-job-status.md)
