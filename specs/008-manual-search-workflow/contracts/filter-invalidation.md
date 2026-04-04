# Contract: filter invalidation (slot snapshots)

**Feature**: `008-manual-search-workflow`  
**Normative rules**: root [`spec.md`](../spec.md) (table “Filter change → invalidate snapshots”).

## Storage helpers

| Rule (what changed) | API | Package | Behaviour |
|---------------------|-----|---------|-----------|
| Stage 3 only (e.g. profile / LLM policy) | `InvalidateStage3SnapshotsForSlot` | `internal/pipeline/storage` | For all `pipeline_runs` with `slot_id`, sets `pipeline_run_jobs.status` from `PASSED_STAGE_3` / `REJECTED_STAGE_3` back to `PASSED_STAGE_2`. Stage-2 rows unchanged. |
| Stage 2 (keyword include/exclude) | `InvalidateStage2And3SnapshotsForSlot` | `internal/pipeline/storage` | `DELETE` from `pipeline_runs` where `slot_id` matches; `pipeline_run_jobs` removed via `ON DELETE CASCADE`. |

Both methods reject `uuid.Nil` with an error. Runs with `slot_id IS NULL` are not matched (legacy rows).

The interface surface is `PipelineRunRepository` in `internal/pipeline/contract.go` (implemented by `internal/pipeline/storage.Repository`).

## Integration with **`009`**

**HTTP MVP** (**[`009`](../../009-http-public-api/spec.md)**): stage-2 **include/exclude** are sent on **`POST /api/v1/slots/{id}/stages/2/run`** (no separate “save filters” route). Run **`InvalidateStage2And3SnapshotsForSlot`** (and persist the new filter parameters) in the **API handler or first activity** of **`PIPELINE_STAGE2`** before recomputing stage 2. Stage-3-only invalidation applies when **`PUT /profile`** (or future stage-3 policy fields) completes—then **`POST …/stages/3/run`** with **`max_jobs`**.

Deferring invalidation entirely to the **first activity** of the workflow is allowed if snapshot helpers are only reachable from the worker; clients must tolerate stale snapshot references until the run starts.

**Which method**: map “what changed” to the storage table above; stage-2 runs must use `InvalidateStage2And3SnapshotsForSlot`.
