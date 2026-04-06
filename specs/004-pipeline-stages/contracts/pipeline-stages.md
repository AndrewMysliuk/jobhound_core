# Contract: pipeline stages (domain logic)

**Feature**: `004-pipeline-stages`  
**Purpose**: Freeze **behaviour** and **boundary types** for **implementation** stage 1 (broad filter), stage 2 (keywords), and stage 3 (LLM scoring). **Orchestration** (Temporal, HTTP handlers) lives outside these packages.

## Mapping to product stages (`product-concept-draft.md`)

| Product (draft) | Implementation in this contract |
|-----------------|----------------------------------|
| **Stage 1** — external ingest, broad keyword string, persist pool | **Out of scope** here; **`006` / `005`**. Callers supply `[]domain.Job` from the slot’s stage-1 pool. |
| **Stage 2** — narrow on **stored** rows only (include/exclude; optional date TBD) | **`ApplyBroadFilter`** then **`ApplyKeywordFilter`** in that order. Optional date/window rules in broad filter align with draft “optional date TBD” for stage 2; role synonyms / remote / country are **additional** narrow dimensions on the same pool. |
| **Stage 3** — LLM on rows that passed stage 2 | **`Scorer` / `Score`** with minimum score + rationale. **Per-run cap**, **deterministic ordering** of who enters the cap, **eligible pool**, and **Temporal idempotency** for batch runs are **not** fully specified here — see **`007`** and draft §4. |

## Shared principles

| Topic | Contract |
|-------|----------|
| Temporal | **No** `go.temporal.io` imports in stage implementation packages. |
| Filter rejection | Jobs that **fail** filter rules are **omitted** from the filtered list — **not** represented as `error` from the filter function. |
| Execution errors | Invalid configuration, LLM transport errors, **unparseable** LLM JSON → **`error`** from the relevant function; **callers** log and decide abort vs skip (see Stage 3). |
| Clock | Stage 1 uses **UTC** for “now” and window bounds; tests use an **injectable** clock (`time.Now` func or small `Clock` interface). |
| Text case | **Case-insensitive** matching for English-centric listings: **`strings.EqualFold`** or equivalent on the relevant field substrings unless implementation documents a different normalizer. |

## `domain.Job` fields (stage-relevant)

The following logical fields are **required** for spec compliance (exact Go field names follow implementation; migrate if persisted):

| Field | Meaning |
|-------|---------|
| `Title`, `Description` | Used in stages 1–2 matching. |
| `PostedAt` | `time.Time`; **zero value** means “unknown”. For stage 1, when a date window applies (explicit or default 7-day), **unknown posting date → job does not pass** (dropped), consistent with “narrowing” behaviour. |
| `Remote *bool` | **nil/unknown** → cannot satisfy “remote only” → **reject** when remote-only rule is on. **true** → remote. **false** → not remote. |
| `CountryCode string` | **Empty** → unknown location for country filter → **reject** when allowlist is non-empty. **Non-empty** → ISO 3166-1 alpha-2, compared case-insensitively to allowlist. |

## Stage 1 — broad filter

**Input**: `[]domain.Job`, **`BroadFilterRules`** (type in `internal/pipeline`; implementation `internal/pipeline/utils`):

| Rule | Semantics |
|------|-----------|
| `From`, `To` | Optional `*time.Time` (compared in **UTC**). If **both** unset → window is **`now−168h` .. `now`** (7 days) in UTC using an injectable clock. If **both** set → `PostedAt` must lie in the **closed interval** `[From, To]` in UTC (inclusive endpoints). Exactly one of `From`/`To` set → **invalid rules** (`ValidateBroadFilterRules` / `ApplyBroadFilter` returns `error`). |
| `RoleSynonyms` | **Empty** → no role-based narrowing. **Non-empty** → at least **one** non-empty synonym must appear as a substring in **`Title` or `Description`** (case-insensitive). |
| `RemoteOnly` | If true: keep only jobs with remote **known true**; unknown or false → drop. |
| `CountryAllowlist` | Empty → **no** country filter. Non-empty → keep only if country **known** and in list. |

**API**: `pipeline/utils.ApplyBroadFilter` — `ApplyBroadFilter(clock func() time.Time, rules BroadFilterRules, jobs []domain.Job) ([]domain.Job, error)`; `clock` may be nil → `time.Now`. **Output**: Subset of input jobs (order **preserved**).

## Stage 2 — keywords

**Input**: `[]domain.Job`, **`KeywordRules`**:

| Rule | Semantics |
|------|-----------|
| `Include` | Optional string slice. If **non-empty**, **at least one** pattern must appear somewhere in **`Title` or `Description`** combined text (case-insensitive). If **empty**, no include constraint. |
| `Exclude` | Optional string slice. If **non-empty**, **any** pattern appearing in that combined text → job **dropped**. |

**Order of evaluation**: Apply **include** constraints first, then **exclude** (or document equivalent if both are checked in one pass — must match tests).

**Output**: Subset of input jobs.

## Stage 3 — LLM scoring

**Input**:

- **Profile text**: single string (user CV / preferences summary).
- **Job**: `domain.Job` (and/or pruned struct) with fields the prompt needs.

**Provider interface** (illustrative):

```go
// Illustrative — actual interface is internal/llm.Scorer (contract.go)
type Scorer interface {
    Score(ctx context.Context, profile string, job domain.Job) (domain.ScoredJob, error)
}
```

**Output** (minimum):

| Field | Type | Required |
|-------|------|----------|
| `Score` | numeric (e.g. `int` 0–100) | Yes |
| `Reason` / `Rationale` | short string | Yes |

Map into **`domain.ScoredJob`** or extend it if extra fields are added (flags, etc.).

**Errors**: Return **`error`** for network failure, non-2xx, empty response, or JSON that does not match the agreed schema.

**Policy**: Whether one failed score aborts the whole batch is **not** fixed here — the **batch scorer** API should document whether it stops on first error or continues; default recommendation: **return error** to caller for single-job failure if using `Score` per job.

## JSON shape (LLM response)

Minimum stable JSON for parsing (exact field names frozen at implementation):

```json
{
  "score": 0,
  "rationale": "short text"
}
```

Optional keys may be added later; parser should **reject** missing required keys.

## Versioning

- Breaking changes to rules structs or JSON contract require updating this file, `plan.md`, and tests.

## Related

- `specs/000-epic-overview/product-concept-draft.md` — product numbering, reset §5, stage-3 policy §4.
- `specs/007-llm-policy-and-caps/` — batch caps, ordering, eligible pool, idempotency.

## Change process

1. Update this contract and `tasks.md`.  
2. Update `domain.Job` / migrations if persisted fields change.  
3. Update unit tests and any README “pipeline behaviour” section.
