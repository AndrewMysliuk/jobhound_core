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

**Where to call**: not fixed by **`008`**. Recommended: invoke the appropriate method from **`009`** after **successful persistence** of the slot filter change (stage-2 vs stage-3), so snapshot data never disagrees with the rules the user just saved. Deferring invalidation to the **first activity** of a subsequent manual run is allowed if the product batches configuration writes, but clients must then tolerate stale `pipeline_run_id` / snapshot references until that run starts.

**Which method**: map UI or API “what changed” to the table above; stage-2 edits must use `InvalidateStage2And3SnapshotsForSlot`.
