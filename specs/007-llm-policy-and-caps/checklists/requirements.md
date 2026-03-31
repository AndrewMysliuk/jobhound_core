# Specification Quality Checklist: LLM policy and caps

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-03-31  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (cap **N**, split `jobs` vs per-run statuses, global pipeline run v1) is explicit
- [ ] No `REJECTED_STAGE_1`; per-run enum for stages 2–3 is explicit
- [ ] Cap applies only to **`PASSED_STAGE_2`** in **one** pipeline-run **execution**
- [ ] Out of scope: Telegram (`010`), HTTP API (`011`), manual/rescore stage 3, per-user private pipelines
- [ ] Dependencies on `001`–`004`, `006`, `003` acknowledged (`002` migrations + storage)

## Requirement Completeness

- [ ] Status transitions match `spec.md` diagram; **`PASSED_STAGE_2`** includes cap backlog
- [ ] Idempotency: same `job_id` not through stage 3 twice in one execution
- [ ] Persistence: `jobs` column + `pipeline_run_jobs` (or agreed name) traceable to **`contracts/pipeline-run-job-status.md`**
- [ ] Cap **N** is a **named constant** (initially **5**), not an env var in v1 — traceable to **`contracts/environment.md`** and **`plan.md`**
- [ ] Retention: deleting `jobs` removes dependent per-run rows (`006`)

## Feature Readiness

- [ ] **`plan.md`** phases and **`tasks.md`** align with **`spec.md`** acceptance criteria
- [ ] Constitution alignment (Postgres, Temporal, config rules) reflected in **`plan.md`**
- [ ] **`pipeline_runs`** (minimal) **owned by `007`** — documented in **`plan.md`** D1 and reflected in migrations/tasks

## Notes

- **Ordering** of which **N** jobs are scored is implementation-defined — document in code when implementing selection.
- Later migrations may add columns to **`pipeline_runs`**; keep the same table name and PK type so **`pipeline_run_jobs`** FKs stay valid.
