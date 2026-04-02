# Contract: collectors (boundary + errors)

**Spec**: `005-job-collectors`  
**Status**: Draft

## Code layout (target)

- **`internal/collectors/`** — one package (or subtree) per source. Site-specific HTTP bodies, selectors, and JSON shapes live **here**, not in `internal/pipeline`.
- **`internal/collectors/utils`** — shared helpers: HTTP defaults (timeouts, user-agent), matching locations to **`data/countries.json`**, date parsing, URL normalization before `StableJobID`.

The type boundary today is **`pipeline.Collector`** in `internal/pipeline/contract.go`:

```go
type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]domain.Job, error)
}
```

- **`Name()`** — stable diagnostic label for logs/metrics.
- **`Fetch`** — one logical run for that source (pagination and request bodies are internal).

## Relationship to orchestration (MVP)

**Collectors are source-scoped**, not slot-scoped: **`Fetch`** returns normalized **`[]domain.Job`** for **one board** in one run. **Search slots**, **`slot_id`**, the **stage-1 broad keyword string**, **which sources are bound** to a slot, **upsert** into PostgreSQL, **watermarks / delta** behavior, and **Redis lock + cooldown per `source_id`** live in **`006`** (and HTTP/workflow contracts in later epics). The same **`Job.Source`** identity is used whether one user has one slot or many; **no requirement** to dedupe across slots inside a collector.

## `Job.Source` (normative string values)

Use a **fixed lowercase string** per board for `Job.Source` and for `StableJobID` (same value in DB `jobs.source` when persisted).

| Source            | `Job.Source` value   |
| ----------------- | -------------------- |
| Europe Remotely   | `europe_remotely`    |
| Working Nomads    | `working_nomads`     |

Implementation may wrap these in a Go typed const block; the **string value** above is what matters for identity.

## HTTP behavior

- **No retries**: each HTTP call is attempted once; failure returns error (subject to context cancellation).
- Reasonable **timeouts** and a normal **User-Agent** are required (defaults may live in `internal/collectors/utils` or config).

## Errors and batch semantics

| Kind | When | Notes |
| ---- | ---- | ----- |
| HTTP / timeout | transport, context deadline | wrap `ctx.Err()` or network error |
| HTTP status | non-2xx, unusable body | include status + short body snippet |
| Decode / parse | bad JSON, envelope missing required fields | explicit in message |
| Per-job required data | missing **listing `URL`**, card/hit cannot be parsed to contract, required DOM/JSON missing | **abort entire `Fetch`** with error — do not return a partial slice for that run |

**Exception (Europe Remotely only, dates):** if **`posted_display`** (listing or detail) cannot be parsed to an absolute time, set **`PostedAt`** to zero, emit a **structured warning** (log) including the raw string, and **continue** that job if all other required fields are present. See **`domain-mapping-mvp.md`**.

Do not return `Job` rows with empty **`URL`** (listing page) when the contract requires it.

**Orchestration:** if the product runs multiple collectors in one pipeline, **failure of one collector must not prevent others from running** unless a higher-level workflow explicitly defines otherwise — collector `Fetch` errors are per source.

## Tests

Offline tests use **`httptest`** and bodies documented in **`contracts/test-fixtures.md`** (samples as fenced blocks). Copies may live under `internal/collectors/.../testdata/` when code exists. Checklist and definition-of-done: **`../tasks.md`**.

## Related

- `spec.md`
- [`specs/000-epic-overview/product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) — slots and stage-1 vs `006`
- `domain-mapping-mvp.md`
- `sources-inventory.md`, `../resources/europe-remotely.md`, `../resources/working-nomads.md`
- `test-fixtures.md`
