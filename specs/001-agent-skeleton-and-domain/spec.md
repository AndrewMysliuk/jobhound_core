# Feature: Agent skeleton and domain model

**Feature Branch**: `001-agent-skeleton-and-domain`  
**Created**: 2026-03-29  
**Last Updated**: 2026-03-29  
**Status**: Implemented

## Goal

Establish Go module layout, shared **domain types** (`domain.Job`, `domain.ScoredJob`), and **stable job identity** for dedup and history **within a single source** (`Source` + canonical listing URL). Reserve optional **user scope** fields for a future multi-user product. No auth in this feature.

## Product context

This feature is the first implementation slice after `000-epic-overview`. It does not deliver ingest, persistence, Temporal, or HTTP API beyond wiring stubs. See `.specify/memory/constitution.md` for stack and pipeline principles.

## Package layout & layers

Target layout (aligned with Cursor rules):

| Area | Responsibility |
|------|----------------|
| `cmd/agent` | Compose `app.Pipeline` from concrete adapters; no business rules |
| `internal/domain` | `Job`, `ScoredJob`, URL normalization, `StableJobID` (single source of truth for identity string) |
| `internal/ports` | `Collector`, `Filter`, `Scorer`, `Dedup`, `Notifier`, `SessionProvider` |
| `internal/app` | Orchestration only (e.g. one pipeline pass); **no** service-to-service calls |
| `internal/adapters/...` | Noop and future real implementations (sources, Telegram, GORM) |

**Rule**: domain “services” introduced in later features take **repository ports only**, not other domain services, to avoid import cycles. Cross-cutting composition stays in `cmd` or `app`.

## Stable job identity (v1)

**Definition**: A stable id identifies “this vacancy **as seen on this source**” (same listing URL on the same source → same id). **Cross-site duplicates** (same role on LinkedIn vs another board) are **out of scope** for v1; they would be different ids until a future dedup/cluster feature.

**Inputs to identity**:

- **`Source`**: non-empty logical source key (e.g. `djinni`, `himalayas`). Normalized for id computation: trim Unicode spaces, lowercase ASCII.
- **`ListingURL`**: canonical **job posting page** URL (where the vacancy is read), **not** the apply/ATS link. **Apply URL is excluded from the id formula** in v1 (apply links are often redirects/trackers and change more often).

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
2. **Given** the repo, **When** `make build` runs, **Then** `bin/agent` is produced.

---

### User Story 2 — Stable id is deterministic (Priority: P2)

Two runs with the same `Source` and semantically same listing URL produce the **same** `Job.ID`; trivial URL variants (case, trailing slash, fragment) collapse to one id.

**Why this priority**: Dedup and `Dedup` port depend on a single stable key per source listing.

**Independent Test**: Table-driven tests on `domain` normalization + `StableJobID` without network or DB.

**Acceptance scenarios**:

1. **Given** `Source` and listing URL that differ only by host case or trailing slash or `#fragment`, **When** `StableJobID` is called, **Then** all yield the **same** id string.
2. **Given** different `Source` values with the same URL, **When** `StableJobID` is called, **Then** ids **differ**.
3. **Given** empty `Source` or unparseable URL, **When** `StableJobID` is called, **Then** it returns an error.

---

### User Story 3 — Forward-compatible user scope (Priority: P3)

The job model can carry an optional **user** (or tenant) identifier for future isolation, unused in single-user runs.

**Why this priority**: Avoids painful migration when auth/multi-user arrives.

**Independent Test**: Type is present and documented; zero value means “unset”.

**Acceptance scenarios**:

1. **Given** a `Job` with user scope unset, **When** pipeline noop runs, **Then** behavior is unchanged vs no field.

---

### Edge cases

- **Same vacancy on two different sources**: two different stable ids in v1 (by design).
- **Only apply URL, no listing URL**: use apply URL for id **only** as documented fallback; collectors should prefer listing URL when available.
- **Relative URLs from HTML**: collectors must resolve to absolute before calling `StableJobID`; domain accepts absolute URLs only.

## Requirements

### Functional requirements

- **FR-001**: The repository **MUST** expose the package layout described in “Package layout & layers” (or a documented equivalent); boundary interfaces **MUST** live in `internal/ports`.
- **FR-002**: `domain.Job` **MUST** include at least: stable `ID` (filled via FR-004), `Source`, `Title`, `Company`, **listing** URL (`URL` or renamed `ListingURL` — document field name in code), optional **`ApplyURL`** (empty if unknown), `Description`, `PostedAt` (zero = unknown), optional **`UserID`** (nil/empty = unset).
- **FR-003**: `domain.ScoredJob` **MUST** include the scored `Job`, integer `Score`, and string `Reason` (sufficient for stage 3 → notify shape until LLM policy specs extend it).
- **FR-004**: Stable identity **MUST** be computed by a **single** exported function in `internal/domain` (e.g. `StableJobID(source, listingURL string) (string, error)`) from **Source + normalized listing URL** per “Stable job identity (v1)”; **MUST NOT** embed apply URL in the formula when listing URL is non-empty.
- **FR-005**: URL normalization **MUST** follow “URL normalization (v1)”; behavior **MUST** be covered by unit tests.
- **FR-006**: `cmd/agent` **MUST** remain thin (wiring only); pipeline orchestration **MUST** live in `internal/app`.
- **FR-007**: Noop adapters **MAY** satisfy `ports` for local runs; **MUST NOT** require Postgres, Temporal, or external HTTP for default tests.

### Key entities

- **Job**: Normalized vacancy from a single source; carries listing and optional apply links; `ID` is the dedup/history key **for that source listing**.
- **ScoredJob**: Job after stage 3 with score and human-readable reason for downstream notification (full LLM metadata deferred).

## Out of scope

- Real collectors, production Telegram, GORM/Postgres, Temporal workflows, public HTTP API.
- **Cross-site / fuzzy duplicate detection** and shared “canonical vacancy” across sources.
- Auth and user lifecycle; only optional field reservation on `Job`.

## Dependencies

- None (first implementation slice after `000`).

## Local / Docker

- Go toolchain only; no Compose services required for this feature’s tests.

## Success criteria

### Measurable outcomes

- **SC-001**: `go test ./...` passes in a clean environment without network.
- **SC-002**: Documented normalization cases in tests demonstrate **one** id per equivalent listing URL per source.
- **SC-003**: `specs/000-epic-overview` feature index refers to **`domain.Job`** (not `model.Job`).

## Related

- `specs/000-epic-overview/spec.md` — feature index and order.
- `.specify/memory/constitution.md` — product principles.
