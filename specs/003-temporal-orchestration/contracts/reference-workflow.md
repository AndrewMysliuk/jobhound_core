# Contract: reference workflow (v0)

**Feature**: `003-temporal-orchestration`  
**Purpose**: Single **demo** workflow and **one** activity to validate Temporal wiring, registration, and Web UI visibility. **Not** product logic; replace or extend in later specs (`006`, `008`, `009`).

## Runtime identifiers (must match worker and client)

| Item | Value |
|------|--------|
| Namespace | `default` (overridable only via `JOBHOUND_TEMPORAL_NAMESPACE`; default must remain `default` for local) |
| Task queue | `jobhound` (overridable only via `JOBHOUND_TEMPORAL_TASK_QUEUE`; default must remain `jobhound`) |

Worker registration and every `ExecuteWorkflow` (or equivalent) call **must** use the same namespace and task queue for this demo.

## Workflow

| Field | Contract |
|-------|----------|
| **Registered name** | `ReferenceDemoWorkflow` — must match `internal/reference/workflows` (`ReferenceWorkflowName`) and worker registration. |
| **Input** | Single **string** (greeting subject, e.g. a name); empty string is allowed (activity substitutes a default). |
| **Output** | Single **string**: deterministic prefix `demo: ` plus activity result (e.g. `demo: Hello, world!`). |
| **Behaviour** | Invokes **exactly one** activity; no branches required beyond error handling; **no** database or external I/O. |

## Activity

| Field | Contract |
|-------|----------|
| **Registered name** | `ReferenceGreetActivity` — must match `internal/reference/workflows` (`ReferenceActivityName`) and worker registration. |
| **Input** | Same **string** as workflow input (subject/name). |
| **Output** | **String** `Hello, <name>!` with default name `world` when input is empty. |
| **Side effects** | **None** that require Postgres, HTTP, or secrets; pure/deterministic logic only for v0. |

## Timeouts and retries

| Layer | Setting |
|-------|---------|
| Activity `StartToCloseTimeout` | 30 seconds |
| Activity `ScheduleToCloseTimeout` | 1 minute |
| Activity retry | `MaximumAttempts`: 3; `InitialInterval`: 1s; `BackoffCoefficient`: 2 |

Workflow **run** timeout is set by the **client** when starting the workflow (worker binary / tests / dev client), not inside workflow code — keep conservative values there as well.

## Versioning

- **v0** is allowed to change registered names **once** before any production traffic; after first stable release, follow Temporal versioning guidance for workflow changes.

## Change process

When renaming workflow/activity or changing inputs:

1. Update this file and `tasks.md` / `plan.md` if behaviour scope changes.  
2. Update automated tests and README “manual UI check” steps.
