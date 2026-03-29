# Tasks: Temporal orchestration foundation

**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/*`  
**Tests**: REQUIRED — acceptance criterion 4: **no Docker** for plain `go test ./...`; use in-memory Temporal test **or** `integration` tag only.

## A. Contracts & documentation

1. [x] **Freeze Temporal env contract** — Definition of done: `contracts/environment.md` matches implemented var names; Makefile `help` lists Temporal-related names; README points to contracts and UI/gRPC ports.
2. [x] **Freeze reference workflow contract** — Definition of done: `contracts/reference-workflow.md` matches registered workflow/activity names, queue, namespace, and input/output shapes in code.

## B. Local stack

1. [x] **Extend Docker Compose** — Definition of done: `docker compose up` starts **Postgres** (unchanged behaviour), **Temporal**, and **Temporal Web UI**; image tags **pinned**; health/depends_on as appropriate.
2. [x] **Document host connectivity** — Definition of done: README (or spec cross-link): Web UI base URL, example `JOBHOUND_TEMPORAL_ADDRESS` (or chosen name per contract) for gRPC from host, aligned with Compose port mappings.

## C. Dependencies & internal package

1. [x] **Add Temporal SDK to Go modules** — Definition of done: `go.mod` includes `go.temporal.io/sdk` (and test suite if used); `go mod tidy` clean.
2. [x] **Implement reference workflow + activity** — Definition of done: under `internal/reference/workflows/` (+ `activities/`, per plan D1); **one** workflow calling **one** activity; **deterministic** result; **no** GORM/Postgres in call path; **explicit** timeout/retry options (conservative).
3. [x] **Keep domain clean** — Definition of done: `internal/domain` has **no** Temporal SDK imports; grep / CI posture as team prefers.

## D. Worker binary

1. [x] **`cmd/worker`** — Definition of done: reads Temporal connection settings from **environment**; registers reference workflow and activity on task queue **`jobhound`**; blocks until shutdown; namespace **`default`** when connecting.
2. [x] **Makefile / build** — Definition of done: `make build` (or documented target) produces worker binary alongside agent if applicable; help text documents how to run worker against Compose.

## E. Client path & manual verification

1. [x] **Start workflow programmatically** — Definition of done: test **and/or** dev-only entrypoint (per plan D3) executes workflow on **`jobhound`** / **`default`** and completes successfully when worker is running.
2. [x] **UI visibility (manual checklist)** — Definition of done: documented steps to see workflow + ≥1 completed activity in Temporal Web UI (acceptance criterion 3).

## F. Tests

1. [x] **Automated Temporal test without Docker** — Definition of done: `go test ./...` runs a test that executes reference workflow logic via **in-memory** Temporal test environment **or** equivalent; no Compose required.
2. [x] **Optional: integration test** — Definition of done (if chosen): `//go:build integration` test against real Temporal in Compose; documented in Makefile/README with `make test-integration` or `go test -tags=integration ./...`.

## G. Optional / deferred (do not block `003` closure)

1. [x] **Shared retry/timeout helpers** — Optional; add when second workflow lands.
2. [x] **Production GCP worker runbook** — Out of scope for `003`; high-level env note in README is enough.
