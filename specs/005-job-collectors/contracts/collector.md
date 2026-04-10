# Contract: collectors (boundary + errors)

**Spec**: `005-job-collectors`  
**Status**: Draft

## Code layout (target)

- **`internal/collectors/`** — one package (or subtree) per source. Site-specific HTTP bodies, selectors, and JSON shapes live **here**, not in `internal/pipeline`.
- **`internal/collectors/utils`** — shared helpers: HTTP defaults (timeouts, user-agent), matching locations to **`data/countries.json`**, date parsing, URL normalization before `StableJobID`.

The type boundary is **`collectors.Collector`** in `internal/collectors/contract.go` (domain type: **`schema.Job`** from `internal/domain/schema`):

```go
type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]schema.Job, error)
}
```

- **`Name()`** — stable diagnostic label for logs/metrics.
- **`Fetch`** — one logical run for that source (pagination and request bodies are internal).

### `SlotSearchFetcher` (slot keyword at ingest)

Optional interface in the same package:

```go
type SlotSearchFetcher interface {
	FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error)
}
```

- **`006`** **`RunIngestSource`**: when **`IngestSourceInput.SlotSearchQuery`** is **non-empty** (trimmed), the activity calls **`FetchWithSlotSearch`** if the collector implements it; otherwise **`Fetch`**. When the query is **empty**, behavior is unchanged (**`Fetch`** / incremental per **`006`**).
- Implementations **must not** mutate shared collector fields in a racy way: use a **shallow copy** of the struct inside **`FetchWithSlotSearch`** then call **`Fetch`**, or equivalent.
- **`slotQuery` empty**: behave like **`Fetch`** (same listings as an unscoped run).

**MVP sources — how `slotQuery` maps to wire**:

| Source (`Job.Source`) | Behavior |
|-----------------------|----------|
| `europe_remotely` | AJAX form field **`search_keywords`** (see `resources/europe-remotely.md`). |
| `working_nomads` | Elasticsearch **`multi_match`** over title / description / tags / category (implementation-defined fields). |
| `dou_ua` | Vacancies **`search`** query param (**overrides** static config search for that run). |
| `himalayas` | Search API **`q`** (`UseSearch` + search endpoint). |
| `djinni` | Listing query **`all_keywords`** with **`search_type=full-text`** (see `resources/djinni.md`). |

## Relationship to orchestration (MVP)

**Collectors are source-scoped**, not slot-scoped: **`Fetch`** returns normalized **`[]schema.Job`** for **one board** in one run. **Search slots**, **`slot_id`**, **which sources are bound** to a slot, **upsert** into PostgreSQL, **watermarks / delta** behavior, and **Redis lock + cooldown per `source_id`** live in **`006`** (and HTTP/workflow contracts in **`008`** / **`009`**). The **slot display name** from **`009`** is passed as **`SlotSearchQuery`** into ingest so **`SlotSearchFetcher`** can scope fetches; **no requirement** to dedupe across slots inside a collector.

## `Job.Source` (normative string values)

Use a **fixed lowercase string** per board for `Job.Source` and for `StableJobID` (same value in DB `jobs.source` when persisted).

| Source            | `Job.Source` value   |
| ----------------- | -------------------- |
| Europe Remotely   | `europe_remotely`    |
| Working Nomads    | `working_nomads`     |
| DOU.ua            | `dou_ua`             |
| Himalayas         | `himalayas`          |
| Djinni            | `djinni`             |

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

**Exception (Europe Remotely, DOU.ua, and Himalayas, dates):** if a **soft-fail date** rule applies (Europe/DOU unparseable display; Himalayas missing or invalid **`pubDate`**), set **`PostedAt`** to zero, emit a **structured warning** (log) including the raw value where helpful, and **continue** that job if all other required fields are present. See **`domain-mapping-mvp.md`**.

Do not return `Job` rows with empty **`URL`** (listing page) when the contract requires it.

**Orchestration:** if the product runs multiple collectors in one pipeline, **failure of one collector must not prevent others from running** unless a higher-level workflow explicitly defines otherwise — collector `Fetch` errors are per source.

## Tests

Offline tests use **`httptest`** and bodies documented in **`contracts/test-fixtures.md`** (samples as fenced blocks). Copies may live under `internal/collectors/.../testdata/` when code exists. Checklist and definition-of-done: **`../tasks.md`**.

## Related

- `spec.md`
- [`specs/000-epic-overview/product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) — slots and stage-1 vs `006`
- `domain-mapping-mvp.md`
- `sources-inventory.md`, `../resources/europe-remotely.md`, `../resources/working-nomads.md`, `../resources/dou.md`, `../resources/himalayas.md`, `../resources/djinni.md`
- `test-fixtures.md`
