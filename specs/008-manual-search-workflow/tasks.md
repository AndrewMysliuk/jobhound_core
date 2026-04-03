# Tasks: Manual search workflow (on-demand slot orchestration)

**Input**: `spec.md`, `plan.md`, [`contracts/manual-workflow.md`](./contracts/manual-workflow.md)  
**Depends on**: `002` (migrations tooling), `003` (Temporal worker), `004` (stage logic), `005`/`006` (collectors, ingest, Redis, watermarks), `007` (`pipeline_runs`, `pipeline_run_jobs`, stage-3 persistence semantics)  
**Tests**: Unit tests without mandatory network; Temporal via **in-memory** test env and/or `//go:build integration` against Compose; plain `go test ./...` stays Docker-free.

## Implementation order

| Order | Section | Rationale |
|-------|---------|-----------|
| 1 | [A](#a-contracts--docs) | Freeze contract ↔ code names and response shape before wiring. |
| 2 | [B](#b-migrations--slot_jobs) | **`slot_jobs`** required for pool semantics and ingest writes. |
| 3 | [C](#c-storage--queries) | Repos power stage-2 input set and contract §6 job sets. |
| 4 | [D](#d-ingest-alignment) | Ingest must populate **`slot_jobs`** and obey **`006`**. |
| 5 | [E](#e-split-stage-2--stage-3-temporal-units) | Unblocks correct Temporal boundaries per spec. |
| 6 | [F](#f-parent-manual-slot-workflow) | Composes children + split units + run kinds. |
| 7 | [G](#g-filter-invalidation) | Normative data rules; callable from **`009`** when filters save. |
| 8 | [H](#h-schema-dtos--client-path) | Stable types for **`009`** and tests. |
| 9 | [I](#i-quality-gates) | Release checks. |

---

## A. Contracts & docs

1. [x] **Manual workflow contract vs code** — Definition of done: registered workflow/activity names, run kinds, and aggregate response fields in [`contracts/manual-workflow.md`](./contracts/manual-workflow.md) match implementation (or contract updated in same PR).  
2. [x] **Environment contract** — Definition of done: if this epic introduces or changes any `JOBHOUND_*` knob (e.g. stage-3 batch cap alignment with **`008`**), document in `contracts/environment.md` and `internal/config`; if none, add a one-line **`008`** note that no new keys are required (per root `spec.md`).

## B. Migrations — `slot_jobs`

1. [x] **Versioned migration** — Definition of done: `migrations/*.up.sql` / `*.down.sql` create **`slot_jobs`** with **`slot_id`**, **`job_id`**, unique pair constraint, FKs/indexes as needed for slot-scoped joins; `down` safe for dev.  
2. [x] **GORM models** — Definition of done: model(s) under agreed `storage/` package; no GORM in `internal/domain` unless domain already owns the type pattern.

## C. Storage & queries

1. [x] **Link jobs to slots** — Definition of done: idempotent **upsert** (or equivalent) of **`(slot_id, job_id)`** on successful ingest paths that introduce jobs to a slot.  
2. [x] **Slot-scoped job reads** — Definition of done: repository methods join **`slot_jobs`** + **`jobs`** for stage-2 pool (**`PASSED_STAGE_1`** per **`004`** / root spec) and any helpers needed for stage-3 selection input (**`007`** / **`PASSED_STAGE_2`** by `pipeline_run_id`).  
3. [x] **Tests** — Definition of done: table-driven tests for membership and query invariants (empty slot, dedup pair, FK expectations if enforced in tests).

## D. Ingest alignment

1. [x] **`slot_jobs` writes** — Definition of done: **`IngestSourceWorkflow` / `RunIngestSource`** (or single persistence choke point) writes membership for the slot; does **not** bypass **`006`** lock / cooldown / watermarks.  
2. [x] **Stage-1 status** — Definition of done: **`PASSED_STAGE_1`** on **`jobs`** remains the stage-1 completion signal used for post–stage-1 pool (consistent with **`006`** / **`007`**).

## E. Split stage-2 & stage-3 Temporal units

1. [x] **Two distinct units** — Definition of done: **separate** registered Temporal workflows or distinctly named activities for **persisted stage 2 only** and **persisted stage 3 only**; **no** single activity that persists both stages for product paths.  
2. [x] **Migrate call graph** — Definition of done: internal pipeline execution uses split units; legacy **`RunPersistedPipelineStages`** is not used for new manual paths (removed, unregistered, or explicitly deprecated per `plan.md` D3).  
3. [x] **Stage-3 selection rule** — Definition of done: eligible jobs ordered by **`jobs.posted_at` DESC**, **up to 20** per batch rule in **`008`** spec/contract; idempotent writes consistent with **`007`**.  
4. [x] **Worker registration** — Definition of done: `cmd/worker` registers both units with options from **`internal/platform/temporalopts`** (or equivalent).

## F. Parent “manual slot run” workflow

1. [x] **Input/output schema** — Definition of done: workflow input includes **`slot_id`**, run kind, and parameters per [`contracts/manual-workflow.md`](./contracts/manual-workflow.md) §3–§4; output matches §5 aggregate (counts, optional `pipeline_run_id`, ingest map, `error_summary`).  
2. [x] **Parallel ingest** — Definition of done: for ingest kinds, **one child `IngestSourceWorkflow` per `source_id`** (or equivalent parallelism); aggregate per-source results.  
3. [x] **`pipeline_runs`** — Definition of done: **`CreateRun(ctx, slotID)`** (or agreed API) when a new run header is required; pass **`pipeline_run_id`** into stage-2/stage-3 units per **`007`**.  
4. [x] **Ordering** — Definition of done: when both stage 2 and 3 run in one execution, **stage 2 completes before stage 3**; workflow code remains **deterministic** (I/O only in activities).  
5. [x] **Tests** — Definition of done: in-memory Temporal test covers at least one run kind end-to-end (e.g. `PIPELINE_STAGE2` only + mock storage, or full path with integration tag).

## G. Filter invalidation

1. [x] **Stage-3-only rule** — Definition of done: when stage-3 rules change, delete **only** stage-3 snapshot data for the affected slot/run policy (per root `spec.md` table).  
2. [x] **Stage-2 rule** — Definition of done: when stage-2 rules change, delete **stage-2 and stage-3** snapshot data (stage 3 depends on stage 2).  
3. [x] **Integration point** — Definition of done: documented whether **`009`** calls invalidation on filter save or first activity of a run; helpers live in **`internal/pipeline/storage`** (or agreed module).

## H. Schema DTOs & client path

1. [x] **Module `schema/`** — Definition of done: request/response (or workflow payload) types live under **`internal/manual/schema`** or split across **`ingest/schema`**, **`pipeline/schema`** per contract—**no** ad-hoc anonymous structs at HTTP boundary for **`009`**.  
2. [x] **Programmatic starter** — Definition of done: tests and/or small client (reuse **`internal/reference/workflows`** pattern) can start the parent workflow on queue **`jobhound`** / namespace **`default`** for manual verification.

## I. Quality gates

1. [x] **`make test` / `go test ./...`** — Definition of done: passes without mandatory Docker/network for default tests.  
2. [x] **`make vet` / `make fmt`** — Definition of done: clean for touched packages.  
3. [x] **Optional: integration** — Definition of done: `//go:build integration` test for migration + worker registration smoke, if team adopts for this epic.  
   - Migration: [`TestMigrationsSlotJobs008_integration`](../../internal/platform/pgsql/migrations_integration_test.go) (`slot_jobs` shape + index + FK).  
   - Worker registration: [`TestManualSlotRunWorkflow_againstServer`](../../internal/manual/workflows/client_integration_test.go) and [`TestReferenceDemoWorkflow_againstServer`](../../internal/reference/workflows/reference_integration_test.go) against a running `bin/worker`.

---

## Optional / deferred (do not block `008` closure)

1. [x] **`cmd/agent` Temporal hook** — Convenience only; **`009`** remains primary product trigger. (`bin/agent -temporal-manual-slot-run` + `-manual-*` flags; JSON aggregate on stdout.)  
2. [x] **Config-backed stage-3 batch size** — Unified with pipeline config: **`JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN`** (`internal/config`, worker → activities); documented in [`contracts/environment.md`](./contracts/environment.md).
