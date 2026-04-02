# Feature: Stage cap before stage 3 and job pipeline statuses

**Feature Branch**: `007-llm-policy-and-caps`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) §4 — cap, **deterministic** ordering, eligible pool, Temporal idempotency; §10 — map **`pipeline_run`** to **search slot** (and **`user_id`** when multi-user lands).

## Goal

Define **how many** vacancies may enter **stage 3** of the pipeline (`004`) in **one execution of a pipeline run** (cap **N**, initially **5** via a code constant), and define **status values + persistence** so that:

- **`PASSED_STAGE_1`** reflects **canonical ingest** (broad stage 1) on the **`jobs`** row.
- **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** reflect **progress for a given vacancy inside a given pipeline run** — the same `job_id` may have **different** stage-2/3 outcomes across runs (e.g. different keyword sets or different slots). There is **no** `REJECTED_STAGE_1`.

**Pipeline scope (MVP):** a **`pipeline_run`** is **scoped to a search slot** via **`pipeline_runs.slot_id`** (see contract). **Multi-user isolation** is reserved through **`user_id`** on headers or slots when registration exists; single-tenant MVP may omit auth but **schema** carries the hooks. **No** “global shared pipeline” for unrelated slots — **caps, ordering, and eligibility** are evaluated **per run** (and thus **per slot** once **`slot_id`** is set).

## Background

- Stages 1–3 are defined in `004` (stage 3 is LLM-backed scoring behind `internal/llm.Scorer`). This spec does **not** change filter or scorer semantics; it caps **how many** `(job, pipeline_run)` pairs in **`PASSED_STAGE_2`** are sent to stage 3 **per pipeline-run execution**, and defines **status enum + persistence**.
- Broad ingest, **slot-scoped** reuse + delta, and **`broad_filter_key_hash`** semantics are **`006`**. Durable run history / workflow IDs in Postgres are **not** required to implement this feature; they can be added later without changing the **`pipeline_run_id`** FK shape introduced here.

## Where statuses live

| Location | Status values |
| -------- | --------------- |
| **`jobs`** | `PASSED_STAGE_1` only for stage-1 completion (no `REJECTED_STAGE_1`). |
| **Per `(job_id, pipeline_run_id)`** (or equivalent join row) | `REJECTED_STAGE_2`, `PASSED_STAGE_2`, `PASSED_STAGE_3`, `REJECTED_STAGE_3`. |

## Job / pipeline status enum

There is **no** `REJECTED_STAGE_1`.

| Value | Meaning |
| ----- | ------- |
| `PASSED_STAGE_1` | Broad stage 1 succeeded; normalized vacancy is stored on **`jobs`**. |
| `REJECTED_STAGE_2` | For **this pipeline run**, stage 2 (keywords) did not pass. |
| `PASSED_STAGE_2` | For **this pipeline run**, stages 1 and 2 passed; stage 3 has **not** yet produced a terminal outcome (includes **cap backlog** waiting for stage 3). |
| `PASSED_STAGE_3` | For **this pipeline run**, stage 3 completed with a **pass** outcome. |
| `REJECTED_STAGE_3` | For **this pipeline run**, stage 3 completed with a **reject** outcome. |

### Allowed transitions (within one pipeline run)

```
PASSED_STAGE_1 → (PASSED_STAGE_2 | REJECTED_STAGE_2)
PASSED_STAGE_2 → (PASSED_STAGE_3 | REJECTED_STAGE_3)
```

- **`PASSED_STAGE_1`** is set on **`jobs`** when the vacancy enters the store after successful broad stage 1; stage 2 operates on that pool and writes **per-run** rows for 2/3 outcomes.
- **Repeat** run of stage 3 for the same job in the same run (rescoring / manual “run stage 3 again”) is **not in scope for v1**.

## Cap (automatic path only)

- **N** is a **code constant** (initial value **5**; may later move to config without changing the rules below).
- **Eligible pool** for stage 3 in that execution: `(job, pipeline_run)` pairs in **`PASSED_STAGE_2`** that do **not** yet have a **terminal** stage-3 outcome (**`PASSED_STAGE_3`** or **`REJECTED_STAGE_3`**) for **this** run — including after a **stage-3-only reset** (product draft §5) or profile-driven invalidation that clears stage-3 outputs.
- From that **eligible** set, at most **N** jobs are sent to stage 3 in **one** execution of that run. **Ordering** must be **deterministic** and **documented** (same inputs → same selection order — e.g. **`job_id` ascending** or a stable timestamp column agreed in storage); retries and UI must not depend on nondeterministic DB row order.
- Pairs that remain **`PASSED_STAGE_2`** because they were **not** selected in that execution **stay** `PASSED_STAGE_2` (cap backlog) until a **later** pipeline-run execution or an explicit **“process next batch”**-style action (product draft §4; API/workflow shapes in **`010`** when implemented).
- **“One automatic run”** means **one pipeline-run execution** (e.g. one workflow execution handling that run). Cap **N** and idempotency below apply to **that** execution.

## Run safety

- Within **one** pipeline-run execution, the same `job_id` **must not** be sent to stage 3 **twice** (idempotent batch selection for that execution).
- Under **Temporal retries**, the same execution must **not** double-consume the cap or write **duplicate** conflicting outcomes for the same **`(pipeline_run_id, job_id)`** — align with product draft §4 and `003` idempotency expectations.

## Persistence

- **`jobs`**: column (or equivalent) for **`PASSED_STAGE_1`** after migration (`002` style) + GORM mapping in `internal/jobs/storage` (or agreed package).
- **Per pipeline run**: a table or rows keyed by **`(job_id, pipeline_run_id)`** holding **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** as the run progresses. **`pipeline_runs`** (minimal surrogate key + timestamps) and **`pipeline_run_jobs`** (or agreed name) are **created in `007`** migrations; later work may add columns to **`pipeline_runs`** without breaking FKs.
- On **hard-delete** of a `jobs` row (`006` retention), **delete related per-run rows** in the same cleanup path so there are **no dangling references**.

## Scope

- Cap **N** and selection rules before stage 3 **per pipeline-run execution**.
- Status enum and split persistence above; migrations + GORM.
- Align orchestration so executions respect cap and status updates (wiring may span `003` worker activities; details in implementation tasks).

## Out of scope

- **Event scheduling, rich run-history rows, workflow IDs in DB** — not in this spec; **`007`** only needs a **`pipeline_run_id`** FK target as defined in the implementation contracts.
- **HTTP API** shapes for listing/filtering (`010`).
- **Manual “run stage 3 on this single job”** and **rescoring** — not in v1.
- **Per-user auth and enforcement** — MVP may be single-tenant; **schema** reserves **`user_id`** / slot ownership for later epics.

## Dependencies

- `000` — [`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) (slots, reset rules, stage-3 policy).
- `001` — `domain.Job`, stable id.
- `002` — Postgres migrations, `jobs` table, storage layer.
- `004` — pipeline stages; stage 3 invocation.
- `006` — broad ingest, `PASSED_STAGE_1` on `jobs`, slot-scoped **watermark** + **broad filter key** + delta.

## Local / Docker

- No new services required for this spec alone; stage 3 tests continue to use **mock** LLM (`004`). Real Anthropic calls use `JOBHOUND_ANTHROPIC_API_KEY` per `internal/config` when wired.

## Implementation artifacts

Plan, task backlog, and frozen contracts: [`plan.md`](./plan.md), [`tasks.md`](./tasks.md), [`contracts/`](./contracts/), quality checklist [`checklists/requirements.md`](./checklists/requirements.md).

## Acceptance criteria

1. **N** is a named constant in code (initially **5**); behavior matches “at most **N** `(job, pipeline_run)` pairs from the **eligible** **`PASSED_STAGE_2`** pool enter stage 3 **per pipeline-run execution**”.
2. **`jobs`** carries **`PASSED_STAGE_1`** for vacancies after successful broad stage 1; **`REJECTED_STAGE_1` does not exist**.
3. **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** are stored **per `(job_id, pipeline_run_id)`** (or equivalent); transitions match the diagram within a run; **`PASSED_STAGE_2`** includes **cap backlog**.
4. One pipeline-run execution does **not** process the same job through stage 3 twice; selection order is **deterministic** and **documented** (see **`contracts/pipeline-run-job-status.md`**).
5. **Temporal** retries do **not** double-consume the cap or corrupt **`(pipeline_run_id, job_id)`** outcomes.
6. **`pipeline_runs`** carries **`slot_id`** (nullable only where migrations have not yet backfilled; **normative** for MVP slot-scoped runs) per contract.
7. Per-job **manual** stage-3 rescoring is **out** of v1; **batch** continuation for cap backlog may be **`010`** — **policy** stays in this spec.
8. Retention cleanup removes **dependent per-run rows** when a `jobs` row is deleted (`006`).
