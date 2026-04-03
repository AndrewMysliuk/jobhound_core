# Feature: Temporal orchestration foundation

**Feature Branch**: `003-temporal-orchestration`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-03  
**Status**: Implemented

## Goal

Add **Temporal** to the stack: a **dedicated worker binary** that runs workflows and activities, a **minimal reference workflow** to prove end-to-end execution, and **Docker Compose** services for **Temporal Server** and **Temporal Web UI** alongside existing Postgres. Workflows orchestrate only; **activities** are the hook for future calls into pipeline and storage (`004`, `006`, `007`). **No database access** is required in this feature’s demo path.

## Product alignment (MVP draft)

End-to-end product behavior—**search slots**, three pipeline stages, **reset rules** (recompute from PostgreSQL without implicit re-fetch), and **Temporal-safe idempotency** for capped LLM work—is the narrative source of truth in [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md). This epic implements **orchestration plumbing only**; it does not encode slot or pipeline semantics.

**Constraints for later workflows** (documented here so implementers of `006`, `007`, `008`, `009` share the same expectations):

- **Context**: product workflows and their activities must accept **`slot_id`** (and **`user_id`** when registration lands) in payloads started from API (and from a future schedule trigger if we add one)—see draft §2–§5 and epics **`008`**, **`009`**. The v0 reference workflow deliberately uses a **single string** input and no slot context.
- **Idempotency**: activities that consume **caps** or persist **stage-3** outcomes must remain correct under **Temporal retries** (no double-charging the cap, no conflicting rows for the same `(slot_id, job_id)` outcome)—draft §4; detailed types and storage in **`004`** / **`007`**.
- **Orchestration vs ingest**: “re-run stages after filter/profile change” means **activities read and write Postgres** per draft §5; a **new stage-1 crawl** is a **separate** user or API action (`006` / collectors), not an automatic side effect of filter edits.

## Worker binary and cmd layout

- **`cmd/worker`**: separate binary whose sole job is to connect to Temporal, register workflows and activities, and **block** on the worker until shutdown. It receives config (address, namespace, task queue) from **environment** (exact variable names documented alongside implementation).
- **`internal/domain`** must not import the Temporal Go SDK. Workflow and activity implementations live **inside the feature module** that owns them: `internal/<feature>/workflows/` with `activities/` underneath (same pattern as `omg-api`), e.g. v0 demo under `internal/reference/workflows/`. **`cmd/worker`** composes: load config, dial Temporal, call each module’s `Register` (or equivalent). Connection settings stay in **`internal/config`** only.

## Client and end-to-end proof

- A **Temporal client** is required to start the reference workflow for verification (automated test and/or a tiny dev-only entrypoint — choice left to implementation as long as acceptance criteria are met).
- Client and worker **must use the same** task queue name and namespace.

## Namespace and task queue (single namespace)

- **Namespace**: `default` — one namespace for local dev and the baseline for later environments; no multi-namespace design in this feature.
- **Task queue**: `jobhound` — used by both worker registration and any client that enqueues the reference workflow.

Timeouts and retry defaults for the reference workflow and its activities should be **explicit and conservative** (documented in code or spec-adjacent docs when implemented); shared helpers for future workflows are optional.

## Reference workflow (v0)

- One **demo workflow** (e.g. orchestrates a greeting or echo) calling **one simple activity** that returns a deterministic result **without** Postgres or GORM.
- Purpose: validate connectivity, registration, and UI visibility of runs — not business logic.

## Docker Compose (local)

- Extend the repo’s Compose file so **`docker compose up`** brings up **Postgres** (as today), **Temporal** (server / auto-setup pattern with pinned image tags), and **Temporal Web UI** with a **documented host port** for browsing workflows and task queues.
- Document how to point the worker (and test client) at the Temporal frontend address matching the Compose network (e.g. host port for gRPC).

## Out of scope

- Real product workflows: **ingest** (`006`), **manual/API-triggered runs** (`008`), **public HTTP API** entrypoints (`009`) — each epic defines workflow names, payloads, and task-queue usage on top of this foundation. **Scheduled auto-refresh** is product backlog (draft §8), not a numbered epic.
- Production deployment topology for workers on GCP (only high-level note that address/namespace will come from env in prod is fine).
- Advanced observability and correlation — **`010`**.

## Dependencies

- **`001-agent-skeleton-and-domain`**: repository layout and layering rules.
- **`002-postgres-gorm-migrations`**: **not required** for the demo workflow in this feature; worker may be started without a database URL if the demo activities stay DB-free.

## Acceptance criteria

1. Compose starts **Postgres**, **Temporal**, and **Temporal UI**; README or equivalent documents UI URL and how workers connect to Temporal from the host.
2. **`cmd/worker`** runs, registers the reference workflow and activity on task queue **`jobhound`**, and successfully executes a run when a workflow is started against namespace **`default`** and that queue.
3. The run is **visible in Temporal Web UI** (workflow + at least one completed activity).
4. **Automated test**: either Temporal’s in-memory test environment **or** one **`integration`-tagged** test against Compose — chosen approach must not require Docker for plain `go test ./...` (per constitution).
5. Environment variable names for Temporal connection (and defaults for namespace/queue if applicable) are **documented** for local dev.

## Related

- `specs/000-epic-overview/product-concept-draft.md` — MVP behavior, slots, reset rules, stage-3 policy.
- `specs/000-epic-overview/spec.md`
- `specs/002-postgres-gorm-migrations/spec.md`
- `specs/006-cache-and-ingest/spec.md` — ingest activities and Redis coordination.
- `specs/007-llm-policy-and-caps/spec.md` — caps and idempotency with Temporal retries.
- `specs/008-manual-search-workflow/spec.md`
- `specs/009-http-public-api/spec.md`
- `specs/010-observability/spec.md`
- `.specify/memory/constitution.md`

## Planning artifacts

- `plan.md` — phases, constitution check, resolved decisions
- `research.md` — Compose/service layout and SDK notes
- `tasks.md` — implementation checklist
- `checklists/requirements.md` — spec quality checklist
- `contracts/environment.md` — Temporal-related env vars
- `contracts/reference-workflow.md` — v0 workflow/activity names, queue/namespace, I/O contract
