# Epic Overview: jobhound_core backend

**Feature Branch**: `000-epic-overview`
**Created**: 2026-03-29
**Last Updated**: 2026-04-03  
**Status**: Active (index complete; numbered epics aligned incrementally with the product draft)

**Purpose**: Index of all planned features under `specs/`. Each numbered folder has its own brief `spec.md` for orientation; detailed requirements come later via Spec Kit per feature. If an epic contradicts [`product-concept-draft.md`](./product-concept-draft.md) on user-visible or data-lifecycle behavior, **fix the epic** (or the draft first if the product decision changed).

**End-to-end product behavior (draft)**: [`product-concept-draft.md`](./product-concept-draft.md) — search **slots**, three stages, filter **reset** rules, stage-3 **cap/ordering/idempotency**, MVP vs later; narrative source of truth until promoted or split.

**Layer 0 (epic index)**: Overview, feature table `001`–`010`, constitution, and Cursor rules. **Docker Compose** for Postgres/Temporal lands with **`002` / `003`**, not as a separate milestone.

## Product summary

Personal job agent built around **search slots** (product constant **3–5** per user later; **single-tenant** operationally for MVP): each slot has an immutable **stage-1 broad keyword string** after the first successful ingest, **bound sources**, and **slot-scoped** vacancy rows and pipeline outcomes (`slot_id`; schema **reserves `user_id`** for registration and isolation without rewriting ownership).

**Three stages** (see draft §1–§4): **(1)** broad external ingest and optional delta refresh; **(2)** local narrow filters (include/exclude; optional date TBD) on the stage-1 pool only; **(3)** LLM on rows that passed stage 2, with **cap, deterministic ordering, eligible pool, and idempotency** per draft §4 (epics **`004` / `007`**). **Manual marks** stay coarse (same passed/failed buckets as the pipeline) plus a small correction path; **reset** when filters change wipes dependent outcomes per draft §5.

**PostgreSQL** for persistence; **Temporal** for workflows. **API-first** (**`009`**); any product UI is a separate deliverable. **Scheduled auto-refresh** (draft §8) is **product backlog**—not a numbered epic in this repo until we add one. **Auth** may be omitted in MVP APIs; **schema** still carries **`user_id`** where needed for a later multi-user model.

## Local development (target)

- **Docker Compose** brings up **PostgreSQL** and **Temporal** (and UI where useful) so the stack runs on a laptop without GCP.
- Application services (API server, Temporal **worker**) run via `make` / `go run` against compose, or are added as compose services as the project matures.
- Documented env vars (no secrets in git); see `specs/*/contracts/environment.md`, README, and `internal/config` as implementations land.

## Testing stance

- **Collectors**: deterministic tests with fixtures (`httptest`, golden HTML); optional targeted live checks outside default CI if needed. Local **`cmd/agent`** debug HTTP (see **`specs/005-job-collectors/spec.md`**) exercises real sources with full `domain.Job` JSON and optional Working Nomads ES overrides.
- **Workflows**: a small set of Temporal tests (e.g. mock activities) for critical paths — no fanaticism.
- **Stages 1–3**: table-driven unit tests, no network.

## Feature index

| # | Folder | One-line scope |
|---|--------|----------------|
| 001 | `001-agent-skeleton-and-domain` | Go layout, `domain.Job` (+ apply URL field), `Source` + listing stable IDs; **`user_id` / slot-ready** domain shapes |
| 002 | `002-postgres-gorm-migrations` | DB connection, GORM, migration strategy, base tables (**slot- and user-aware** as specs require) |
| 003 | `003-temporal-orchestration` | Client, worker, dev/prod wiring, thin hello-workflow pattern |
| 004 | `004-pipeline-stages` | Pure stage logic: **broad pool → narrow (local) → LLM**; callable from activities; semantics per draft |
| 005 | `005-job-collectors` | `Collector` interface, tiered sources (HTTP, goquery, rod) |
| 006 | `006-cache-and-ingest` | Normalized job store, watermarks, delta vs refresh; **Redis lock + cooldown by `source_id`** (shared across slots); slot-scoped association |
| 007 | `007-llm-policy-and-caps` | Caps, **ordering, eligible pool, idempotency** (draft §4); pipeline runs mapped to **slot** (+ user); manual “next batch” style actions |
| 008 | `008-manual-search-workflow` | Same engine, **API-triggered** workflow, response contract; **slot id**, reset rules §5 |
| 009 | `009-http-public-api` | REST (or RPC) for UI: slots, profile, runs, manual actions; **§5 reset** semantics |
| 010 | `010-observability` | Structured logging; correlation (Temporal + HTTP + **`slot_id`/`user_id`**); GCP-friendly export; optional Grafana-style **ops** dashboards (draft §7) — post-core per draft §9 |

## Suggested implementation order

**Product phasing** (see draft §9): **(1)** core vertical—slots, profile, ingest + delta, stage-2/3 recompute, persistence, **minimal API / manual triggers** to drive it; **(2)** more sources (`005`); **(3)** extensions—scheduled auto-refresh (draft §8; backlog until specced), richer observability (`010`).

**Dependency-friendly sequence**: `001` → `002` → `003` → `004` → `005` / `006` (schema coordinated) → `007` → **`008` / `009`** (manual workflows + HTTP for the core path) → **`010`** (can overlap early with `003`).

## Related

- [`product-concept-draft.md`](./product-concept-draft.md) — global product draft (slots, stages, reset rules, stage-3 policy, MVP vs backlog).
- `.specify/memory/constitution.md` — principles and stack.
- Reference layout style: `omg-bo/specs` (folder per feature, `spec.md` inside).
