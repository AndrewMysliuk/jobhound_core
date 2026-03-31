# Feature: Stage cap before stage 3 and job pipeline statuses

**Feature Branch**: `007-llm-policy-and-caps`  
**Created**: 2026-03-29  
**Last Updated**: 2026-03-31  
**Status**: Draft  

## Goal

Define **how many** vacancies may enter **stage 3** of the pipeline (`004`) in **one execution of a pipeline run** (cap **N**, initially **5** via a code constant), and define **status values + persistence** so that:

- **`PASSED_STAGE_1`** reflects **canonical ingest** (broad stage 1) on the **`jobs`** row.
- **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** reflect **progress for a given vacancy inside a given pipeline run** — the same `job_id` may have **different** stage-2/3 outcomes across runs (e.g. different keyword sets). There is **no** `REJECTED_STAGE_1`.

**Pipeline scope (v1):** a **pipeline run is global** (shared): different people may use the same broad search; they share the same pipeline identity for matching filters. Per-user private pipelines are **out of scope** unless added later.

## Background

- Stages 1–3 are defined in `004` (stage 3 is LLM-backed scoring behind `internal/llm.Scorer`). This spec does **not** change filter or scorer semantics; it caps **how many** `(job, pipeline_run)` pairs in **`PASSED_STAGE_2`** are sent to stage 3 **per pipeline-run execution**, and defines **status enum + persistence**.
- Broad ingest vs “same query” reuse + delta is **`006`**. Durable run history / workflow IDs in Postgres are **not** required to implement this feature; they can be added later without changing the **`pipeline_run_id`** FK shape introduced here.

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
- Applies **only** to `(job, pipeline_run)` pairs already in **`PASSED_STAGE_2`** when **this pipeline run’s execution** selects the batch for stage 3.
- From that set, at most **N** jobs are sent to stage 3 in **one** execution of that run. **Which** N (ordering) is **implementation-defined**; the spec does not require a particular sort.
- Pairs that remain **`PASSED_STAGE_2`** because they were **not** selected for stage 3 in that execution **stay** `PASSED_STAGE_2` until a later feature allows manual or additional processing (out of scope for v1 except as noted in “Out of scope”).
- **“One automatic run”** means **one pipeline-run execution** (e.g. one workflow execution handling that run). Cap **N** and idempotency below apply to **that** execution.

## Run safety

- Within **one** pipeline-run execution, the same `job_id` **must not** be sent to stage 3 **twice** (idempotent batch selection for that execution).

## Persistence

- **`jobs`**: column (or equivalent) for **`PASSED_STAGE_1`** after migration (`002` style) + GORM mapping in `internal/jobs/storage` (or agreed package).
- **Per pipeline run**: a table or rows keyed by **`(job_id, pipeline_run_id)`** holding **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** as the run progresses. **`pipeline_runs`** (minimal surrogate key + timestamps) and **`pipeline_run_jobs`** (or agreed name) are **created in `007`** migrations; later work may add columns to **`pipeline_runs`** without breaking FKs.
- On **hard-delete** of a `jobs` row (`006` retention), **delete related per-run rows** in the same cleanup path so there are **no dangling references**.

## Scope

- Cap **N** and selection rules before stage 3 **per pipeline-run execution**.
- Status enum and split persistence above; migrations + GORM.
- Align orchestration so executions respect cap and status updates (wiring may span `003` worker activities; details in implementation tasks).

## Out of scope

- **Telegram** message format and delivery (`010`); cap **aligns** with “at most N notifications per run” there but this spec does not define Telegram.
- **Event scheduling, rich run-history rows, workflow IDs in DB** — not in this spec; **`007`** only needs a **`pipeline_run_id`** FK target as defined in the implementation contracts.
- **HTTP API** shapes for listing/filtering (`011`).
- **Manual “run stage 3 on this job”** and **rescoring** — not in v1.
- **Per-user private pipelines** — not in v1; pipeline is **global** for matching filters.

## Dependencies

- `001` — `domain.Job`, stable id.
- `002` — Postgres migrations, `jobs` table, storage layer.
- `004` — pipeline stages; stage 3 invocation.
- `006` — broad ingest, `PASSED_STAGE_1` on `jobs`, normalized stage-1 query key + delta.

## Local / Docker

- No new services required for this spec alone; stage 3 tests continue to use **mock** LLM (`004`). Real Anthropic calls use `JOBHOUND_ANTHROPIC_API_KEY` per `internal/config` when wired.

## Implementation artifacts

Plan, task backlog, and frozen contracts: [`plan.md`](./plan.md), [`tasks.md`](./tasks.md), [`contracts/`](./contracts/), quality checklist [`checklists/requirements.md`](./checklists/requirements.md).

## Acceptance criteria

1. **N** is a named constant in code (initially **5**); behavior matches “at most **N** `(job, pipeline_run)` pairs in **`PASSED_STAGE_2`** enter stage 3 **per pipeline-run execution**”.
2. **`jobs`** carries **`PASSED_STAGE_1`** for vacancies after successful broad stage 1; **`REJECTED_STAGE_1` does not exist**.
3. **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** are stored **per `(job_id, pipeline_run_id)`** (or equivalent); transitions match the diagram within a run; **`PASSED_STAGE_2`** includes **cap backlog**.
4. One pipeline-run execution does **not** process the same job through stage 3 twice.
5. Repeat stage-3 / manual stage-3 triggers are **explicitly** out of v1 (no API/workflow requirement in this spec).
6. Retention cleanup removes **dependent per-run rows** when a `jobs` row is deleted (`006`).
