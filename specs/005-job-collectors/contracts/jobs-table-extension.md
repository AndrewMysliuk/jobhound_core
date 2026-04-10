# Contract: `jobs` table extension (collector fields)

**Spec**: `005-job-collectors`  
**Status**: Draft

**Supplements** `specs/002-postgres-gorm-migrations/contracts/jobs-schema.md` when **`domain.Job`** gains salary/tags/position/timezone offsets (see **`domain-mapping-mvp.md`**). Apply via a **new numbered migration** in `migrations/` at implementation time (not part of this markdown-only spec deliverable).

## Columns to add

| Column | Type | Nullable | Default | Maps from `domain.Job` |
| ------ | ---- | -------- | ------- | ---------------------- |
| `salary_raw` | text | NO | `''` | `SalaryRaw` |
| `tags` | text | NO | `'[]'` | `Tags` as JSON array of strings |
| `position` | text | **YES** | SQL `NULL` | `Position` — **`NULL`** when pointer is **nil**; otherwise the string value |
| `timezone_offsets` | text | NO | `'[]'` | `TimezoneOffsets` as JSON array of numbers (floats), e.g. `[5.5,8]`; empty domain slice ↔ **`[]`** |

## Rules

- Empty `Tags` in domain ↔ JSON **`[]`** in SQL.
- Empty **`TimezoneOffsets`** in domain ↔ JSON **`[]`** in **`timezone_offsets`**.
- **`position`**: `domain.Job.Position == nil` ↔ **`NULL`**; non-nil pointer ↔ column text.
- GORM mapping lives under **`internal/jobs/storage`** alongside the existing `Job` model.

## Related

- `002` `jobs-schema.md` — base table
- `domain-mapping-mvp.md` — field provenance
