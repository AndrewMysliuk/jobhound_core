# jobhound_core Constitution

## Product

Personal job aggregator on Go: ingest vacancies from several sources, narrow in **three stages** (role/time → keywords include/exclude → CV-aware LLM), persist results and history. **Temporal** runs durable workflows for manual runs and scheduled **events** (see specs).

High-level flow: **collect / read cache → stage 1 → stage 2 → stage 3 (LLM) → persist**. Same engine for interactive API triggers and cron-like event runs; optional third-party notifications are **out of MVP** (see product draft).

## Core Principles

### I. One collector interface

Every source implements a single `Collector` contract; adding a site does not require changing unrelated packages.

### II. Stages before blanket LLM

Stage 1 (broad role / time window) and stage 2 (keyword include/exclude) shrink the set; the LLM (stage 3) runs on that pool so we do not score obvious noise. Policy for auto vs manual LLM passes is defined in specs (e.g. per-run cap in `007`).

### III. Session abstraction for headless

LinkedIn (and similar) use a `session.Provider`; start with a cookie file, swap implementation without rewriting collectors.

### IV. Postgres as system of record

Vacancies, deduplication, event run history, user profile text, and scoring outcomes live in **PostgreSQL** (via GORM + migrations). No SQLite in the target architecture.

### V. Temporal for orchestration

Workflows coordinate activities (fetch, persist, score). Retries, visibility, and local/cloud parity are first-class; auth is deferred but data models stay extensible for a future `user_id` (or equivalent).

### VI. Config without secrets in repo

Secrets and local paths live in `.env` (gitignored). Variable **names** are documented in **`specs/*/contracts/environment.md`**, **README**, and **`internal/config`** (not duplicated in the Makefile). Production targets **GCP** for runtime and secrets.

**Single source of truth**: infrastructure and app settings that come from the environment are defined in **`internal/config`**: exported env **name** constants, typed structs (e.g. `Database`, `Temporal`), and loaders (`Load`, `LoadDatabaseFromEnv`, `LoadTemporalFromEnv`). **`cmd/*`** reads env and passes structs into `internal/*`. **Feature modules** (`internal/jobs`, `internal/pipeline`, …) must not call `os.Getenv` for shared knobs — add parsing and defaults in `internal/config` instead. **Temporal connection** (address, namespace, task queue) is config only; **workflow and activity code** live inside each feature module under `internal/<feature>/workflows/` (with `activities/` as needed), same idea as `omg-api` — not a single catch-all `internal/temporal` package. Workflow registration: constructor **`New...`** on a workflows type must **list every** `RegisterWorkflow` call so the inventory is obvious (same pattern as `omg-api` feature workflows).

**Product HTTP** may later live under **`cmd/api/`** with thin composition (deps → route `Group` → `handlers.NewHTTPHandler`), like `omg-bo/cmd/api`. Until then, **`internal/collectors/handlers/debughttp/`** is **development-only** debug HTTP for collectors — not the public API; same **omg-bo handler layout** (`handler.go` + `registerRoutes()` + one file per route, `helpers`/`response` as needed). **Collectors** module **data types** (debug POST bodies, enums, constants, payloads — everything in the **`schema/`** sense) belong in **`internal/collectors/schema/`** at the **module root**, not under `handlers/`. Implemented with **`net/http`** and **`debughttp.NewHTTPHandler`** wired from **`cmd/agent`**; short package comment stating dev-only.

## Stack (target)

- Go 1.24
- PostgreSQL + GORM + migrations
- Temporal (worker + workflows + activities)
- Collectors: `net/http`, goquery, go-rod where needed
- Claude API for scoring; results exposed via API/UI per specs (no Telegram in MVP)
- Local dev: **Docker Compose** (Postgres, Temporal stack) as specified in `specs/000-epic-overview`

## Internal layout

**`internal/platform/`** holds **process infrastructure**, not product modules: shared wiring helpers (e.g. **`platform/pgsql`** — GORM open, pool from `config`, `GormGetter` for repositories). It is **not** a peer of `jobs` / `pipeline`; feature `storage/` packages depend on types from `platform` only for DB access shaped in `cmd`.

Feature work lives in **modules** under `internal/<name>/`: each module is a self-contained unit; only expose what other packages need.

**`internal/domain/`** is the **shared product kernel** (not a feature module with `contract.go`): **`internal/domain/schema`** holds cross-cutting types (`Job`, `ScoredJob`); **`internal/domain/utils`** holds stable vacancy identity (`StableJobID`, `NormalizeListingURL`, `AssignStableID`). **No** GORM and **no** Temporal SDK under `internal/domain/**`.

At the module root, **`contract.go`** (or the same role split across files) holds interfaces and the module’s public surface.

Optional subfolders inside a module (create only what the module uses):

| Folder | Role |
|--------|------|
| `handlers/` | Inbound adapters (HTTP, etc.). `handler.go` with `registerRoutes()` listing all routes; **one file per handler/route**; `helpers.go` for ALL shared parsing helpers (path values, query params, pagination, etc.); `response.go` for all write helpers. **Never define package-level functions inside a route file** — if it is not a method on the handler struct it goes in `helpers.go` or `response.go`. |
| `impl/` | Application **service** / use cases. Folder name is **`impl`** (not `service`). **No Temporal SDK imports** (`go.temporal.io/sdk/...`) — Temporal-to-domain mapping belongs in `workflows/`. |
| `schema/` | Module-local **data model** — not “HTTP DTOs only”. Everything that is a **named data shape or classification** belongs here: structs (domain-facing and wire-facing), **enums**, **named constants** tied to those types (status codes, keys, limits where they are part of the model), request/response bodies, workflow/handler/activity payloads, and module-local **exported errors** when they are part of the contract surface. Pure behavior stays out (`impl/`, `utils/`). No global schema repo — types stay inside the module. Every exported string enum follows the **Canonical Enum Pattern** (see below). |
| `storage/` | Persistence only. **Use this name only** — do not introduce parallel `repository/` packages. |
| `mapper/` | Optional mapping between layers (e.g. DTO <-> domain). Prefer storage `ToModel` / `ToDomain` until mapping outgrows that. |
| `mock/` | Test doubles for the module. |
| `utils/` | Small helpers used only inside this module. **Module root (`contract.go`, `errors.go`, etc.) holds only interfaces and sentinel errors** — standalone helper functions go in `utils/` even if two lines. |
| `workflows/` | **Required when the module uses Temporal** (not optional). `New...` constructor explicitly calls `RegisterWorkflow` for every workflow. Temporal-state-to-domain mapping helpers live in `workflows/mappers.go` (or `workflows/utils/`). `activities/` for activity structs and methods. Wire from `cmd/worker`. |

**`internal/llm/`** is its own module: **`contract.go`** (e.g. `Scorer`), provider packages (`anthropic/`, …), **`mock/`**, and **`utils/`** for shared LLM response parsing / small helpers — **not** loose `*.go` helpers at `internal/llm` root.

**`internal/pipeline/`**: stage rule **structs** and other pipeline **value types** (`BroadFilterRules`, `KeywordRules`, stage payloads, …) belong in **`internal/pipeline/schema/`**; the module root keeps **`contract.go`**, errors, and similar **non-data** entrypoints only. **Implementations** of stages 1–3 batching live under **`internal/pipeline/utils/`** (`ApplyBroadFilter`, `ApplyKeywordFilter`, `ScoreJobs`). If legacy type definitions still sit at the pipeline root (e.g. `stage_rules.go`), move them into **`schema/`** the next time that code is touched.

**`internal/publicapi/`** (product HTTP for the browser client): **`handlers/`** is one-file-per-route + `handler.go` + `schemas_embed.go` only; **shared HTTP helpers** (path/query, JSON read/write, CORS middleware) live in **`internal/publicapi/utils/`**. **Request bodies**: embed JSON Schema under **`handlers/json_schema/*.schema.json`** with **`//go:embed`** (same sequence as `omg-ap` transaction handlers: schema validate, then decode into typed structs in **`publicapi/schema/`**). Use **`publicapi/utils.ValidateJSONInstance`** with **`github.com/santhosh-tekuri/jsonschema/v6`**; **`publicapi/utils.ReadValidatedJSON`** wraps read → validate → `json.Decoder.DisallowUnknownFields()` decode.

**`cmd/`** holds binaries and **composition only** (construct deps, register workflows, mount HTTP groups). Business rules belong in `internal/<module>/impl`, not in `cmd`. **`internal/platform/`** and the migrate CLI do not mirror the full feature-module table.

**Documentation in code**: prefer readable names and layout over long comments. Reserve comments for non-obvious behavior, invariants, and exported APIs that need `godoc`. Avoid banner blocks and redundant per-method commentary.

## Canonical Patterns

These patterns are named so that instructions can reference them without pointing to a specific project (which may not exist in future contexts).

### Canonical Enum Pattern

Every exported string-based enum type (e.g. in `schema/`) must expose:

```go
func (e MyType) String() string                          { return string(e) }
func (e MyType) Equals(s string) bool                    { return string(e) == s }
func (e MyType) Pointer() *MyType                        { return &e }
func (e MyType) FromValue(s string) (MyType, error)      { /* switch + descriptive error */ }
func ValuesMyType() []MyType                             { return []MyType{...} }
func FromStringMyType(s string) (MyType, error)          { var z MyType; return z.FromValue(s) }
```

A bare `Valid() bool` is insufficient and must be extended to this form.

### Schema-first validation (HTTP)

HTTP handlers must decode request bodies into a **typed struct** in that module’s **`schema/`** (request types are part of the data model, not a separate “DTO-only” bucket). Decoding uses `json.Decoder.DisallowUnknownFields()`. **Structural / field rules** for JSON bodies should be expressed in an **embedded JSON Schema** (`//go:embed` + validate **before** decode), same idea as `ValidateRequestBySchema` in `omg-ap` — in jobhound_core **`internal/publicapi`**, that is **`handlers/json_schema/*.schema.json`** plus **`publicapi/utils.ValidateJSONInstance`** and **`publicapi/utils.ReadValidatedJSON`**. A `Validate() error` on the request type is optional and mainly for non-HTTP entry points (workflows, CLIs) when needed; do not duplicate the same rules in both JSON Schema and `Validate()` for public API bodies.

**Forbidden**: `map[string]json.RawMessage` with per-field existence checks (`if _, has := raw["field"]; !has`). This pattern must never appear in handler code.

### Temporal Separation Pattern

Temporal workflow code is isolated from application service code:

- `impl/` — application service; receives workflow results through interfaces; no Temporal SDK imports.
- `workflows/` — workflow definitions and `RegisterWorkflow` calls; `activities/` for activity methods.
- `workflows/mappers.go` (or `workflows/utils/`) — functions that convert `*client.WorkflowExecutionDescription` or other Temporal types to domain / schema types.

## Testing

- **Unit tests** live next to the code they cover: `*_test.go` in the same directory and the **same package** as the implementation (white-box). This avoids export hacks and matches Go stdlib practice.
- **Where to test (boundaries, not trivia)**: prefer tests that exercise **product HTTP handlers** (route-level behavior against the mux), **application services** (`impl/`), and **persistence** (`storage/`). Do **not** add tests whose main job is to cover **`utils/`** one-liners, **raw `schema/`** shapes, or **enum** `String` / `FromValue` round-trips unless the logic is genuinely non-trivial (security-sensitive transforms, subtle parsing, invariants that are easy to break). Thin glue is covered **indirectly** by handler or service tests.
- **Black-box tests** (optional): same directory, `package foo_test`, import the package under test to assert only its exported API.
- **Integration tests** (real Postgres, migrations, Temporal, etc.): use the build tag **`integration`** (`//go:build integration` at the top of the file). Keep them beside the package they exercise (e.g. `internal/platform/pgsql/…`) or, when a scenario spans modules, under **`tests/integration/`** with the same tag. Default `go test ./...` must stay fast and must not require Docker; use `make test-integration` or `go test -tags=integration ./...` for tagged tests.

## Governance

- Amend this file when architecture decisions change; keep it short and actionable.
- Feature details and order of implementation: `specs/` (per-feature folders, same style as `omg-bo/specs`).

**Version**: 1.8.2 | **Ratified**: 2026-03-29 | **Amended**: 2026-04-05 — testing policy: unit tests at handlers / `impl` / `storage` boundaries; no trivial `utils` / schema / enum tests
