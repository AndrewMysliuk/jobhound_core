# Specification Quality Checklist: Cache and ingest

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-03-31  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (Postgres durable store, Redis lock/cooldown, no Redis search cache v1) is explicit
- [ ] **Slot-scoped** broad filter key + **per `(slot_id, source_id)`** watermark + retention + explicit refresh covered — traceable to **`spec.md`** and [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md)
- [ ] Non-goals: public API (`009`), third-party notifications, scheduler UI
- [ ] Dependencies: `001`, `002`, `005`, `007` (stage-1 column + `pipeline_run_jobs` CASCADE) acknowledged

## Requirement Completeness

- [ ] Redis: key patterns, TTL defaults (**600** / **3600** s), **fail closed** when Redis down — traceable to **`contracts/redis-ingest-coordination.md`**
- [ ] Explicit refresh: **cooldown bypass only**, **lock always** — traceable to **`spec.md`** and **`contracts/environment.md`**
- [ ] Watermark table **`ingest_watermarks`** — traceable to **`contracts/ingest-watermark-and-filter-key.md`**
- [ ] **`broad_filter_key_hash`** on **`pipeline_runs`** (or documented alternative) — traceable to same contract
- [ ] **`PASSED_STAGE_1`** / column name — **single source**: **`007`** `contracts/pipeline-run-job-status.md`
- [ ] Description-only update: **does not** reset per-run pipeline stages — traceable to **`plan.md`** D7
- [ ] Retention: cron **every 7 days UTC**, manual path, **7-day** cutoff, CASCADE — traceable to **`contracts/retention-jobs.md`**

## Feature Readiness

- [ ] **`plan.md`** phases and **`tasks.md`** align with **`spec.md`** and contracts
- [ ] **`JOBHOUND_REDIS_URL`** and **`JOBHOUND_INGEST_EXPLICIT_REFRESH`** documented in **`contracts/environment.md`**

## Notes

- Worst-case **wall-clock** retention may exceed 7 days slightly because the scheduled job runs **at most** every 7 days — see **`retention-jobs.md`**.
