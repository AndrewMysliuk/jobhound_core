# Research Notes: Pipeline stage services

**Branch**: `004-pipeline-stages`  
**Spec**: `specs/004-pipeline-stages/spec.md`  
**Date**: 2026-03-30

Inventory of **jobhound_core** pipeline and domain state, semantics choices, and testing strategy. Unknowns marked **TBD** until locked in `plan.md` Resolved decisions or `contracts/pipeline-stages.md`.

---

## 1. Current repo state (`004` start)

| Area | Status |
|------|--------|
| `internal/domain/job.go` | `Job` has `Title`, `Description`, `PostedAt`, etc.; **`ScoredJob`** (`Score`, `Reason`) in same package; **no** `Remote` / `Country` fields yet — **extend** per spec + contract |
| `internal/pipeline/contract.go` | `Filter`, `Scorer` interfaces; **narrowing** implementation TBD |
| `internal/pipeline/impl` | Pipeline glue exists; will call new stage functions |
| `internal/config/anthropic.go` | `JOBHOUND_ANTHROPIC_API_KEY` loader present |
| Temporal | `003` — stages **must not** import SDK |

---

## 2. Stage semantics (from spec)

### Stage 1 — broad filter

- **Date**: `PostedAt` within window; explicit `from`/`to` **or** default rolling **7 days** UTC.
- **Role synonyms**: substring (or agreed matching) in `Title` **and** `Description`, case-insensitive.
- **Remote**: when rule demands remote-only, pass only if job **explicitly** remote per domain field; **unknown** → reject.
- **Countries**: empty allowlist → any; non-empty → pass only if country **known** and listed; **unknown** → reject.

### Stage 2 — keywords

- Fields: `Title`, `Description`.
- **Include** list optional; **exclude** list optional.
- **Default semantics** (spec): if include non-empty, **all** includes must match; if exclude non-empty, **any** exclude term present → reject (exact phrasing locked in contract).

### Stage 3 — LLM

- Inputs: **profile text** (single block) + job fields needed for scoring.
- Output: **numeric score** + **short rationale** minimum; JSON shape TBD until prompt/schema fixed.
- **Abstraction**: interface + mock; real Anthropic client later.

---

## 3. Testing strategy

- **Unit tests** next to implementation; same package.
- **No real HTTP** in default tests: `httptest` only if testing a thin transport later; for `004`, prefer **interface mock** for LLM.
- **Edge cases**: zero `PostedAt`, empty lists, remote unknown, country unknown, all keywords optional empty.

---

## 4. Dependencies on other specs

| Spec | Relationship |
|------|--------------|
| `001` | `domain.Job`, `ScoredJob`, stable ID |
| `002` | Migrations if new `Job` fields persist |
| `003` | Worker may call stages from activities — **not** part of `004` deliverable |
| `005` | Collectors populate dates/geo — **out of scope** for stage logic |

---

## 5. Out of scope (recap)

- HTTP, Telegram, DB **inside** stage functions.
- Parsing “5 days ago” text — `005`.
- Per-vacancy status tracking for filter drops.

---

## 6. References (paths)

- Spec: `specs/004-pipeline-stages/spec.md`
- Epic: `specs/000-epic-overview/spec.md`
- Constitution: `.specify/memory/constitution.md`
- Config: `internal/config/anthropic.go`
