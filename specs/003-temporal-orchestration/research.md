# Research Notes: Temporal orchestration foundation

**Branch**: `003-temporal-orchestration`  
**Spec**: `specs/003-temporal-orchestration/spec.md`  
**Date**: 2026-03-29

Inventory of **jobhound_core** today, Temporal Go SDK usage expectations, and Compose patterns. Unknowns are marked **TBD** until implementation locks them (see `plan.md` Resolved decisions).

---

## 1. Current repo state (`003` start)

| Area | Status |
|------|--------|
| `go.mod` | Go **1.24**; **no** Temporal SDK dependency yet |
| `docker-compose.yml` | **Postgres only** (`postgres:16`, port `5432`) per `002` |
| `cmd/agent` | Agent binary; **no** Temporal worker |
| `internal/domain` | Job domain types; **must remain** free of `go.temporal.io` imports |
| Makefile | `build`, `run`, `test`, etc. — worker target / Temporal help text **TBD** |

---

## 2. Temporal Go SDK (expected integration)

- **Module**: `go.temporal.io/sdk` (workflow, activity, worker, client packages).
- **Worker**: `worker.New(c, taskQueue, options)` + `RegisterWorkflow` / `RegisterActivity`; `Run()` blocks until interrupt.
- **Client**: `client.Dial(options)` or equivalent; `ExecuteWorkflow` with workflow name, task queue, and namespace consistent with worker.
- **Testing**: `go.temporal.io/sdk/testsuite` provides in-memory `WorkflowEnvironment` / `TestWorkflowEnvironment` for unit tests without a real server — aligns with acceptance “no Docker for plain `go test ./...`”.

**TBD**: exact SDK minor version pin in `go.mod`; match samples from [Temporal Go SDK samples](https://github.com/temporalio/samples-go) for option names.

---

## 3. Docker Compose (Temporal + UI)

Common local patterns:

- **`temporalio/auto-setup`** (or documented all-in-one image) bootstraps server with DB backend; often pairs with **Postgres** service — may share existing `postgres` service or use embedded/ephemeral DB per image docs (**lock in implementation** to avoid two conflicting Postgres roles).
- **Temporal Web UI**: separate image or bundled in stack; expose HTTP port to host for browsing runs and task queues.

**Constraints from spec**:

- **`docker compose up`** brings up **Postgres (as today)**, **Temporal**, **UI**.
- **Pin image tags** (no floating `latest` in committed compose).
- Document **UI URL** and **gRPC frontend** host port for workers on the host.

**TBD**: exact image names/tags and whether Temporal uses the same Postgres instance as the app or an internal DB — decide in implementation and document in README.

---

## 4. Layering and future hooks

- **Workflows orchestrate only**; **activities** are the extension point for pipeline and storage (`004`–`006`).
- **This feature**: demo activity returns a fixed or computed string **without** DB — proves wiring only.
- **`cmd/worker`** should stay thin: parse env from `internal/config`, dial Temporal, call each feature module’s workflow registration (e.g. `internal/reference/workflows.Register`).

---

## 5. Dependencies on other specs

| Spec | Relationship |
|------|----------------|
| `001` | Repo layout; domain must not import Temporal |
| `002` | **Not required** for demo path; worker may omit `JOBHOUND_DATABASE_URL` if activities stay DB-free |
| `000` | Epic: Compose stack includes Postgres + Temporal |

---

## 6. Out of scope (recap)

- Production worker deployment on GCP (env-only note OK).
- Real schedules, ingest, Telegram — later specs.
- Advanced tracing/correlation — `012`.

---

## 7. References (paths)

- Spec: `specs/003-temporal-orchestration/spec.md`
- Postgres compose: `docker-compose.yml`
- Constitution: `.specify/memory/constitution.md`
- Epic: `specs/000-epic-overview/spec.md`
