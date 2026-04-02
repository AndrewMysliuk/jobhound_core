# Feature: Cache and ingest

**Feature Branch**: `006-cache-and-ingest`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Draft

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — search slots, stage-1 refresh vs filter resets, Redis keyed by `source_id`.

## Goal

**PostgreSQL** is the durable store for normalized vacancies: **upsert** by stable id, **no cross-source merging** (identity is **source + vacancy link** / stable id per `001`).

**Redis** coordinates ingest **per source**: **lock** (no overlapping ingest for the same source) and **cooldown** (minimum interval between successful ingests for the same source). No Redis-backed search-result cache in v1.

Define **when** ingest hits external collectors vs reads from Postgres, **incremental cursor** (when supported by collectors), **retention** for old job rows, and optional **explicit refresh** (default off).

**Search slot (MVP):** stage-1 ingest and the **stage-1 pool** are **scoped to a search slot** (`slot_id`), not a global “same broad query” cache across slots. Two slots may use the **same** broad keyword string; each still has its **own** pool and downstream rows — **no** requirement to dedupe or merge vacancies **across** slots (see `002` canonical `jobs` vs slot associations). The **normalized broad filter key** hash and **watermarks** are reconciled in **`(user_id, slot_id)`** scope (see **`contracts/ingest-watermark-and-filter-key.md`**). Narrow stage-2/3 filtering is **not** ingest (`004` / `007`).

## Non-goals

- Notification delivery and idempotency (other specs).
- Full public API contract (`010`).
- Event scheduler UI.

## Identity and deduplication

- **No** merging vacancies across different sources.
- Stable **`id`** and upsert semantics align with `001` (source + link).
- **Content unchanged** (skip unnecessary downstream work): compare **all persisted vacancy fields except `description`** (description is excluded due to size). **Do not** include `created_at` / `updated_at` in equality — they are row metadata only.
- If only **`description`** differs: still **update** the row. **Downstream (`004` / `007`)**: ingest **does not** by itself invalidate or reset per-run stage-2/3 state for that job; re-filtering after a description-only change is **out of scope** for this spec unless a later epic defines it.

## Broad query key and “cache”

- **“Same” broad stage-1 request** for bookkeeping is identified by a **normalized filter key**: canonical JSON (includes **`slot_id`** and reserved **`user_id`** when present — see **`contracts/ingest-watermark-and-filter-key.md`**) then **SHA-256** hex. The hash is stored per pipeline run (e.g. **`pipeline_runs.broad_filter_key_hash`**) once `007`’s **`pipeline_runs`** table exists.
- **Within one slot**, later runs with the **same** immutable broad parameters (per product draft §2–3) **merge new** vacancies into that slot’s pool via **delta** ingest + canonical **`jobs`** upsert — not a frozen snapshot from day one only. **Across slots**, there is **no** shared “global” row pool keyed only by keywords: identical strings in two slots still produce **separate** slot pools and association rows.
- **Stage 2+** (keywords like Vue vs React) filters **existing** rows in the slot’s stage-1 pool — that path is **`004` / `007`**, not a second full ingest in this spec.

## Incremental fetch (watermark)

- **Watermark** = **per `(slot_id, source_id)`** **cursor** stored in **PostgreSQL** (table **`ingest_watermarks`**) for “fetch only newer than X” when the collector supports it (`005`). Cursor value is **opaque** to this spec (`005` defines payload). Two slots hitting the **same** source **must not** share one cursor — each slot advances its own watermark.
- Until a collector exposes incremental semantics, ingest follows that collector’s **full-fetch** behavior; the watermark row may exist with **`cursor` unused** for that pair.

## Redis usage (v1, minimal)

- **`ingest:lock:{source_id}`**: lock so two concurrent ingests for the same source do not run together. **Default TTL: 600 seconds** (code constant). Acquire with **SET NX** (or equivalent).
- **`ingest:cooldown:{source_id}`**: set after a **successful** ingest; **default TTL: 3600 seconds** (code constant). Blocks a new ingest until expiry unless **explicit refresh** bypasses cooldown (see below).
- **Source id** in keys is **normalized** (trim, lowercase; stable slug per collector) — see **`contracts/redis-ingest-coordination.md`**.
- If Redis is unavailable: **fail closed** — **do not** start a new ingest for that source (log + error). No in-process single-flight fallback in v1.

## Explicit refresh

- Optional: **bypass cooldown** only; **lock is always taken** before ingest work.
- **Default: disabled** — enable via **`JOBHOUND_INGEST_EXPLICIT_REFRESH`** and/or an explicit workflow/activity flag (documented at call site).
- Does **not** bypass the lock.

## Retention

- **Cutoff:** hard-delete rows from **`jobs`** where **`created_at` < now() in UTC − 7 days**.
- **Automatic:** run on a **cron** schedule **once per 7 days** in **UTC** (exact minute implementation-defined).
- **Manual:** the same delete logic may be run by an operator (one-off activity, CLI, etc.).
- Because the job runs **at most** every 7 days, a row may persist **slightly longer than 7 wall-clock days** in the worst case — see **`contracts/retention-jobs.md`**.
- **DELETE** only — no soft-delete, no status-only “archived” state for this cleanup.
- **Cascade** (or delete in one transaction): remove **dependent** rows that reference `jobs` (e.g. **`pipeline_run_jobs`** from `007`) so **no dangling foreign keys** remain.

## Stage-1 status on `jobs`

- **`PASSED_STAGE_1`** and the **`jobs`** column name/type are **defined in `007`** — **`specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md`**. This spec only requires ingest to **set** that column when broad stage 1 completes.

## Indexes and queries

- Indexes supporting stage-1 style queries (role, time window, source) and retention (`jobs.created_at` if needed) as required for performance.

## Dependencies

- **`000`** ([`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)): slot-scoped stage-1 pool, Redis lock by `source_id`, immutable broad string per slot after first ingest.
- **`001`**: `domain.Job`, stable id rules.
- **`002`**: Postgres schema, migrations; canonical `jobs` vs slot associations.
- **`005`**: collectors; watermark wired when incremental behavior exists.
- **`007`**: `pipeline_runs` (minimal, **`slot_id`** per contract); **`PASSED_STAGE_1`** on `jobs`; per-run rows and **ON DELETE CASCADE** from `pipeline_run_jobs` to `jobs` — ingest and retention **must** align with those migrations/contracts.
- **`010`** (when implemented): HTTP API shapes for “pull new” / slot lifecycle — referenced for **refresh** UX, not required to freeze Redis/Postgres rules here.

## Local / Docker

- Postgres per existing Compose / docs.
- **Redis** required for ingest coordination in v1 (add to Compose when implementing this feature).

## Implementation artifacts

Plan, task backlog, and frozen contracts: [`plan.md`](./plan.md), [`tasks.md`](./tasks.md), [`contracts/`](./contracts/), quality checklist [`checklists/requirements.md`](./checklists/requirements.md).

## Acceptance criteria

1. Ingest uses **Redis** lock + cooldown with **default TTLs 600 s / 3600 s** and **fail closed** if Redis is unavailable.
2. **Explicit refresh** (when enabled) bypasses **cooldown only**; **lock** is always acquired; default is **off**.
3. **Watermark** persisted in **`ingest_watermarks`** **per `(slot_id, source_id)`**; **broad filter key** is **SHA-256** hex of canonical JSON (includes **`slot_id`**) per **`contracts/ingest-watermark-and-filter-key.md`**, stored per run (e.g. **`pipeline_runs.broad_filter_key_hash`**).
4. **`jobs`** receives **`PASSED_STAGE_1`** per **`007`** contract when broad stage 1 completes.
5. **Retention** deletes `jobs` older than **7 days** by **`created_at` (UTC)**; schedule **at most every 7 days UTC**; **manual** path uses the same semantics; dependent **`pipeline_run_jobs`** rows are removed (**CASCADE** or same transaction).
6. **Description-only** row update does **not** by itself reset **`004`/`007`** per-run outcomes for that job.
