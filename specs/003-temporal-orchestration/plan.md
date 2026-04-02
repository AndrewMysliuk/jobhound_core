# Implementation Plan: Temporal orchestration foundation

**Branch**: `003-temporal-orchestration`  
**Date**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Spec**: `specs/003-temporal-orchestration/spec.md`  
**Input**: Feature specification + `research.md` + [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)

## Summary

Add **Temporal** to local dev and the repo: **Docker Compose** services for **Temporal Server** (with auto-setup) and **Temporal Web UI** alongside existing **Postgres**, a dedicated **`cmd/worker`** binary that registers workflows and activities on task queue **`jobhound`** in namespace **`default`**, and a **reference v0 workflow** plus one **deterministic, DB-free activity** to prove end-to-end execution and UI visibility. A **Temporal client** path (test and/or tiny dev entrypoint) starts the same workflow against the same queue/namespace. **`internal/domain`** must not import the Temporal Go SDK; workflow/activity code lives under a dedicated package (see Resolved decisions). **Plain `go test ./...`** stays Docker-free via **in-memory test env** *or* **`integration`-tagged** tests only (constitution).

**MVP draft alignment**: the global product draft defines **slots**, **reset-without-refetch**, and **idempotent** stage-3 work under retries. This plan stays **foundation-only**; those behaviors are specified in **`004`**, **`006`**, **`007`**, and consumer epics **`008`–`010`**, not in `003`.

## Technical Context

**Language/Version**: Go 1.24  
**Orchestration**: Temporal Go SDK (`go.temporal.io/sdk`); server + UI via Compose, pinned image tags  
**Worker**: `cmd/worker` — connect, register, block until shutdown  
**Domain boundary**: no Temporal imports under `internal/domain`  
**Testing**: default unit tests without Docker; Temporal coverage via SDK test suite **or** `//go:build integration` against Compose  
**Layout**: per-feature `internal/<feature>/workflows/` + `activities/` (no global `internal/temporal`); `cmd/worker` calls each module’s registration

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| V. Temporal for orchestration | **PASS** (foundation worker + reference workflow) |
| VI. Config without secrets in repo | **PASS** (document env names; no secrets in Temporal local defaults) |
| Layering: domain without orchestration imports | **PASS** (SDK only outside `internal/domain`) |
| Testing: `go test ./...` without Docker | **PASS** (in-memory **or** integration-tagged only) |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Research | `research.md` |
| 1 Contracts | `contracts/environment.md`, `contracts/reference-workflow.md` |
| 2 Compose | Extend `docker-compose.yml`: Temporal + UI, ports documented; Postgres unchanged |
| 3 Dependencies | `go.mod`: Temporal SDK (+ test helpers if used) |
| 4 Internal package | Reference workflow + activity under `internal/reference/workflows/` (+ `activities/`); explicit timeouts/retries in code or spec-adjacent docs |
| 5 Worker binary | `cmd/worker`: env-based config, register, run worker on `jobhound` |
| 6 Client path | Start reference workflow for verification (test and/or dev-only `cmd` — per Resolved decisions) |
| 7 Docs | README: UI URL, gRPC address from host, env vars; README aligns with `contracts/environment.md` |
| 8 Tests | In-memory Temporal test **or** integration-tagged test; must not require Docker for default `go test` |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **Package for workflows/activities** | **Per module** under `internal/<feature>/workflows/` with `activities/` (v0 demo: `internal/reference/workflows/`). Aligns with `omg-api` feature layout. |
| D2 | **Namespace / task queue** | Fixed per spec: namespace **`default`**, task queue **`jobhound`**. Worker and every client must match. |
| D3 | **Client entry for dev** | **Prefer** automated test as primary proof; optional **`cmd/temporaldev`** or similar *only if* needed for manual UI demos — must be documented as dev-only. Final choice recorded in `tasks.md` when implemented. |
| D4 | **Temporal test strategy** | **Default**: Temporal **in-memory** test environment for `go test ./...`. **Optional**: add `-tags=integration` test against Compose; document `make test-integration` if added. |
| D5 | **Compose stack** | Use **official Temporal Docker images** with **auto-setup** pattern (pinned tags in `docker-compose.yml`); expose **Web UI** on a **documented host port** (e.g. `8088` or `8233` — lock to one value in implementation). |
| D6 | **gRPC address from host** | Document **frontend** host:port for workers/clients (typically **`localhost:7233`** when mapped from container); exact mapping in README + `contracts/environment.md`. |
| D7 | **Reference workflow shape** | One workflow calling **one** activity; **deterministic** result; **no** Postgres/GORM in the demo path (see `contracts/reference-workflow.md`). |
| D8 | **Timeouts / retries** | **Explicit and conservative** on workflow and activity options (in code with short comments or in `reference-workflow.md`); shared helpers for future workflows optional. |

## Engineering follow-ups (non-blocking)

- Real product workflows, ingest, schedules, and API-triggered runs — `006`, `008`, `009`, `010` (payloads include **`slot_id`** / reserved **`user_id`**; activities idempotent where caps or outcomes are written — draft §4).
- GCP worker topology and prod addressing — env-only note sufficient for `003`.
- Correlation / advanced observability — `011`.

## Project structure (documentation)

```text
specs/003-temporal-orchestration/
├── spec.md
├── plan.md
├── research.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── environment.md
    └── reference-workflow.md
```

## Source structure (anticipated — implementation phase)

```text
cmd/
├── worker/                 # Temporal worker binary (register + block)
└── ...                     # optional: tiny dev client cmd (D3)
internal/
└── reference/workflows/    # v0 demo: workflows.go, client helper, activities/
    └── activities/         # reference activities (expand with real features alongside)
docker-compose.yml          # postgres + temporal + temporal-ui (extended)
```
