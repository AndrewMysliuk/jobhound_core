# Feature: Agent skeleton and domain model

**Feature Branch**: `001-agent-skeleton-and-domain`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Implemented

## Goal

Establish Go module layout, shared **domain types** (`domain.Job`, `domain.ScoredJob`), and **stable job identity** for dedup and history **within a single source** (`Source` + canonical listing URL). Align with the global MVP narrative: **search slots** and **`user_id`** are reserved in the product model; this epic defines the **normalized vacancy shape** and identity rules. **Slot association** (`slot_id`) lives at persistence and workflow boundaries (see **`002` / `006` / `007`**), not as a hard requirement on the domain struct unless a later epic adds an optional field for in-memory threading. No auth in this epic.

## Product context

This feature is the first implementation slice after `000-epic-overview`. It does not deliver ingest, persistence, Temporal, or public HTTP API by itself beyond composition roots that wire mocks.

- **Global product behavior** (slots, three stages, reset rules, stage-3 policy): [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md). If this epic ever disagrees with that draft on user-visible or data-lifecycle behavior, **update this spec** (or the draft first if the product decision changed).
- **Three stages** (same numbering as `000`): **(1)** broad external ingest into a **slot’s** pool; **(2)** local narrow filters on that pool only; **(3)** LLM on rows that passed stage 2. `domain.Job` is the normalized **listing** flowing out of collectors into stage 1; `domain.ScoredJob` is the post–stage-3 shape for downstream delivery specs.
- **Per draft §2**: MVP does **not** require deduplication **across** slots—each slot has its own stage-1 pool. **Stable job id** remains **(source, listing URL)** so the same logical vacancy on the same board always gets the **same** id string; storage may attach **different** `slot_id` rows for the same id across slots without collapsing them in MVP.

Engineering layout and testing policy: `.specify/memory/constitution.md` and `.cursor/rules/specify-rules.md`.

## Package layout & layers

Target layout (aligned with the constitution and Cursor rules — **not** the older `internal/ports` + `internal/app` sketch):

| Area | Responsibility |
|------|----------------|
| `cmd/agent` | Composition: pipeline deps, optional **dev-only** `debughttp` handler; **no** business rules |
| `cmd/worker` | Temporal worker composition (when used) |
| `internal/domain` | `Job`, `ScoredJob`, URL normalization, `StableJobID` (single source of truth for identity string) |
| `internal/collectors` | `Collector` contract, sources, `schema/`, `handlers/debughttp/` (dev-only HTTP) |
| `internal/pipeline` | `contract.go` (`Dedup`, `Notifier`, `PipelineRunRepository`, …), stage rule types, `impl/` orchestration, `utils/` stage implementations, `mock/` |
| `internal/llm` | Scorer contract, providers, `mock/` |
| `internal/jobs`, `internal/ingest`, … | Persistence and ingest modules as specified in later epics |
| `internal/platform/pgsql`, `internal/platform/temporalopts`, … | Shared infra, not product modules |

**Rule**: feature modules expose **`contract.go`** (or equivalent) at the module root; **orchestration** for the pipeline lives in **`internal/pipeline/impl`**, not in `cmd`. **`cmd/*`** stays thin.

## Stable job identity (v1)

**Definition**: A stable id identifies “this vacancy **as seen on this source**” (same listing URL on the same source → same id). **Cross-site duplicates** (same role on LinkedIn vs another board) are **out of scope** for v1; they would be different ids until a future dedup/cluster feature.

**Inputs to identity**:

- **`Source`**: non-empty logical source key (e.g. `djinni`, `himalayas`). Normalized for id computation: trim Unicode spaces, lowercase ASCII.
- **`ListingURL`**: canonical **job posting page** URL (where the vacancy is read), **not** the apply/ATS link. **Apply URL is excluded from the id formula** in v1 when listing URL is non-empty (apply links are often redirects/trackers and change more often).

**Fallback**: If the listing URL is missing but **`ApplyURL`** is present, v1 **MAY** use normalized `ApplyURL` as the URL input to `StableJobID` so collectors can still produce an id (document in collector specs when added).

**Output**: A single opaque string stored in `Job.ID`, produced only by **`domain.StableJobID`** (or equivalent) so all collectors share one rule.

### URL normalization (v1)

Before combining with `Source`, the listing URL string is canonicalized:

1. Trim surrounding space on the raw string.
2. Parse as an absolute URL (`http` or `https`). If parsing fails, identity computation **MUST** fail (return error); callers must not invent alternate ids silently.
3. Scheme and host **lowercase**; **fragment** (`#...`) **dropped**.
4. **Query string**: kept as-is in v1 (revisit when a collector shows noisy tracking params).
5. **Path**: remove a single **trailing slash** except for path `/` (so `/jobs/123` and `/jobs/123/` match).

**Delimiter**: Concatenate `sourceKey + "\x1e" + normalizedURLString` (record separator is unlikely inside source keys) or another fixed, documented separator; exact encoding is implementation-defined as long as it is **deterministic** and **documented in code comments**.

## User scenarios & testing

### User Story 1 — Project builds and tests pass (Priority: P1)

A developer clones the repo and verifies the module compiles and unit tests pass with only the Go toolchain.

**Why this priority**: Unblocks every later feature.

**Independent Test**: `make test` (or `go test ./...`) succeeds in CI/local.

**Acceptance scenarios**:

1. **Given** a clean checkout, **When** `go test ./...` runs, **Then** it exits zero.
2. **Given** the repo, **When** `make build` runs, **Then** `bin/agent` (and worker binary if present in Makefile) is produced.

---

### User Story 2 — Stable id is deterministic (Priority: P2)

Two runs with the same `Source` and semantically same listing URL produce the **same** `Job.ID`; trivial URL variants (case, trailing slash, fragment) collapse to one id.

**Why this priority**: Dedup and pipeline dedup hooks depend on a single stable key per source listing.

**Independent Test**: Table-driven tests on `domain` normalization + `StableJobID` without network or DB.

**Acceptance scenarios**:

1. **Given** `Source` and listing URL that differ only by host case or trailing slash or `#fragment`, **When** `StableJobID` is called, **Then** all yield the **same** id string.
2. **Given** different `Source` values with the same URL, **When** `StableJobID` is called, **Then** ids **differ**.
3. **Given** empty `Source` or unparseable URL, **When** `StableJobID` is called, **Then** it returns an error.

---

### User Story 3 — Forward-compatible user and slot model (Priority: P3)

The job model can carry an optional **user** identifier for future isolation. **Slot** ownership and **`slot_id`** on stored rows are defined in **`002` / `006` / `007`**; the domain type does **not** have to duplicate `slot_id` for MVP as long as workflows and repositories always pass scope explicitly.

**Why this priority**: Avoids painful migration when auth/multi-user and multi-slot UI land.

**Independent Test**: `UserID` is present and documented; zero value means “unset”; pipeline behavior without DB unchanged for unit tests.

**Acceptance scenarios**:

1. **Given** a `Job` with user scope unset, **When** pipeline runs with mocks, **Then** behavior does not require a real `user_id`.
2. **Given** the product draft §2, **When** implementing slot-scoped storage, **Then** engineers use **`slot_id`** (and future **`user_id`**) at persistence/workflow layers without contradicting stable id rules above.

---

### Edge cases

- **Same vacancy on two different sources**: two different stable ids in v1 (by design).
- **Same listing in two slots**: same stable **string** id; MVP may store **two** slot-associated rows (draft: no cross-slot dedup requirement).
- **Only apply URL, no listing URL**: use apply URL for id **only** as documented fallback; collectors should prefer listing URL when available.
- **Relative URLs from HTML**: collectors must resolve to absolute before calling `StableJobID`; domain accepts absolute URLs only.

## Requirements

### Functional requirements

- **FR-001**: The repository **MUST** follow the module layout in “Package layout & layers” (or a documented equivalent). Boundary interfaces for the pipeline **MUST** live in **`internal/pipeline/contract.go`** (and sibling module `contract.go` files), not ad hoc in `cmd`.
- **FR-002**: `domain.Job` **MUST** include at least: stable `ID` (filled via FR-004), `Source`, `Title`, `Company`, **listing** URL (`URL` — canonical job posting page; used for stable id before `ApplyURL` fallback), optional **`ApplyURL`** (empty if unknown), `Description`, `PostedAt` (zero = unknown), optional **`UserID`** (nil/empty = unset). Implementations **MAY** add further normalized fields required by collectors or persistence (**`005` / `007`**)—e.g. remote flag, country, salary text, tags, MVP position label, stage-1 status pointer—as long as FR-004 identity rules stay unchanged.
- **FR-003**: `domain.ScoredJob` **MUST** include the scored `Job`, integer `Score`, and string `Reason` (sufficient for stage 3 → notify shape until LLM policy specs extend it).
- **FR-004**: Stable identity **MUST** be computed by a **single** exported function in `internal/domain` (e.g. `StableJobID(source, listingURL string) (string, error)`) from **Source + normalized listing URL** per “Stable job identity (v1)”; **MUST NOT** embed apply URL in the formula when listing URL is non-empty.
- **FR-005**: URL normalization **MUST** follow “URL normalization (v1)”; behavior **MUST** be covered by unit tests.
- **FR-006**: `cmd/agent` **MUST** remain thin (wiring only); pipeline orchestration **MUST** live in **`internal/pipeline/impl`**.
- **FR-007**: Mock adapters **MAY** satisfy pipeline/LLM contracts for local runs; default **`go test ./...`** **MUST NOT** require Postgres, Temporal, or external HTTP unless tests are behind the **`integration`** build tag.

### Key entities

- **Job**: Normalized vacancy from a single source; carries listing and optional apply links; `ID` is the dedup/history key **for that source listing**. Product **slots** aggregate rows in PostgreSQL keyed by **`slot_id`** (see global draft and **`006`**).
- **ScoredJob**: Job after stage 3 with score and human-readable reason for downstream notification (full LLM metadata deferred to **`004` / `007`**).

## Out of scope

- Real collectors beyond what later epics specify, full GORM/Postgres schema ( **`002`** ), Temporal workflows ( **`003`** ), public HTTP API (**`010`**).
- **Cross-site / fuzzy duplicate detection** and a single global canonical vacancy across sources.
- Auth and user lifecycle; only optional field reservation on `Job` here.
- **Slot CRUD and reset semantics** — narrative in product draft §5; technical contracts in **`009` / `010`** and storage epics.

## Dependencies

- **`000-epic-overview`** (index and stack).
- **`product-concept-draft.md`** for MVP boundaries and slot/stage vocabulary.

## Local / Docker

- Go toolchain only; no Compose services required for this feature’s **unit** tests.

## Success criteria

### Measurable outcomes

- **SC-001**: `go test ./...` passes in a clean environment without network (excluding `integration`-tagged tests).
- **SC-002**: Documented normalization cases in tests demonstrate **one** id per equivalent listing URL per source.
- **SC-003**: `specs/000-epic-overview` feature index refers to **`domain.Job`** (not ad hoc `model.Job`), and numbered epics use the same **stage** and **slot** language as the product draft where they touch this domain.

## Related

- [`specs/000-epic-overview/spec.md`](../000-epic-overview/spec.md) — feature index and order.
- [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — global MVP narrative (slots, stages, reset, stage-3 policy).
- `.specify/memory/constitution.md` — engineering principles and internal layout.
