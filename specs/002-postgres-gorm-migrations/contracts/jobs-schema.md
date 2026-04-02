# Contract: `jobs` table & domain mapping

**Feature**: `002-postgres-gorm-migrations`  
**Status**: **Frozen** for `002` — first migration (`migrations/000001_*_jobs.up.sql` or next free version) and storage mapping **must** match this document.  
**Domain type**: `internal/domain/job.go` → `Job`

## MVP: canonical row vs search slots

[`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) §2–3: a **search slot** has its own stage-1 pool and downstream results. **`jobs`** stores **canonical** listing facts **keyed only by stable `id`** (`001`); it does **not** carry **`slot_id`**. Membership of a vacancy in a slot’s pool is expressed through **slot/run-scoped** tables (coordinated with **`006` / `007` / `010`**—e.g. links from runs to `job_id`, future `slot_id` on run headers). The same canonical `id` may appear in **multiple** slots via **multiple association rows**, not multiple `jobs` PKs.

**`user_id`** here is optional listing-level attribution reserved for multi-user (`001`); **slot ownership** is **not** defined solely by this column.

## SQL table `jobs`

Logical columns (names and types are normative):

| Column | Type | Nullable | Notes |
|--------|------|----------|-------|
| `id` | `text` | NO | Primary key; stable job id (`Job.ID`) |
| `source` | `text` | NO | |
| `title` | `text` | NO | `DEFAULT ''` — domain uses plain `string`; empty string is stored as `''` |
| `company` | `text` | NO | same |
| `url` | `text` | NO | listing URL; same default as `title` |
| `apply_url` | `text` | YES | optional listing; see mapping |
| `description` | `text` | NO | same default as `title` |
| `posted_at` | `timestamptz` | YES | |
| `user_id` | `text` | YES | multi-user reserved; see mapping |
| `created_at` | `timestamptz` | NO | set on insert (DB default + GORM) |
| `updated_at` | `timestamptz` | NO | set on insert/update (GORM / app) |

**Indexes**: PK on `id` only in `002`. Further indexes deferred until ingest patterns in `006` (see `plan.md`).

### Canonical `up` DDL (first migration)

The initial `jobs` migration **must** create a table equivalent to the following (ordering and constraint names may differ; behavior and columns must match).

```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    apply_url TEXT,
    description TEXT NOT NULL DEFAULT '',
    posted_at TIMESTAMPTZ,
    user_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
```

- **Defaults**: `created_at` / `updated_at` may use `DEFAULT now()` in SQL if desired; GORM must still maintain `updated_at` on updates (Postgres has no built-in “auto-update this column” for arbitrary `TIMESTAMPTZ`).
- **Down**: `DROP TABLE IF EXISTS jobs;` (or equivalent safe downgrade).

## Domain ↔ storage mapping

| `domain.Job` | Storage / SQL | Rule |
|--------------|---------------|------|
| `ID` | `id` | direct |
| `Source` | `source` | direct |
| `Title` | `title` | direct; `""` ↔ `''` |
| `Company` | `company` | direct; `""` ↔ `''` |
| `URL` | `url` | direct; `""` ↔ `''` |
| `ApplyURL` | `apply_url` | **`""` ↔ SQL `NULL`**; non-empty string ↔ stored value |
| `Description` | `description` | direct; `""` ↔ `''` |
| `PostedAt` | `posted_at` | **zero `time.Time` ↔ `NULL`** (plan D7) |
| `UserID *string` | `user_id` | **`nil` or pointer to `""` ↔ `NULL`**; non-empty string ↔ stored value (plan D8) |

## GORM model location

- Struct and tags live only under **`internal/.../storage/...`**, not in `internal/domain`.

## Related

- `spec.md` — Initial schema (v0) and “Alignment with MVP”
- `product-concept-draft.md` — slots, pools, delete semantics for child data
- `plan.md` — D7, D8, D3 (`jobs` only in `002`)
