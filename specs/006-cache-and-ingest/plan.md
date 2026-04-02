# Implementation Plan: Cache and ingest

**Branch**: `006-cache-and-ingest`  
**Date**: 2026-03-31  
**Last Updated**: 2026-04-02  
**Spec**: `specs/006-cache-and-ingest/spec.md`  
**Input**: Feature specification + `contracts/*` + [`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Durable normalized vacancies in **PostgreSQL** (upsert by stable id per `001`); **Redis** coordinates ingest **per source** (lock + cooldown, **no** search-result cache in v1). Define **slot-scoped** **canonical broad filter key** (SHA-256 of canonical JSON including **`slot_id`**), **watermark** rows in Postgres **per `(slot_id, source_id)`** for incremental collectors (`005`), **ingest alignment** with **`PASSED_STAGE_1`** on `jobs` per **`007`** contract, **retention** hard-delete with CASCADE to **`pipeline_run_jobs`**, and optional **explicit refresh** (bypass cooldown only, default off). Wire **`JOBHOUND_REDIS_URL`**, scheduled **retention cron** (every 7 days UTC) plus **manual** same path, and Compose **Redis**.

## Technical Context

**Language/Version**: Go 1.24  
**Stores**: PostgreSQL + GORM (`002`); Redis for coordination only  
**Orchestration**: Temporal (`003`) for ingest activities and retention schedule where applicable  
**Config**: `internal/config` for `JOBHOUND_REDIS_URL`, `JOBHOUND_INGEST_EXPLICIT_REFRESH`, DB URL — no env reads in feature modules ad hoc  
**Testing**: Unit tests without real Redis/Postgres by default; `integration` tag optional for migrations / Redis if adopted

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| Postgres as system of record | **PASS** |
| Temporal for orchestration | **PASS** (ingest + retention triggers) |
| Config in `internal/config` | **PASS** |
| Unit tests colocated; integration tagged optional | **PASS** |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Contracts | Frozen `contracts/*` (already under this spec tree) |
| 1 Compose & config | Redis service + `JOBHOUND_REDIS_URL`; loaders in `internal/config` |
| 2 Migrations | `ingest_watermarks` **(`slot_id`, `source_id`)** — supplement if shipped as `source_id`-only; optional `pipeline_runs.broad_filter_key_hash` (depends on `007` `pipeline_runs` existing) |
| 3 Redis client | Thin wrapper or use std/miniredis in tests; lock + cooldown per `redis-ingest-coordination.md` |
| 4 Ingest core | Upsert `jobs`, equality skip except description, set `stage1_status` per `007`; merge delta for same `broad_filter_key_hash` |
| 5 Collectors | Integrate `005`; watermark read/write; cooldown/lock/refresh |
| 6 Retention | Scheduled + manual path per `retention-jobs.md` |
| 7 Quality | `make test`, `make vet`, `make fmt` |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | Lock / cooldown TTL | **600** s lock, **3600** s cooldown — **code constants** (see `contracts/redis-ingest-coordination.md`). |
| D2 | Redis down | **Fail closed** — no new ingest without working lock path. |
| D3 | Explicit refresh | Bypass **cooldown only**; **lock always** taken. Enable via `JOBHOUND_INGEST_EXPLICIT_REFRESH` and/or explicit workflow/activity flags (documented). Default **off**. |
| D4 | Watermark | Postgres table **`ingest_watermarks`** — **PK `(slot_id, source_id)`** (`contracts/ingest-watermark-and-filter-key.md`). |
| D5 | Broad filter key | Canonical JSON (includes **`slot_id`**, optional **`user_id`**) → **SHA-256** hex; store on **`pipeline_runs.broad_filter_key_hash`** when column added by **`006`** migration (after `007` creates `pipeline_runs`). |
| D9 | Slot vs global | **No** cross-slot reuse of stage-1 pools by hash alone; Redis lock remains **per `source_id`** only (product draft §3). |
| D6 | `jobs` stage-1 column | Name and enum per **`007`** `contracts/pipeline-run-job-status.md` — **no second definition** in `006`. |
| D7 | Description-only job update | **Ingest** updates the row; **does not** by itself reset or re-run **`004`/`007`** per-run stages for that job. |
| D8 | Retention | Cron **every 7 days UTC** + **manual** same delete logic; cutoff **`created_at` < now() UTC − 7 days**; CASCADE per `007`. |

## Engineering follow-ups (non-blocking)

- Optional **env overrides** for lock/cooldown TTLs via `internal/config` (keep defaults as now).  
- Heartbeat for long ingest if TTL proves too short in production.  
- Separate **`broad_query_keys`** table if analytics/UI needs key history.  
- **Schema migration** from **`source_id`-only watermarks** to **`(slot_id, source_id)`** if the repo shipped the pre–slot-scoped DDL — see **`tasks.md`** supplement.

## Project structure (documentation)

```text
specs/006-cache-and-ingest/
├── spec.md
├── plan.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── environment.md
    ├── redis-ingest-coordination.md
    ├── ingest-watermark-and-filter-key.md
    └── retention-jobs.md
```

## Source structure (anticipated — implementation phase)

```text
internal/config/                 # JOBHOUND_REDIS_URL, explicit refresh flag
internal/jobs/storage/           # upsert, stage1_status
internal/.../ingest/ or internal/pipeline/  # filter key hash, canonical JSON helper
migrations/                      # ingest_watermarks, pipeline_runs column
cmd/worker/                      # register ingest + retention workflows/activities
```
