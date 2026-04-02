# Implementation Plan: Job collectors

**Branch**: `005-job-collectors`  
**Date**: 2026-03-30  
**Last Updated**: 2026-04-02  
**Spec**: `specs/005-job-collectors/spec.md`  
**Input**: Feature specification, `resources/*`, `contracts/*`, [`product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Implement **two MVP collectors** (Europe Remotely, Working Nomads) under **`internal/collectors/`**, each satisfying **`pipeline.Collector`**, normalizing to **`domain.Job`** per **`contracts/domain-mapping-mvp.md`**. **HTTP**: `net/http` + **goquery** (Europe) and **`encoding/json`** (Working Nomads); **no retries**; **`httptest`** + fixtures for **`go test ./...`** without live network.

**Product boundary**: collectors are **per-source fetch + map**; **slot-scoped ingest**, **upsert**, **watermarks**, and **coordination** are **`006`** — see **`spec.md`** “Product alignment” and **`contracts/collector.md`** “Relationship to orchestration”.

## Technical Context

**Language/Version**: Go 1.24  
**Domain**: `internal/domain.Job`; extensions and `jobs` table per **`jobs-table-extension.md`** when those fields ship  
**Module home**: `internal/collectors/` (+ `utils/`); **`internal/pipeline`** stays free of site-specific parsing  
**Testing**: Colocated `*_test.go`; fixtures from **`contracts/test-fixtures.md`** or **`testdata/`** copies  
**Clock**: Europe Remotely relative dates — injectable **`time.Now`** (or small clock) in tests per **`domain-mapping-mvp.md`**

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| Tiered stack (stdlib / goquery before rod) | **PASS** (MVP sources are T2) |
| Config: `JOBHOUND_*` in `internal/config` only | **PASS** (no `os.Getenv` in collector packages beyond agreed wiring) |
| Interfaces at module boundary | **PASS** (`pipeline.Collector` + source-internal parse helpers as needed) |
| `go test ./...` without mandatory network | **PASS** (`httptest` + saved bodies) |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Contracts | Already in `contracts/*` + `resources/*`; adjust only if implementation finds gaps |
| 1 Shared utils | `internal/collectors/utils` — HTTP client defaults, country resolution from `data/countries.json`, URL normalization, shared date helpers |
| 2 Europe Remotely | `POST admin-ajax.php` + listing fragment parse + `GET` detail parse → `domain.Job`; tests per `tasks.md` |
| 3 Working Nomads | `POST jobsapi/_search` decode + map `_source` → `domain.Job`; tests per `tasks.md` |
| 4 Wire-up | Register collectors where the agent/worker builds the pipeline; domain + migration if `SalaryRaw` / `Tags` / `Position` land with this feature |
| 5 Quality gates | `make test`, `make vet`, `make fmt` clean |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **Source strings** | `europe_remotely`, `working_nomads` — **`contracts/collector.md`** |
| D2 | **Retries** | None on collector HTTP — **`contracts/collector.md`** |
| D3 | **Partial batch** | Missing required fields on a card/hit → **abort whole `Fetch`** for that source (except Europe date soft-fail) — **`contracts/collector.md`** |
| D4 | **Fixtures** | Samples live in **`contracts/test-fixtures.md`**; code may copy to `testdata/` |
| D5 | **Rod / T3** | Out of scope for MVP two sources; **`contracts/environment.md`** when first T3 collector appears |

## Engineering follow-ups (non-blocking)

- Remaining inventory rows — later specs/tasks.
- **`006-cache-and-ingest`** — persistence, dedup, watermarks.

## Project structure (documentation)

```text
specs/005-job-collectors/
├── spec.md
├── plan.md
├── research.md
├── tasks.md
├── checklists/
│   └── requirements.md
├── contracts/
│   └── …
└── resources/
    └── …
```

## Source structure (anticipated — implementation phase)

```text
internal/collectors/
├── europe_remotely/   # or euremote/ — one package per source
├── working_nomads/
└── utils/
```
