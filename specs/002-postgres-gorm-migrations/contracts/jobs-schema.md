# Contract: `jobs` table & domain mapping

**Feature**: `002-postgres-gorm-migrations`  
**Domain type**: `internal/domain/job.go` → `Job`

## SQL table `jobs`

Logical columns (migration is source of truth; names here must match SQL):

| Column | Type | Nullable | Notes |
|--------|------|----------|-------|
| `id` | `text` | NO | Primary key; stable job id (`Job.ID`) |
| `source` | `text` | NO | |
| `title` | `text` | NO | use `''` default if domain allows empty string |
| `company` | `text` | NO | same |
| `url` | `text` | NO | listing URL |
| `apply_url` | `text` | YES | |
| `description` | `text` | NO | same default note as title |
| `posted_at` | `timestamptz` | YES | |
| `user_id` | `text` | YES | multi-user reserved |
| `created_at` | `timestamptz` | NO | set on insert |
| `updated_at` | `timestamptz` | NO | updated on write |

**Indexes**: minimum PK on `id`. Additional indexes (`source`, `posted_at`, …) optional in `002`; document in migration comments if added.

**TBD in migration file**: whether empty-string text fields use `DEFAULT ''` or `NOT NULL` without default — must be consistent with GORM zero values and domain.

## Domain ↔ storage mapping

| `domain.Job` | Storage / SQL |
|--------------|----------------|
| `ID` | `id` |
| `Source` | `source` |
| `Title` | `title` |
| `Company` | `company` |
| `URL` | `url` |
| `ApplyURL` | `apply_url` (empty string ↔ NULL **or** store `''`; pick one and test) |
| `Description` | `description` |
| `PostedAt` | `posted_at`: zero `time.Time` ↔ **NULL** |
| `UserID *string` | `user_id`: `nil` ↔ **NULL** |

## GORM model location

- Struct and tags live only under **`internal/.../storage/...`**, not in `internal/domain`.

## Related

- `spec.md` — Initial schema (v0)
- `plan.md` — Resolved decisions D7, D8
