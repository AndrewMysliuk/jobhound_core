# Implementation Plan: Observability (structured logging)

**Branch**: `010-observability`  
**Date**: 2026-04-05  
**Last Updated**: 2026-04-05  
**Spec**: [`spec.md`](./spec.md)  
**Input**: [`spec.md`](./spec.md) + [`contracts/logging.md`](./contracts/logging.md) + [`contracts/environment.md`](./contracts/environment.md) + [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Introduce **`internal/platform/logging`** (zerolog, `EnrichWithContext`, context-carried correlation ids, field constants aligned with omg-api **names**). Wire **`JOBHOUND_LOG_LEVEL`** / **`JOBHOUND_LOG_FORMAT`** through **`internal/config`**. Add HTTP middleware for **`X-Request-ID`** on **debug HTTP** (`cmd/agent`) and **public API** (`cmd/api`). Instrument **every important boundary**: command entrypoints, all debug HTTP handlers, all public API handlers, **`slots` / `profile` / `pipeline` `impl`**, **ingest / pipeline / manual / retention** workflows and their **activities**, **`collectors/multi`** (replace ad-hoc `log.Printf`). Prefer **Error** + `.Err()` on failures, **Debug** for optional request/start lines; **storage** remains mostly silent except intentional **Warn** skips. Replace stray **`log.Printf`** / **`log.Println`** in **`cmd/api`**, **`cmd/worker`**, **`cmd/agent`**, **`cmd/retention`**, and **`internal/collectors/multi`** with structured logs where they represent app events (startup/shutdown/fatal errors may stay minimal but should use the same logger where practical).

## Technical Context

**Language/Version**: Go 1.24  
**Logging**: `github.com/rs/zerolog` (new direct dependency)  
**Constraint**: **No** `go-common` — see [`../002-postgres-gorm-migrations/spec.md`](../002-postgres-gorm-migrations/spec.md)  
**HTTP**: stdlib `net/http` — middleware wraps `http.Handler`  
**Temporal**: Activities use `context.Context` + `activity.GetInfo`; workflows use workflow logger fields per project Temporal rules  
**Testing**: Nop or `httptest` + discarded logger in tests; no mandatory network

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| Config in `internal/config` | **PASS** — all `JOBHOUND_LOG_*` in config loaders only |
| Module layout | **PASS** — `internal/platform/logging` as shared platform code |
| No secrets in logs | **PASS** — contract in `contracts/logging.md` §Security |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Contracts | [`contracts/logging.md`](./contracts/logging.md), [`contracts/environment.md`](./contracts/environment.md) (this epic) |
| 1 Platform package | `internal/platform/logging`: constants, context helpers, `EnrichWithContext`, optional `New` for root logger from config |
| 2 Config & cmd bootstrap | `internal/config` logging fields; `cmd/agent`, `cmd/api`, `cmd/worker`, `cmd/retention` construct root logger and pass into handlers/worker wiring |
| 3 HTTP middleware | `X-Request-ID` + context: debughttp router (`cmd/agent`); public API `NewHTTPHandler` / mux wrapper (`cmd/api`) |
| 4 Debug HTTP handlers | Every route file under `internal/collectors/handlers/debughttp/`: `logH` + handler field + errors |
| 5 Public API handlers | Every handler in `internal/publicapi/handlers/` (except pure test helpers): same pattern; extend `Deps` or ctor with `*zerolog.Logger` |
| 6 Domain impl | `internal/slots/impl`, `internal/profile/impl`, `internal/pipeline/impl`: service logger with `service` field; per-method `method` + `EnrichWithContext` |
| 7 Temporal — activities | `internal/ingest/workflows/activities`, `internal/pipeline/workflows/activities`, `internal/manual/workflows/activities`, `internal/jobs/workflows/activities`: activity entry logging + workflow/run/slot ids |
| 8 Temporal — workflows | `internal/ingest/workflows` (`IngestSourceWorkflow`), `internal/manual/workflows`, `internal/jobs/workflows`: `workflow` field + deterministic ids (pipeline package: activities only today) |
| 9 Collectors & CLI | `internal/collectors/multi/multi.go`; `cmd/agent/temporal_manual.go` — structured errors/info |
| 10 Quality | `make test`, `make vet`, `make fmt`; README / Compose snippet for `JOBHOUND_LOG_FORMAT=json` |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | Package location | **`internal/platform/logging`** |
| D2 | HTTP correlation | **`X-Request-ID`**; generate if absent; attach to `r.Context()` for debug + public API |
| D3 | omg-api parity | Field **names** and Enrich pattern; implementation code is local, not imported from Omega repos |

## Project structure (documentation)

```text
specs/010-observability/
├── spec.md
├── plan.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── environment.md
    └── logging.md
```

## Source structure (touch list)

```text
internal/platform/logging/          # new
internal/config/                    # LOG_* env parsing
cmd/agent/main.go
cmd/agent/temporal_manual.go
cmd/api/main.go
cmd/worker/main.go
cmd/retention/main.go
internal/collectors/handlers/debughttp/   # handler.go middleware + each *.go route
internal/publicapi/handlers/               # handler.go + deps; each route file
internal/slots/impl/
internal/profile/impl/
internal/pipeline/impl/
internal/ingest/workflows/
internal/ingest/workflows/activities/
internal/pipeline/workflows/
internal/pipeline/workflows/activities/
internal/manual/workflows/
internal/manual/workflows/activities/
internal/jobs/workflows/
internal/jobs/workflows/activities/
internal/collectors/multi/multi.go
```
