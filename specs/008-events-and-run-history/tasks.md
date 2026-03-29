# Tasks: Scheduled events and run history

**Input**: `spec.md`; schema stubs **deferred** from `specs/002-postgres-gorm-migrations` (plan D3, optional “runs/events” in `002/spec.md`).  
**Depends on**: `003`, `006`, `007` (per `008/spec.md`); DB bootstrap and migrate path from `002`.

## A. Contracts

1. [ ] **Event + run history schema contracts** — Definition of done: `contracts/events-schema.md` and/or `contracts/run-history-schema.md` (or one combined doc) describe tables, PKs, nullability, and how they relate to Temporal workflow/run IDs and incremental watermarks.

## B. Migrations (stub tables carried from `002`)

2. [ ] **SQL migration: run history stub** — Definition of done: versioned `up`/`down` under repo `migrations/` creates a minimal table (names/columns owned here; align with `002` optional sketch: id, timestamps, status, nullable `temporal_workflow_run_id`, nullable `payload` jsonb or equivalent agreed in contract). Down drops or safe downgrade per team preference.

3. [ ] **SQL migration: scheduled event stub** — Definition of done: minimal table for saved search + schedule metadata needed before full `011` API; exact columns fixed in contract A.1. Integrates with watermark / “last successful run” fields from `008` goal.

## C. Storage layer

4. [ ] **GORM models + mapping** — Definition of done: packages under `internal/.../storage/` (no GORM in `internal/domain`); `TableName()`, `NewModel` / `ToDomain` (or symmetric) for event and run-history entities; `WithContext` on queries.

5. [ ] **Repository interfaces** — Definition of done: minimal ports for append run history, read last watermark / last run time, CRUD or subset for events as needed by `003` worker and `006` ingest alignment.

## D. Tests

6. [ ] **Integration: migrations** — Definition of done: same approach as `002` (`integration` build tag / Compose): migrate up, assert expected tables/columns exist.

7. [ ] **Unit: mapping** — Definition of done: table-driven tests for NULL/zero and round-trip for storage models introduced in C.4.

## E. Cross-cutting (not duplicated in `006`)

- **Dedup / job store persistence** stays under **`006`** per `002/tasks.md` task 16; do not conflate with run-history rows unless spec explicitly merges concerns.
