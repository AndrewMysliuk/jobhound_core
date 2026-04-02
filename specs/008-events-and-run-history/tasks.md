# Tasks: Scheduled auto-refresh and run history

**Input**: [spec.md](./spec.md), [contracts/scheduled-runs-and-history.md](./contracts/scheduled-runs-and-history.md), [plan.md](./plan.md).  
**Schema stubs**: optional carry-over from `specs/002-postgres-gorm-migrations` (plan D3) if still listed there—**table names and columns are owned by this epic’s contract**.  
**Depends on**: `003`, `004`, `006`, `007` (per spec); DB bootstrap from `002`.  
**Product order**: implement after core vertical (`009` / `010` + ingest + pipeline) per `specs/000-epic-overview/product-concept-draft.md` §8–§9.

## A. Contracts

1. [ ] **Schedule + run history schema contract** — Definition of done: [contracts/scheduled-runs-and-history.md](./contracts/scheduled-runs-and-history.md) reviewed; open questions in §5 resolved; aligns with `pipeline_runs.slot_id` and slot lifecycle (**draft** §2).

## B. Migrations (stub or full tables)

2. [ ] **SQL migration: run history** — Definition of done: versioned `up`/`down` under repo `migrations/` creates the history table (columns per contract §3); down is safe per team preference.

3. [ ] **SQL migration: slot schedule** — Definition of done: versioned `up`/`down` creates the schedule table (columns per contract §2); integrates with optional `temporal_schedule_id`.

## C. Storage layer

4. [ ] **GORM models + mapping** — Definition of done: packages under `internal/.../storage/`; `TableName()`, mapping helpers; `WithContext` on queries.

5. [ ] **Repository interfaces** — Definition of done: minimal ports for append history, read/update schedule by `slot_id`, list recent history; no duplicate “saved search” entity—parameters stay on the slot.

## D. Tests

6. [ ] **Integration: migrations** — Definition of done: same approach as `002` (`integration` build tag / Compose): migrate up, assert expected tables/columns exist.

7. [ ] **Unit: mapping** — Definition of done: table-driven tests for NULL/zero and round-trip for storage models introduced in C.4.

## E. Cross-cutting

- **Dedup / job store persistence** stays under **`006`**; run history rows record **tick outcomes**, not normalized job bodies, unless spec explicitly extends scope.
- **Temporal** schedule registration vs external trigger: document chosen approach in contract or `plan.md` D3 when implemented.

---

## Changelog (spec / artifacts only)

**2026-04-02** — Aligned [spec.md](./spec.md) with [`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md): slot-scoped schedules, post-core phasing, delta + cap references, no filter snapshot history. Added [contracts/scheduled-runs-and-history.md](./contracts/scheduled-runs-and-history.md), [plan.md](./plan.md), [checklists/requirements.md](./checklists/requirements.md). Task identifiers A–E unchanged; wording updated for **slot** centricity and contract path.
