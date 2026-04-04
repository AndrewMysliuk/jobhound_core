# Implementation Plan: HTTP public API (UI-facing)

**Branch**: `009-http-public-api`  
**Date**: 2026-04-04  
**Last Updated**: 2026-04-04  
**Spec**: `specs/009-http-public-api/spec.md`  
**Input**: Feature specification + [`contracts/http-public-api.md`](./contracts/http-public-api.md) + [`contracts/environment.md`](./contracts/environment.md) + [`../008-manual-search-workflow/spec.md`](../008-manual-search-workflow/spec.md) + [`../008-manual-search-workflow/contracts/manual-workflow.md`](../008-manual-search-workflow/contracts/manual-workflow.md) + [`../008-manual-search-workflow/contracts/filter-invalidation.md`](../008-manual-search-workflow/contracts/filter-invalidation.md) + [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Deliver a **stable JSON HTTP API** under **`/api/v1`** for a **browser** client: slot list/card/create/delete (cap **3**), global stage-3 **profile** (`GET`/`PUT`), **on-demand** stage **2** and **3** runs mapped to Temporal run kinds **`PIPELINE_STAGE2`** and **`PIPELINE_STAGE3`** (**no** **`POST …/stages/1/run`** for existing slots), **paginated** job lists per stage with optional **`bucket`** filter, and **manual bucket** corrections (`PATCH`). Implementation: thin **`cmd/api`** (stdlib `net/http` or a small router—project choice), **omg-bo-style** **`handlers/`** + module-local **`schema/`**; **CORS** from config (**`JOBHOUND_API_CORS_ORIGINS`**). **Stage math, persistence, and workflows** stay in **`002`–`008`**; this epic wires **HTTP ↔ storage/Temporal** with normative **409** concurrency (**one active run per stage per slot**), **409** slot cap, and the **single error envelope** from the spec.

## Technical Context

**Language/Version**: Go 1.24  
**HTTP**: stdlib `net/http` and/or chosen router; **no** Gin unless product explicitly adopts it elsewhere  
**Orchestration**: Temporal client from **`cmd/api`** — start **`008`** parent / run kinds per spec; I/O and workflow starts **outside** workflow determinism  
**Stages / data**: `004`–`007`; slot membership and manual workflow semantics: **`008`** + `contracts/manual-workflow.md`  
**Database**: PostgreSQL + GORM + existing **`internal/jobs`** (or agreed) **storage** for slots, jobs, pipeline state  
**Testing**: Colocated unit tests for handlers (httptest), table-driven; optional `//go:build integration` with Compose + worker when needed  
**Module layout**: Per constitution — **`contract.go`**, **`impl/`**, **`storage/`**, **`schema/`**, **`handlers/`** under the chosen feature module(s); **`cmd/api`** is composition only  

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| V. Temporal for orchestration | **PASS** (API only **starts** workflows; logic stays in **`008`** / pipeline modules) |
| IV. Postgres as system of record | **PASS** (reads/writes via existing storage patterns; no ad-hoc SQL in handlers) |
| VI. Config in `internal/config` | **PASS** (**`JOBHOUND_API_CORS_ORIGINS`**, bind address/port, any API-only knobs) |
| Handlers layout (omg-bo) | **PASS** — `handler.go` + `registerRoutes` + one file per route; shared small `helpers`/`response` if needed |
| Testing: default `go test` without Docker | **PASS** for handler/unit layer; integration optional |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Config & contracts | **`JOBHOUND_API_*`** (CORS origins, listen addr/port) in **`internal/config`**; add or refresh **`specs/009-http-public-api/contracts/environment.md`** when keys are frozen |
| 1 **`cmd/api` skeleton** | Binary builds via **`make`**; HTTP server lifecycle; route registry entrypoint mirroring **`debughttp.NewHTTPHandler`** pattern |
| 2 Shared HTTP plumbing | JSON encode/decode helpers; **error envelope** (`400`/`404`/`409`/`422`/`500` per spec); **CORS** middleware or wrapper using config list |
| 3 **`schema/`** | Request/response DTOs for every route in **`spec.md`** (slots, profile, stage runs, job lists, PATCH bucket); stable field names for UI |
| 4 Slots API | **`GET/POST /api/v1/slots`**, **`GET/DELETE /api/v1/slots/{slot_id}`** — cap **3**, **`201`** shape vs list item shape per spec |
| 5 Profile API | **`GET/PUT /api/v1/profile`** — text + **`updated_at`**; document interaction with stage-3 invalidation (**§5** product draft) on engine side |
| 6 Stage run API | **`POST …/stages/2/run`** and **`POST …/stages/3/run`** — body validation, **`202`** payload; enforce **409** `stage_already_running`; map to **`008`** kinds + parameters (**`max_jobs`**, **`include`/`exclude`**) |
| 7 Job lists API | Paginated **`GET …/stages/1|2|3/jobs`** — **`page`**, **`limit`**, optional **`bucket`**; sort **`posted_at` DESC**, **`job_id` ASC**; **`total`** |
| 8 Manual bucket PATCH | **`PATCH …/stages/2|3/jobs/{job_id}`** — **`404`** when out of scope |
| 9 Quality gates | **`make test`**, **`make vet`**, **`make fmt`**; integration smoke optional |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **Feature module home** | One or more modules under **`internal/`** (e.g. slot-centric **`internal/slots`** or split **`internal/profile`**) with **`handlers/`** + **`schema/`**; **`cmd/api`** wires constructors only. |
| D2 | **Router** | Team picks stdlib mux vs minimal router; **route list** must remain obvious in **`registerRoutes`**. |
| D3 | **Stage object `error` on failure** | **One** style (object vs string) project-wide per **`spec.md`**; document in **`schema/`** comments or contract file when added. |
| D4 | **`stage_3_rationale`** | **Either** always present as `null` **or** omit when empty—pick one and apply to all list/card DTOs. |
| D5 | **`max_jobs` vs zero candidates** | **`422`** optional per spec; if not used, **`200`** list + successful run with zero scored—document chosen behavior in handler tests. |
| D6 | **Empty `include` / `exclude`** | Allowed or rejected—align with **`004`** / **`008`** filter semantics; document next to stage-2 handler. |

## Engineering follow-ups (non-blocking)

- **OpenAPI** document generation or hand-written spec — deferred per **`spec.md`**.  
- **Authentication** — reserved **`user_id`**; out of MVP.  
- **`cmd/agent` debug HTTP** — remains dev-only; product traffic targets **`cmd/api`**.

## Project structure (documentation)

```text
specs/009-http-public-api/
├── spec.md
├── plan.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── http-public-api.md     # routes, codes, JSON fields, Temporal mapping
    └── environment.md         # JOBHOUND_API_* ; keep in sync with internal/config
```

## Source structure (anticipated — implementation phase)

```text
cmd/api/main.go                      # thin composition: config, DB, Temporal client, NewHTTPHandler
internal/config/                     # JOBHOUND_API_CORS_ORIGINS, listen bind, etc.
internal/<feature>/                  # e.g. slots + profile — contract, impl, storage as needed
    schema/                          # JSON DTOs per spec.md
    handlers/                        # handler.go + registerRoutes + one file per route
internal/platform/pgsql/             # existing DB dial (reuse)
cmd/worker/                          # unchanged registration; API is a client
```
