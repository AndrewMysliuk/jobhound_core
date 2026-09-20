# {{FEATURE_TITLE}} — concept

> Template: product and architecture spec. No Go types or HTTP bodies — behavior, boundaries, modules only.
> Pair with frontend spec if applicable: `jobhound_frontend/docs/{{feature_slug}}/`

## Why

**Problem today:** [what is wrong with the current solution / why this feature is needed]

**New solution:**

1. [key flow, step 1]
2. [key flow, step 2]
3. [what changes at runtime — persist, Temporal, collectors, LLM]

Product context: [who, in which scenario]. Primary entity: **{{PRIMARY_ENTITY}}**.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| HTTP | **`net/http`** | `internal/publicapi` and/or `internal/{{module}}/handlers` |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/{{module}}/storage` |
| Orchestration | **Temporal** | `internal/{{module}}/workflows` |
| Ingest | **[http / goquery / go-rod]** | `internal/collectors` |
| Scoring | **Claude API** | `internal/llm` |

[What does **not** go through these layers — debug HTTP, leftover in-process path, etc.]

## {{PRIMARY_ENTITY}}

**Input:**

- **[field 1]** — [purpose].
- **[field 2]** — [purpose].

**Lifecycle / creation:** [how the entity appears, who writes it, how many steps].

**Review / editing:** [what can change; what is frozen after lock / success].

Statuses: **{{STATUS_A}}** → **{{STATUS_B}}**. [Rule: one table / two / immutable after X].

**After {{STATUS_B}}** [what is frozen]. [How to start over — new entity / new slot / new run].

## Runtime (if applicable)

**Start:** [explicit HTTP, Temporal parent, collector fetch — what is written first].

**Main flow:** [what drives the process — stages, watermarks, run kinds].

**UI-only fields:** [fields for the frontend only; engine does **not** rely on them].

**Outcomes** (if there are terminal states):

- **{{OUTCOME_1}}** — [when];
- **{{OUTCOME_2}}** — [when];
- **{{OUTCOME_3}}** — [technical failure, etc.].

**Adjacent modules:** [who reads the result, not part of the feature core].

**Progress:** [where progress is stored — run row, stage status, not free-form logs]. [What counts as a bug].

## One {{UNIT_NAME}} (core loop)

[e.g. slot run, ingest tick, score batch.] Sequential chain; **state is committed only after full success** unless the spec says otherwise.

1. [step 1 — load, fetch, filter]
2. **[{{STEP_2_NAME}}]** — [rule / LLM; output; what is **not** in the output]
3. **[{{STEP_3_NAME}}]** — [persist / next stage]
4. Persist; [when counters / watermarks increment]

### Failure at any step

[Failure behavior — Temporal retry, mark run failed, do not confuse with business skip]

- [entity / stage unchanged]
- [retry vs terminal fail]
- in logs: `failure_stage` = `{{STAGE_A}}` | `{{STAGE_B}}` | ...

## {{DOMAIN_RULES_TITLE}}

| Situation | Behavior |
| --------- | -------- |
| [situation 1] | [action / transition] |
| [situation 2] | [action / transition] |
| [edge case] | [action / transition] |
| [409 / conflict] | [reject / no second start] |

**Hard rules:** [e.g. at most one active run per stage per slot; no Temporal SDK in `impl/`].

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/{{MODULE_1}}`** | [use cases, storage] |
| **`internal/{{MODULE_2}}`** | [workflows / activities] |
| **`internal/publicapi`** | [HTTP surface, if this slice has one] |
| **`internal/{{LEGACY}}`** | [legacy — **do not touch** / parallel path] |

## Layers

- **Saved** — [what does not change during a run]
- **Run / session** — [progress, stage status, outcome]
- **Wire** — [HTTP / workflow payloads; live in module `schema/`]

## Implementation order

1. [schema + contract]
2. [storage / migration]
3. [impl]
4. [workflows if Temporal]
5. [handlers + JSON Schema]
6. [handler / impl / storage tests]

Early check: [minimal smoke path — one handler or one workflow with mocks].

## Out of scope (concept level)

- [frontend behavior — link to frontend concept instead]
- [unrelated module refactors]
- [scheduled auto-refresh, auth, third-party push — unless this slice is that]
