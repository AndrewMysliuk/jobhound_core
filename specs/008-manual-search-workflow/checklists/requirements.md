# Specification Quality Checklist: Manual search workflow

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-04-03  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (**on-demand** orchestration per **`slot_id`**, optional parallel stage-1 ingest, **separate** persisted stage 2 and stage 3, parent “run everything” workflow) is explicit — traceable to **`spec.md`** §Goal and [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) §2–§5, §9
- [ ] **Three user-visible phases** + **fourth** “all three in order” + **parallelism across sources** documented — traceable to **`spec.md`** §Product model
- [ ] **`slot_jobs`** semantics (**slot ∩ jobs** pool, not slot-scoped `jobs` rows alone) explicit — traceable to **`spec.md`** §Data model
- [ ] Stage 2 / 3 as **snapshots** (recreatable; not a single job-row status machine for UI) + **filter invalidation** table explicit — traceable to **`spec.md`** §Data model and [`contracts/manual-workflow.md`](../contracts/manual-workflow.md) §3
- [ ] Non-goals: full public REST / slot CRUD (**`009`**), **scheduled** refresh, observability beyond **`010`** — traceable to **`spec.md`** §Out of scope
- [ ] Dependencies **`002`**–**`007`** (migrations, Temporal, stages, collectors, Redis/ingest, `pipeline_runs` / `pipeline_run_jobs`) acknowledged — traceable to **`spec.md`** §Dependencies

## Requirement Completeness

- [ ] Temporal: stage 2 and stage 3 are **two distinct** registered units; **no** bundled persisted 2+3 on product paths — traceable to **`spec.md`** §Temporal architecture and **`contracts/manual-workflow.md`** §4.1
- [ ] **Parent** manual workflow: parallel **`IngestSourceWorkflow`** children, **`CreateRun`** when needed, **stage 2 before stage 3**, deterministic workflow / I/O in activities — traceable to **`contracts/manual-workflow.md`** §4.2 and **`spec.md`** §Temporal architecture
- [ ] **Run kinds** (`INGEST_SOURCES`, `PIPELINE_STAGE2`, `PIPELINE_STAGE3`, combinations, `INGEST_THEN_PIPELINE`, `DELTA_INGEST_THEN_PIPELINE`) and **invalidation** follow-ups documented — traceable to **`contracts/manual-workflow.md`** §3
- [ ] Stage 1: **per-source listing cap** (~**100**), **`006`** lock / cooldown / watermarks, **no** manual bypass — traceable to **`spec.md`** §Stage rules and **`006`** contracts
- [ ] Stage 2 input: **`slot_jobs`** ∩ **`jobs`** with **`PASSED_STAGE_1`** — traceable to **`spec.md`** §Stage rules and **`contracts/manual-workflow.md`** §6
- [ ] Stage 3: batch **≤ 20**, order **`posted_at` DESC** (tie-break implementation-defined) — traceable to **`spec.md`** §Stage rules and **`contracts/manual-workflow.md`** §3 / §6
- [ ] **Response aggregate** (`temporal_workflow_id`, `temporal_run_id`, optional `pipeline_run_id`, `ingest`, `stage2`, `stage3`, `error_summary`) — traceable to **`contracts/manual-workflow.md`** §5
- [ ] **DTO / `schema/`** placement and **`009`** as thin HTTP layer — traceable to **`spec.md`** §Goal / §Scope and **`contracts/manual-workflow.md`** §1

## Feature Readiness

- [ ] **`plan.md`** phases and **`tasks.md`** sections align with **`spec.md`** scope and **`contracts/manual-workflow.md`**
- [ ] Constitution alignment (Temporal, Postgres, `internal/config`, module layout) reflected in **`plan.md`** constitution check
- [ ] **`contracts/environment.md`**: either documents new **`JOBHOUND_*`** for this epic or explicitly states **none required** — traceable to **`spec.md`** §Dependencies and **`tasks.md`** §A

## Notes

- **Ordering**: stage-3 **scoring batch** selection for **`008`** uses **`posted_at` DESC** per root spec / manual-workflow contract; do not conflate with other ordering rules from **`007`** unless a single code path is documented to satisfy both.
- **Legacy**: **`RunPersistedPipelineStages`** (bundled) is **out** for product semantics once **`008`** ships — see **`spec.md`** §Implementation snapshot and **`plan.md`** D3.
