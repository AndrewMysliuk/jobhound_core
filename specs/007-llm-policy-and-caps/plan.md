# Implementation Plan: Stage cap before stage 3 and pipeline run statuses

**Branch**: `007-llm-policy-and-caps`  
**Date**: 2026-03-31  
**Last Updated**: 2026-04-02  
**Spec**: `specs/007-llm-policy-and-caps/spec.md`  
**Input**: Feature specification + `contracts/*` + [`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Introduce a **cap N** (initially **5**, **named code constant**) on how many `(job_id, pipeline_run_id)` pairs from the **eligible** **`PASSED_STAGE_2`** pool may enter **stage 3** in **one pipeline-run execution**, with **idempotent** batch selection and **deterministic** ordering (see contract). Persist **split statuses**: **`PASSED_STAGE_1`** on **`jobs`** (broad ingest / stage 1 complete — no `REJECTED_STAGE_1`), and **per-run** rows for **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`** keyed by **`(job_id, pipeline_run_id)`**. **`pipeline_runs`** includes **`slot_id`** so a run maps to a **search slot** (product draft). Wire **orchestration** (Temporal activities / worker, `003`) so executions load candidates, apply cap, invoke `004` stage 3 only for the selected batch, and update rows per **`contracts/pipeline-run-job-status.md`**.

This spec does **not** change stage 1–3 **math** from `004`; it adds **policy + persistence + batching**.

## Technical Context

**Language/Version**: Go 1.24  
**Domain**: `internal/domain.Job` (`001`); stages **`004`**; ingest alignment **`006`**  
**Database**: PostgreSQL + versioned SQL migrations under `migrations/` + GORM models under feature `storage/` packages (`002` patterns)  
**Cap**: **Not** an environment variable in v1 — exported **constant** in agreed package (see Resolved decisions)  
**Orchestration**: Temporal worker (`003`) — activities select `PASSED_STAGE_2` candidates, enforce **N** and idempotency, call `internal/llm.Scorer` / pipeline batch APIs, persist per-run status transitions  
**Testing**: Unit tests without network; stage 3 continues to use **mock** LLM (`004`); real Anthropic only when key present (`contracts/environment.md`)  
**Cleanup**: On **hard-delete** of a `jobs` row (`006` retention), delete dependent per-run rows in the same path (**ON DELETE CASCADE** or explicit delete — see contract)

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| II. Stages before blanket LLM | **PASS** (cap limits stage 3 batch size; stages 1–2 unchanged) |
| IV. Postgres as system of record | **PASS** (statuses + cap bookkeeping in DB) |
| V. Temporal for orchestration | **PASS** (execution-scoped cap in workflow/activity layer) |
| VI. Config without secrets; config in `internal/config` | **PASS** (N is constant in v1; no `os.Getenv` in feature modules for cap; LLM key stays in `internal/config` per `004`) |
| Testing: `go test ./...` without Docker for unit tests | **PASS** (mocks + table-driven tests) |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Contracts | `contracts/environment.md`, `contracts/pipeline-run-job-status.md` |
| 1 Migrations | SQL `up`/`down`: `jobs` column for stage **1** status; **`pipeline_runs`** (minimal + **`slot_id`** per contract — supplement migration if initial ship omitted it); **`pipeline_run_jobs`** per contract; indexes and **FK** + **ON DELETE CASCADE** from per-run rows to `jobs` |
| 2 Domain & storage | GORM models + repository methods: upsert per-run status transitions; load `PASSED_STAGE_2` candidates for a run; optional domain types for enum |
| 3 Cap + selection | Pure helper or small service: given **eligible** `PASSED_STAGE_2` set + execution idempotency key, select ≤ **N** jobs; ordering **deterministic** per **`contracts/pipeline-run-job-status.md`** |
| 4 Orchestration | Worker activities / pipeline glue: after stage 2 outcomes written, run stage 3 batch loop respecting cap; update to **`PASSED_STAGE_3`** / **`REJECTED_STAGE_3`**; never double-send same `job_id` in one execution |
| 5 Ingest alignment | **`006`** path sets **`PASSED_STAGE_1`** on **`jobs`** when broad stage 1 completes; retention deletes cascade per contract |
| 6 Quality gates | `make test`, `make vet`, `make fmt`; integration tests for migrations optional (`integration` tag) |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **`pipeline_runs` ownership** | **`007`** migration creates **`pipeline_runs`** (minimal: surrogate PK + timestamps + **`slot_id`** per contract). Per-run rows **require** `pipeline_run_id` → **`pipeline_runs(id)`**. Later epics may **`ALTER TABLE pipeline_runs`** to add metadata (schedule, workflow ids, etc.) **without** renaming the table or changing the PK type. |
| D2 | **Per-run table name** | **`pipeline_run_jobs`** — canonical name in **`contracts/pipeline-run-job-status.md`** (may differ in code only if contract is updated). |
| D3 | **Cap constant location** | Exported identifier in **`internal/pipeline`** (e.g. `MaxStage3JobsPerPipelineRunExecution` = **5**) or `internal/config` package-level const — **one** place; referenced by selection logic and tests. |
| D4 | **Ordering** | **Normative**: among **eligible** rows (non-terminal stage 3 for this run), order by **`job_id` ascending** unless a later contract adds an explicit tie-breaker column — **same inputs → same order** (product draft §4). Document in selection code. |
| D5 | **Stage 3 failure** | Align with **`004`**: LLM failure may return **`error`**; activity decides abort vs mark job failed — document chosen behaviour in activities or contract addendum. |

## Engineering follow-ups (non-blocking)

- Move **N** from const to **`internal/config`** env without changing cap **rules** (spec allows later).  
- Manual / repeat stage 3 (`007` out of scope).  
- **`010`** Telegram — at most **N** notifications per run aligns with cap; delivery not defined here.

## Project structure (documentation)

```text
specs/007-llm-policy-and-caps/
├── spec.md
├── plan.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── environment.md
    └── pipeline-run-job-status.md
```

## Source structure (anticipated — implementation phase)

```text
internal/jobs/storage/           # jobs row + new stage1 column mapping
internal/pipeline/               # cap constant; optional batch selection helpers
internal/<feature>/storage/      # pipeline_run_jobs repo (or under jobs/ — match repo conventions)
internal/<feature>/workflows/    # optional: workflows/activities for run execution
internal/<feature>/activities/
migrations/                      # new *.up.sql / *.down.sql
```
