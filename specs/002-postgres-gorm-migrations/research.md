# Research Notes: PostgreSQL, GORM, migrations

**Branch**: `002-postgres-gorm-migrations`  
**Spec**: `specs/002-postgres-gorm-migrations/spec.md`  
**Date**: 2026-03-29

Inventory of **jobhound_core** today and external patterns referenced by the spec. Unknowns are marked **TBD** until implementation locks them.

---

## 1. Current repo state (`002` start)

| Area | Status |
|------|--------|
| `go.mod` | Go **1.24** only; **no** GORM, migrate, or Postgres driver yet |
| `cmd/agent` | No DB; noop pipeline only |
| `internal/domain/job.go` | `Job` with `ID`, `Source`, `Title`, `Company`, `URL`, `ApplyURL`, `Description`, `PostedAt`, `UserID *string` — aligns with spec “Initial schema” |
| Docker Compose | **Absent**; to be added per spec + `000-epic-overview` |
| Makefile | `build`, `run`, `test`, `fmt`, `vet`, `tidy` — no migrate or DB help text yet |

---

## 2. Style reference (omg-api) — patterns only

The spec asks for **omg-api-like** patterns **without** importing Omega code:

- **GormGetter**: `func() *gorm.DB` injected into storage constructors; queries use `dbGetter().WithContext(ctx)`.
- **golang-migrate**: SQL up/down files; CLI or embedded file source with `file://migrations` (or equivalent).
- **Storage package**: GORM model + `TableName()`, mapping funcs `ToDomain` / `NewModel` (naming may match repo conventions).

**Do not** add `github.com/omgbank/go-common` or vendor paths from omg-api.

---

## 3. golang-migrate (expected integration)

- Module: `github.com/golang-migrate/migrate/v4`
- Postgres driver: typically `_ "github.com/golang-migrate/migrate/v4/database/postgres"` + `"github.com/golang-migrate/migrate/v4/source/file"` (or `iofs` if embedding).
- **Idempotence**: re-running `up` when at latest version is normal migrate semantics (no-op at ceiling).

**TBD**: exact `cmd/migrate` vs `make` + installed `migrate` binary — see `plan.md` D4.

---

## 4. GORM + Postgres

- Expected deps: `gorm.io/gorm`, `gorm.io/driver/postgres`.
- **TranslateError**: optional `gorm.Config{ TranslateError: true }` for friendlier `errors.Is` with `gorm.Err*` (confirm against GORM v2 docs for the pinned version).
- **Pool**: `SetMaxOpenConns`, `SetMaxIdleConns`, `ConnMaxLifetime` from env or sane defaults in code — final numbers in implementation.

---

## 5. Domain ↔ SQL mapping edge cases

| Domain | SQL |
|--------|-----|
| `PostedAt` zero | `NULL` `timestamptz` |
| `UserID` nil or empty string (product rule TBD) | `NULL` vs store empty — **default**: nil → NULL; empty string → TBD in `jobs-schema.md` if needed |
| `ID` | PK **text**, stable job id from `001` |

---

## 6. Dependencies on other specs

- **`001`**: stable id assignment and field semantics for `Job`.
- **`000`**: local stack expectations (Postgres via Compose); Temporal may follow in `003` — **not required** for `002` completion if milestones match epic.

---

## 7. Out of scope (recap)

- Full event/run schema, HTTP API, GCP secrets runtime — later specs.
- CDC / Debezium.

---

## 8. References (paths)

- Spec: `specs/002-postgres-gorm-migrations/spec.md`
- Domain: `internal/domain/job.go`
- Constitution: `.specify/memory/constitution.md`
- Epic: `specs/000-epic-overview/spec.md`
