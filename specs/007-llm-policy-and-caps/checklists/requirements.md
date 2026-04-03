# Specification Quality Checklist: LLM policy and caps

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-03-31  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (cap **N**, split `jobs` vs per-run statuses, **`pipeline_runs.slot_id`**, deterministic ordering) is explicit — traceable to [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) §4
- [ ] No `REJECTED_STAGE_1`; per-run enum for stages 2–3 is explicit
- [ ] Cap applies to the **eligible** **`PASSED_STAGE_2`** pool in **one** pipeline-run **execution** (see **`spec.md`** / contract §2)
- [ ] Out of scope: third-party notification delivery, HTTP API (`009`), per-job manual rescoring; **auth** deferred — traceable to **`spec.md`**
- [ ] Dependencies on `001`–`004`, `006`, `003` acknowledged (`002` migrations + storage)

## Requirement Completeness

- [ ] Status transitions match `spec.md` diagram; **`PASSED_STAGE_2`** includes cap backlog
- [ ] Idempotency: same `job_id` not through stage 3 twice in one execution; **Temporal** retries do not double-consume cap or corrupt **`(pipeline_run_id, job_id)`** rows
- [ ] Persistence: `jobs` column + `pipeline_runs.slot_id` + `pipeline_run_jobs` traceable to **`contracts/pipeline-run-job-status.md`**
- [ ] Cap **N** is a **named constant** (initially **5**), not an env var in v1 — traceable to **`contracts/environment.md`** and **`plan.md`**
- [ ] Retention: deleting `jobs` removes dependent per-run rows (`006`)

## Feature Readiness

- [ ] **`plan.md`** phases and **`tasks.md`** align with **`spec.md`** acceptance criteria
- [ ] Constitution alignment (Postgres, Temporal, config rules) reflected in **`plan.md`**
- [ ] **`pipeline_runs`** (minimal) **owned by `007`** — documented in **`plan.md`** D1 and reflected in migrations/tasks

## Notes

- **Ordering** for cap selection is **normative** (`job_id` ascending) — see **`contracts/pipeline-run-job-status.md`** §2.
- Later migrations may add columns to **`pipeline_runs`**; keep the same table name and PK type so **`pipeline_run_jobs`** FKs stay valid.
