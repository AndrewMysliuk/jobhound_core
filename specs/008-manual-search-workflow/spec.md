# Feature: Manual search workflow

**Feature Branch**: `008-manual-search-workflow`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-04  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — **§2**–**§5** as updated with **`009`**; **§3** (repeat ingest on the same slot is **not** in public HTTP MVP—see **`009`**).

## Goal

Provide **on-demand** orchestration (Temporal) for a **search slot** (`slot_id`): **stage-1 ingest** (parallel per source), **stage 2** and **stage 3** as **separate** persisted Temporal units, and **parent** workflows that compose run kinds (see [`contracts/manual-workflow.md`](./contracts/manual-workflow.md)).

**Public HTTP (`009`)** maps to this epic as follows: **`POST /api/v1/slots`** starts **stage 1 only** (ingest). **Stage 2** and **stage 3** are **separate** HTTP actions (`PIPELINE_STAGE2` then, when the client chooses, **`PIPELINE_STAGE3`** with a caller-supplied **`max_jobs`**). There is **no** HTTP-triggered re-ingest for an existing slot. Composite run kinds (e.g. **`INGEST_THEN_PIPELINE`**, **`PIPELINE_STAGE2_THEN_STAGE3`**, **`DELTA_INGEST_THEN_PIPELINE`**) remain for **CLI, tests, internal hooks, or future product features**—not the default browser MVP flow.

Triggers: **`009`**, CLI, tests, internal hooks—not cron.

The outcome is a **stable request/response contract** (DTOs in module `schema/`) and **documented data rules** (`slot_jobs`, snapshots, filter invalidation) so **`009`** stays a thin HTTP layer.

## Product model (UI vs engine)

- **Three user-visible phases**: (1) pull listings per **backend-configured** sources (client supplies only the broad keyword string on slot create—**`009`**), (2) include/exclude filters → passed / not passed, (3) LLM scoring → passed / not passed. The backend may add identifiers (`pipeline_run_id`, Temporal ids, etc.).
- **HTTP MVP (`009`)**: create slot → **stage 1** runs once; client then **`POST …/stages/2/run`** (invalidates 2+3, recomputes **2 only**), then **`POST …/stages/3/run`** with **`max_jobs`** when ready. A single parent workflow that runs **1 → 2 → 3** in one shot is **not** the default public API shape; it may still be used from **CLI/tests** via composite run kinds.
- **Parallelism**: fetches to **different sources are independent**—run **in parallel** (e.g. one child `IngestSourceWorkflow` per `source_id`). Scaling to many sources does not require sequential crawling.

## Data model

### Slot ↔ job membership

- Add a **`slot_jobs`** (name TBD) association: **`slot_id` + `job_id`** (unique pair), plus any useful metadata the implementation needs (e.g. `first_seen_at`). This is **normative**: job rows in **`jobs`** are not slot-scoped by themselves; the slot’s candidate set is **`slot_jobs` ∩ `jobs`**.

### Stage 1 “who is in the pool”

- **`jobs.stage1_status`** with **`PASSED_STAGE_1`** after successful broad/stage-1 ingest is **sufficient** for “post–stage-1 pool” together with **`slot_jobs`**. No separate “stage-1 snapshot table” is required.

### Stages 2 and 3 as snapshots

- **Intermediate results** for stages **2** and **3** are **snapshots** (materialized per run / per slot policy): which jobs **passed** or **failed** under the **current** stage-2 and stage-3 rules. The engine does **not** treat this as “one job row that keeps moving between abstract statuses” for UI semantics—it is **stored outcome sets** that can be **deleted and recreated** when rules change.
- Implementation may map snapshots to **`pipeline_run_jobs`** (or follow-on tables) keyed by **`pipeline_run_id`** and **`job_id`** with statuses such as `REJECTED_STAGE_2`, `PASSED_STAGE_2`, `PASSED_STAGE_3`, `REJECTED_STAGE_3` (**`007`**). **`008`** requires that **stage 2** and **stage 3** are **separate Temporal workflows or activities** (see below)—not a single combined persisted step.

### Filter change → invalidate snapshots

When **filter rules** for a slot change:

| What changed | What to delete / drop |
|--------------|------------------------|
| **Stage 3 only** (e.g. profile / LLM policy) | **Only** stage-3 snapshot data (lists / rows for stage 3). Stage-2 snapshots stay valid. |
| **Stage 2** (match / exclusion) | **Stage 2 and stage 3** snapshot data—stage 3 **depends** on stage 2. |

Then **recompute** the affected stages on the next manual run (no implicit full re-crawl when only filters changed—align with product draft **§5**). For **HTTP**, stage-2 and stage-3 recomputes are **separate** client calls after invalidation (**`009`**); engine wipe semantics are unchanged.

*Where* the delete runs (e.g. first activity of the workflow vs API pre-step) is an implementation detail; the **rule** is normative for **`008`**.

## Stage rules (normative)

### Stage 1 — ingest

- **Per-source limit** on **parsing / collector response** (order of **~100 listings per source** for MVP). This is **not** a Postgres row cap; do not conflate with LLM batch size.
- Uses **`006`** Redis lock, cooldown, **slot-scoped watermarks**, same path as any other slot (**manual** does **not** bypass coordination).
- First successful ingest for a slot follows product **§2** / **`006`** (immutable broad keyword string where applicable). Incremental “pull new” on the **same** slot is **not** exposed in public HTTP MVP (**`009`**); watermarks remain valid for **`006`** when a future API or schedule adds repeat ingest.

### Stage 2 — filters

- **Input**: jobs that belong to the slot (**`slot_jobs`**) and have **`PASSED_STAGE_1`** (see **`004`** for stage math).
- **Output**: two logical lists—**passed** / **not passed** under current stage-2 rules—persisted as the **stage-2 snapshot** for the active run.

### Stage 3 — LLM

- **Input**: jobs in the **passed** list of the **stage-2 snapshot** (`PASSED_STAGE_2` in **`007`** terms).
- **LLM batch**: select jobs ordered by **`posted_at`** descending, then **`job_id`** ascending (same tie-break as **`009`** job lists). Batch size is **`max_jobs`** from the **`009`** request body (**1–100**, further capped by **`007`** policy—the **effective** count is the **minimum** of engine cap and caller request).
- **Output**: two lists—**passed** / **failed** at stage 3—persisted as the **stage-3 snapshot**.

## Temporal architecture (normative for this epic)

- **Stage 2** and **stage 3** MUST be **separate** registered workflows or activities (two distinct Temporal units with clear boundaries). **Bundling persisted stages 2 and 3 in a single activity** (as in today’s **`RunPersistedPipelineStages`**) is **legacy** and **must be split** to satisfy this spec.
- **Stage 1** remains **per-source** **`IngestSourceWorkflow`** (or equivalent), parallelized under a parent when multiple sources run.
- **Parent workflow** (composite run kinds) may compose: optional parallel ingest children → **stage-2 workflow** → **stage-3 workflow**, aggregating counts/errors ([`contracts/manual-workflow.md`](./contracts/manual-workflow.md)). **`009`** does not use a single **1→2→3** parent for the browser MVP.

Workflow code stays **deterministic**; I/O only inside activities (**`003`** pattern).

## Clarifications (legacy wording)

- **“Manual”** means **user- or operator-initiated**, not ad hoc SQL. It does **not** bypass **`006`**.
- **Filter-only** edits do **not** require new collector fetches; invalidate snapshots per the table above, then run **stage 2 only**, **stage 3 only**, or **2 then 3** (one workflow or two HTTP calls—**`009`** uses **separate** calls for 2 and 3).
- A future **scheduled** refresh epic should **reuse** the same activities/workflows; trigger differs (schedule vs explicit start).

## Scope

- **`002`** migration(s) for **`slot_jobs`** (+ indexes as needed); ingest path writes membership for the slot.
- **Temporal**: parent **manual slot run** workflow; **separate** stage-2 and stage-3 workflows (or activities with distinct names); parallel **ingest** children per source.
- **Repositories**: list jobs for a slot (`slot_jobs` + `jobs`), snapshot read/write for stages 2–3 per **`007`** (possibly split from monolithic persisted activity).
- **Stable DTOs** and **response shape** for **`009`**—see [`contracts/manual-workflow.md`](./contracts/manual-workflow.md).

## Out of scope

- **Full public REST** and slot CRUD (**`009`**).
- **Schedule definition** and append-only tick history (product backlog).
- **Observability** beyond **`010`**.

## Dependencies

- **`003`** Temporal patterns.  
- **`004`** stage logic (pure rules; may be invoked from thinner activities).  
- **`005`** / **`006`** collectors, normalize, persist, watermarks, Redis lock.  
- **`007`** `pipeline_runs`, `pipeline_run_jobs`, caps, idempotency for stage 3—adapt when splitting activities.  
- **`002`** migrations for **`slot_jobs`**.  

**Contracts**: [manual-workflow](./contracts/manual-workflow.md), [environment](./contracts/environment.md). Stage-3 batch size for HTTP is **`max_jobs`** from **`009`** (see **`007`** for engine-side caps and defaults for non-HTTP callers).

## Implementation snapshot (`internal/`)

| Area | Location | What exists today |
|------|-----------|-------------------|
| Temporal worker | `cmd/worker/main.go` | Pipeline activities (**including bundled** `RunPersistedPipelineStages`), ingest workflow, jobs retention, reference demo. |
| Ingest | `internal/ingest/workflows/`, `.../activities/`, `internal/ingest/schema/` | **`IngestSourceWorkflow`** → **`RunIngestSource`**; **`SlotID`** required; **`PASSED_STAGE_1`** via **`SaveIngest`**. |
| Pipeline (legacy bundle) | `internal/pipeline/workflows/activities/stages.go` | **`RunPersistedPipelineStages`** does **both** stage 2 and stage 3 persistence in **one** activity—**contradicts** this spec; **split** during **`008`** implementation. |
| Pipeline run header | `internal/pipeline/storage/`, `internal/pipeline/contract.go` | **`CreateRun(ctx, slotID)`** → **`pipeline_runs.slot_id`**. |
| Jobs repo | `internal/jobs/` | No **`slot_jobs`**; no slot-scoped list API yet. |
| Agent / debug HTTP | `cmd/agent/`, `internal/collectors/handlers/debughttp/` | Collectors only; **no** Temporal client for manual runs. |
| Reference client | `internal/reference/workflows/client.go` | **`ExecuteWorkflow`** pattern for tests/CLI. |

**Gaps for this epic**: **`slot_jobs`** migration + writes on ingest + queries; **split** persisted pipeline into **stage-2** and **stage-3** Temporal units; **parent** manual workflow for composite kinds; wire **invalidation** with **`009`** (`POST …/stages/2/run` carries **include/exclude**; profile via **`PUT /profile`** + **`POST …/stages/3/run`**).

## Local / Docker

Compose: **Temporal** + **Postgres**; ingest paths need **Redis** and configured collectors. Start workflows from worker tests, a small Temporal client (see `internal/reference/workflows`), or future **`cmd/agent`** / CLI. HTTP entrypoints—**`009`**.
