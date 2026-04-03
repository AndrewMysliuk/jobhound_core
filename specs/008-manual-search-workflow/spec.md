# Feature: Manual search workflow

**Feature Branch**: `008-manual-search-workflow`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-03  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — **§2** (search **slot** as unit of work; **`slot_id`**; schema reserves **`user_id`**), **§3** (first ingest vs later **“pull new”** / incremental path—exact trigger shape lives here and in **`009`**), **§4** (ordering and eligible pool per execution), **§5** (filter/profile **reset wipes** dependent outcomes), **§9** (manual/API triggers first; scheduled auto-refresh stays **product backlog** per draft §8).

## Goal

Provide **on-demand** orchestration (Temporal **preferred**) for a **search slot** (`slot_id`): optional **stage-1 ingest** (parallel per source), then **stage 2** and **stage 3** as **separate** persisted steps—each exposed as its own workflow (or equivalent registered Temporal unit), plus a **parent** workflow that runs **1 → 2 → 3** in sequence when the user submits a full “run everything” action.

Triggers: **API** (**`009`**), **CLI**, tests, or internal hooks—not cron.

The outcome is a **stable request/response contract** (DTOs in module `schema/` when implemented) and **documented data rules** (`slot_jobs`, snapshots, filter invalidation) so **`009`** stays a thin HTTP layer.

## Product model (UI vs engine)

- **Three user-visible phases**: (1) pull listings per configured source, (2) match / exclusion filters → two lists (passed / not passed), (3) LLM scoring → two lists (fits / does not fit). The backend may add identifiers (`pipeline_run_id`, Temporal ids, hashes of filter configs, etc.).
- **Fourth action**: start **all three phases in order** for one slot (parent workflow).
- **Parallelism**: HTTP fetches to **different sources are independent**—run them **in parallel** (e.g. one child `IngestSourceWorkflow` per `source_id`, or parallel activities). Scaling to many sources does not require sequential crawling.

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

Then **recompute** the affected stages on the next manual run (no implicit full re-crawl when only filters changed—align with product draft **§5**).

*Where* the delete runs (e.g. on **`009`** filter save vs first activity of a workflow) is an implementation detail; the **rule** is normative for **`008`**.

## Stage rules (normative)

### Stage 1 — ingest

- **Per-source limit** on **parsing / collector response** (order of **~100 listings per source** for MVP). This is **not** a Postgres row cap; do not conflate with LLM batch size.
- Uses **`006`** Redis lock, cooldown, **slot-scoped watermarks**, same path as any other slot (**manual** does **not** bypass coordination).
- First successful ingest for a slot still follows product **§2** / **`006`** (immutable broad keyword string where applicable); incremental “pull new” uses watermark state.

### Stage 2 — filters

- **Input**: jobs that belong to the slot (**`slot_jobs`**) and have **`PASSED_STAGE_1`** (see **`004`** for stage math).
- **Output**: two logical lists—**passed** / **not passed** under current stage-2 rules—persisted as the **stage-2 snapshot** for the active run.

### Stage 3 — LLM

- **Input**: jobs in the **passed** list of the **stage-2 snapshot** (`PASSED_STAGE_2` in **`007`** terms).
- **LLM batch**: select jobs with **`posted_at`** descending (most recent first), take **up to 20** for the scorer in that execution. **Secondary ordering when `posted_at` ties** is implementation-defined (not specified).
- **Output**: two lists—**fits** / **does not fit**—persisted as the **stage-3 snapshot**.

## Temporal architecture (normative for this epic)

- **Stage 2** and **stage 3** MUST be **separate** registered workflows or activities (two distinct Temporal units with clear boundaries). **Bundling persisted stages 2 and 3 in a single activity** (as in today’s **`RunPersistedPipelineStages`**) is **legacy** and **must be split** to satisfy this spec.
- **Stage 1** remains **per-source** **`IngestSourceWorkflow`** (or equivalent), parallelized under a parent when multiple sources run.
- **Parent workflow** composes: optional parallel ingest children → **stage-2 workflow** → **stage-3 workflow**, aggregating counts/errors for the response contract ([`contracts/manual-workflow.md`](./contracts/manual-workflow.md)).

Workflow code stays **deterministic**; I/O only inside activities (**`003`** pattern).

## Clarifications (legacy wording)

- **“Manual”** means **user- or operator-initiated**, not ad hoc SQL. It does **not** bypass **`006`**.
- **Filter-only** edits do **not** require new collector fetches; invalidate snapshots per the table above, then run **stage 2 only**, **stage 3 only**, or **2 then 3**, as appropriate.
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

**Contracts**: [manual-workflow](./contracts/manual-workflow.md), [environment](./contracts/environment.md) (no new env keys required solely for this epic; stage-3 batch size **20** may be config later—document in `internal/config` / env contract if added).

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

**Gaps for this epic**: **`slot_jobs`** migration + writes on ingest + queries; **split** persisted pipeline into **stage-2** and **stage-3** Temporal units; **parent** manual workflow; wire **invalidation** rules with **`009`** when filters are saved (or equivalent).

## Local / Docker

Compose: **Temporal** + **Postgres**; ingest paths need **Redis** and configured collectors. Start workflows from worker tests, a small Temporal client (see `internal/reference/workflows`), or future **`cmd/agent`** / CLI. HTTP entrypoints—**`009`**.
