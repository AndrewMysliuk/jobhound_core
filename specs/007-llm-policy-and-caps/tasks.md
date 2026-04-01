# Tasks: Stage cap before stage 3 and pipeline run statuses

**Input**: `spec.md`, `plan.md`, `contracts/*`  
**Depends on**: `001`, `002`, `004`, `003` (worker / activities wiring). Spec **`006`** is required only for [Phase 2](#phase-2-after-006).  
**Tests**: Unit tests **without** real LLM network by default (`004` mocks); `integration` build tag optional for migrations against Postgres.

## Implementation order (cross-spec)

### Phase 1: Before 006

Do first; unblocks **`specs/006-cache-and-ingest`**. Spec **`006`** depends on **`pipeline_runs`**, **`pipeline_run_jobs`**, and the **`jobs`** stage-1 column from **`007`** migrations and contracts. Complete the sections below **before** implementing ingest, Redis, watermark, and retention in **`006`**.

| Section | Scope |
| -------- | ------ |
| [A. Contracts & docs](#a-contracts--docs) | Frozen contracts; no contradiction with `004`. |
| [B. Migrations & schema](#b-migrations--schema) | `jobs` stage-1 column, `pipeline_runs`, `pipeline_run_jobs` + **ON DELETE CASCADE** to `jobs`. |
| [C. Storage layer (GORM)](#c-storage-layer-gorm) | Models, repos, load `PASSED_STAGE_2` candidates, transitions — tests may use fixtures / seeded rows. |
| [D. Cap & batch selection](#d-cap--batch-selection) | Constant **N**, selection + idempotency (pure logic). |
| [E. Orchestration](#e-orchestration-temporal--pipeline-glue) | Stage 2 → cap → stage 3 path; **`PASSED_STAGE_1`** on rows may come from **test data** until **`006`** sets it in prod. |

**Definition of done for this phase:** migrations applied; storage + cap + orchestration covered by unit tests; worker wiring per `003`. Ingest does not need to exist yet.

### Phase 2: After 006

Requires **`specs/006-cache-and-ingest`** (ingest path + retention).

| Section | Why spec **006** |
| -------- | ---------------- |
| [F. Ingest & retention alignment](#f-ingest--retention-alignment) | **`PASSED_STAGE_1`** must be set on the **ingest** path when broad stage 1 completes; **retention** hard-delete runs in **`006`** — verify no dangling `pipeline_run_jobs` (CASCADE or same transaction per contracts). |

**Definition of done for this phase:** end-to-end path from real ingest through stage 2/3 matches `spec.md`; deleting old `jobs` via `006` retention does not leave orphan per-run rows.

---

## A. Contracts & docs

1. [x] **Contracts match intent** — Definition of done: `contracts/pipeline-run-job-status.md` and `contracts/environment.md` align with `spec.md` acceptance criteria and `plan.md` resolved decisions; no contradiction with `004` stage semantics.

## B. Migrations & schema

1. [x] **`jobs` stage-1 status** — Definition of done: versioned `up`/`down` under `migrations/` adds column per contract §3; `down` is safe for dev/CI; matches `contracts/pipeline-run-job-status.md`.

2. [x] **`pipeline_runs` (minimal)** — Definition of done: migration creates `pipeline_runs` per contract §4 (same change set as `pipeline_run_jobs`); surrogate PK + timestamps; no dependency on other specs.

3. [x] **`pipeline_run_jobs`** — Definition of done: migration creates table per contract §5 — PK `(pipeline_run_id, job_id)`, FKs to `jobs` and `pipeline_runs`, **ON DELETE CASCADE** from `job_id`, index for `(pipeline_run_id, status)` (or equivalent for candidate queries).

## C. Storage layer (GORM)

1. [x] **Models + mapping** — Definition of done: GORM models under agreed `storage/` package; no GORM in `internal/domain`; `TableName()` / tags; enum ↔ string or typed fields consistent with contract.

2. [x] **Repository API** — Definition of done: methods to insert/update per-run status through allowed transitions; load `PASSED_STAGE_2` candidates for a `pipeline_run_id`; tests with sqlite/postgres mock or integration-tagged DB tests per repo practice.

## D. Cap & batch selection

1. [x] **Named constant N = 5** — Definition of done: single exported const (see `plan.md` D3); referenced by selection and unit tests.

2. [x] **Selection + idempotency** — Definition of done: for one execution, select at most **N** distinct `job_id` from `PASSED_STAGE_2`; same `job_id` not sent to stage 3 twice in that execution; ordering documented in code (implementation-defined).

## E. Orchestration (Temporal / pipeline glue)

1. [x] **Activities or pipeline runner** — Definition of done: after stage 2, persist `REJECTED_STAGE_2` / `PASSED_STAGE_2`; when running stage 3, respect cap; update to `PASSED_STAGE_3` / `REJECTED_STAGE_3`; aligns with `003` worker registration pattern.

2. [x] **Stage 3 invocation** — Definition of done: uses existing `004` / `internal/llm` scorer; no change to filter/scorer **math** unless contract gap found — then update `004` contract instead of forking semantics here.

## F. Ingest & retention alignment

*Prerequisite: [`006`](../006-cache-and-ingest/tasks.md) ingest, `PASSED_STAGE_1` wiring, and retention delete path exist.*

1. [x] **`PASSED_STAGE_1` on jobs** — Definition of done: `006` ingest path (or agreed single place) sets `jobs` stage-1 status when broad stage 1 completes; consistent with `006` spec and **`007`** contract.

2. [x] **Retention cleanup** — Definition of done: when a `jobs` row is hard-deleted per `006` retention, dependent `pipeline_run_jobs` rows are removed (CASCADE from `007` migration **or** explicit delete in the same path as `006`) — no dangling references.

## G. Quality gates

1. [x] **`make test` / `go test ./...`** — Definition of done: passes without mandatory network for default tests.

2. [x] **`make vet` / `make fmt`** — Definition of done: clean for touched packages.

## H. Optional / deferred

1. [x] **Integration: migrations** — Definition of done: `//go:build integration` test applies migrations and asserts tables/columns/indexes (same approach as `002` if adopted).

2. [x] **Config-backed N** — Definition of done: **Implemented** — `JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN` in `internal/config/pipeline.go` + `contracts/environment.md`; cap **rules** unchanged from `spec.md`.
