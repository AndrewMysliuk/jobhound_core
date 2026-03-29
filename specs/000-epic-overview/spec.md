# Epic Overview: jobhound_core backend

**Feature Branch**: `000-epic-overview`
**Created**: 2026-03-29
**Last Updated**: 2026-03-29
**Status**: Completed

**Purpose**: Index of all planned features under `specs/`. Each numbered folder has its own brief `spec.md` for orientation; detailed requirements come later via Spec Kit per feature.

**Layer 0 (epic index)**: **Completed** — overview, feature table `001`–`012`, constitution v1.1, and Cursor rules aligned. **Docker Compose** for Postgres/Temporal is implemented when delivering `002` / `003`, not part of this milestone.

## Product summary

Personal job agent: ingest vacancies, **three narrowing stages** (role/time → keywords → CV-aware LLM), store in **PostgreSQL**, run **manual** and **scheduled event** flows via **Temporal**, optional **Telegram** (short payload). Web UI lives in a **separate repo**; this backend exposes HTTP API. **Auth omitted for now**; models stay extensible for a future user id.

## Local development (target)

- **Docker Compose** brings up **PostgreSQL** and **Temporal** (and UI where useful) so the stack runs on a laptop without GCP.
- Application services (API server, Temporal **worker**) run via `make` / `go run` against compose, or are added as compose services as the project matures.
- Documented env vars (no secrets in git); see Makefile / `config` as implementations land.

## Testing stance

- **Collectors**: deterministic tests with fixtures (`httptest`, golden HTML); optional targeted live checks outside default CI if needed.
- **Workflows**: a small set of Temporal tests (e.g. mock activities) for critical paths — no fanaticism.
- **Stages 1–3**: table-driven unit tests, no network.

## Feature index

| # | Folder | One-line scope |
|---|--------|----------------|
| 001 | `001-agent-skeleton-and-domain` | Go layout, `domain.Job` (+ apply URL field), `Source`+listing stable IDs, optional user scope |
| 002 | `002-postgres-gorm-migrations` | DB connection, GORM, migration strategy, base tables |
| 003 | `003-temporal-orchestration` | Client, worker, dev/prod wiring, thin hello-workflow pattern |
| 004 | `004-pipeline-stages` | Pure services: stage 1 / 2 / 3 callable from activities |
| 005 | `005-job-collectors` | `Collector` interface, tiered sources (HTTP, goquery, rod) |
| 006 | `006-cache-and-ingest` | Normalized job store, watermarks, cache vs explicit refresh rules |
| 007 | `007-llm-policy-and-caps` | Auto cap (e.g. 5), extra statuses, manual “run LLM” actions |
| 008 | `008-events-and-run-history` | Event entity, schedule, incremental runs, history (0 results OK) |
| 009 | `009-manual-search-workflow` | Same engine, API-triggered workflow, response contract |
| 010 | `010-telegram-notifications` | Activity: short messages, rate/cap alignment with 007 |
| 011 | `011-http-public-api` | REST (or RPC) for future UI: events, runs, manual actions |
| 012 | `012-observability` | Logging, correlation with workflow run ids, ops hooks for GCP |

## Suggested implementation order

`001` → `002` → `003` → `004` → `005` / `006` (schema coordinated) → `007` → `008` → `009` → `010` → `011` → `012` (can overlap early with 003).

## Related

- `.specify/memory/constitution.md` — principles and stack.
- Reference layout style: `omg-bo/specs` (folder per feature, `spec.md` inside).
