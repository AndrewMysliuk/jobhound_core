# Contract: Watermark & broad filter key

**Feature**: `006-cache-and-ingest`  
**Purpose**: Freeze **Postgres** shapes for **slot-scoped** **watermark** and the **normalized broad filter key** used for pipeline bookkeeping (aligned with [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) §2–3, §10).

**Related**: `001` stable job id; `005` collectors; `007` `pipeline_runs` (minimal table) — **`006`** may **extend** `pipeline_runs` with a nullable hash column via migration (after `007` creates the table).

---

## 1. Watermark (incremental fetch)

**Storage**: **PostgreSQL only** — not Redis.

### Table: `ingest_watermarks` (normative name unless repo-wide rename with spec update)

| Column      | Type        | Notes |
|-------------|-------------|--------|
| `slot_id`   | `UUID` **NOT NULL** | Search slot this cursor belongs to; same source in **two** slots → **two** rows. |
| `source_id` | `TEXT` **NOT NULL** | Normalized same way as Redis `source_id` in `redis-ingest-coordination.md`. |
| `cursor`    | `TEXT`      | **Opaque** string from the collector (`005`) when incremental mode is supported; **NULL** or empty when unused. |
| `updated_at`| `TIMESTAMPTZ` **NOT NULL** | Last write. |

**Primary key**: **`(slot_id, source_id)`**.

**Semantics**: If a collector does not support incremental fetch, the row may exist with **`cursor` unused**; ingest uses full-fetch for that **(slot, source)** pair.

**Transition note**: Older deployments that implemented **`source_id` only** as PK require a **follow-up migration** (see `006` **`tasks.md`** supplement) to add **`slot_id`** and retarget PK — spec **normative** shape is the composite above.

---

## 2. Broad filter key (same stage-1 search **within a slot**)

**Equivalence**: Two broad stage-1 requests are “the same” for hash purposes iff they produce the **same** canonical payload and thus the **same** hash — **including** the same **`slot_id`** (and **`user_id`** when set). Different slots **always** differ in canonical JSON, even if keywords and sources are identical.

### Canonical JSON (v1)

Build a JSON object with **fixed key order** (as listed), **case-insensitive** normalization for string values (compare lowercased Unicode for text fields), **sorted** unique arrays where applicable:

| Field           | Type / notes |
|-----------------|--------------|
| `slot_id`       | string — UUID canonical form (lowercase hex with hyphens) |
| `user_id`       | optional string, trimmed; **omit** if `NULL` / empty (single-tenant MVP) |
| `role`          | string, trimmed, lowercased |
| `time_window`   | object or string as defined by product — must be stable (e.g. `{ "from": "ISO8601", "to": "ISO8601" }` UTC) |
| `sources`       | array of source ids, **sorted** |
| `keywords`      | array of top-level broad keywords, **sorted**, trimmed, lowercased |

Omit empty optional sections consistently (document in code).

Serialize to a **single canonical UTF-8 string** (no insignificant whitespace), then compute:

**Hash**: **SHA-256** over that UTF-8 string, encode as **lowercase hex** (64 characters). This is **`broad_filter_key_hash`**.

No separate `broad_query_keys` lookup table is required for v1 unless a later epic needs analytics; the hash may be stored on **`pipeline_runs`** (see §3).

---

## 3. Where `broad_filter_key_hash` lives

**Preferred**: nullable column **`broad_filter_key_hash`** (`TEXT`, hex) on **`pipeline_runs`**, added in a **`006`** migration **after** `007` creates `pipeline_runs`. Each pipeline run for a broad search stores the hash for that run’s filter.

**Alternative** (only if `pipeline_runs` must not be altered): a small table `pipeline_run_filter_keys(pipeline_run_id PK/FK, broad_filter_key_hash TEXT NOT NULL)` — same hash rules. Prefer the single column on `pipeline_runs` for fewer joins.

---

## 4. Stage-1 status on `jobs`

Column name and allowed values for **`PASSED_STAGE_1`** are **owned by `007`** — see `specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md` (e.g. `stage1_status`). **`006`** ingest sets this when broad stage 1 completes for that vacancy row.
