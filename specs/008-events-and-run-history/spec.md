# Feature: Scheduled auto-refresh and run history

**Feature Branch**: `008-events-and-run-history`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — **§3** (delta / “pull new”, not full universe re-import by default), **§4** (stage-3 cap, ordering, eligible pool apply to **each** automatic execution), **§5** (no filter snapshot versioning; a scheduled tick always uses **current** slot parameters), **§8–§9** (this epic is **post-core**—after a working vertical with manual/API triggers), **§10** (schedules and history must be **`slot_id`**-scoped and respect reset semantics).

## Goal

Add **scheduled** execution for a **search slot**: at a configured cadence (e.g. hourly or every few hours), run **incremental** stage-1 ingest for that slot’s bound sources (**`006`**) and then **recompute** stages 2–3 against the **current** stage-1 pool and filters (**`004`**, **`007`**). Each tick should align with the product rule “same slot, same immutable broad string, **append** new listings” (draft §3), not a default full re-crawl of history.

Persist an **append-only run history** (and minimal schedule metadata) so operators and future UI can see per-tick outcomes—**found N**, **found 0**, **errors**—without storing a versioned history of filter configurations (draft §5).

## Clarifications (vs older “Event = saved search” wording)

- A **schedule** is **not** a second copy of broad keywords, sources, or stage-2/3 parameters. Those belong to the **slot** (and related persistence as **`002`/`011`** define). This epic stores **when** (and whether) to run the engine for a given **`slot_id`** (+ reserved **`user_id`**), plus **execution** history rows.
- **`008`** complements **`009`** (on-demand / API-triggered workflow): same pipeline semantics where possible; **Temporal** Schedule or an external trigger (e.g. GCP Cloud Scheduler) is an implementation choice documented in the contract.
- Each automatic execution creates or continues work that remains subject to **`007`** caps and idempotency **per pipeline-run execution** (draft §4); history rows may reference **`pipeline_run_id`** when a distinct run is recorded for that tick.

## Scope

- Durable **schedule definition** (minimal): **`slot_id`**, optional **`user_id`**, cadence / cron or equivalent, enabled flag, optional mapping to a Temporal Schedule ID or external job name.
- **Run history**: append-only records per execution—timestamps, coarse outcome (success / partial / failure), summary counts where cheap, nullable error text, nullable **`temporal_workflow_run_id`**, optional **`pipeline_run_id`** FK aligned with **`007`**.
- **Incremental ingest** rules, Redis lock by **`source_id`**, and watermarks remain **`006`**; this epic **consumes** those contracts, it does not redefine them.

## Out of scope

- **Telegram** notifications (**`010`**).
- **Full public HTTP CRUD** for schedules and history (**`011`**); thin internal hooks from worker/API are acceptable if needed for registration.
- **Versioned filter snapshots** or “what the filters were last Tuesday” (explicitly out of product scope until a later epic—draft §5).
- Replacing or merging the **manual** workflow spec (**`009`**); only **parity** of engine behavior is assumed.

## Dependencies

- **`003`** Temporal worker and schedules/triggers.  
- **`004`** stage logic inside activities.  
- **`006`** ingest, delta, coordination.  
- **`007`** `pipeline_run` / cap / per-job statuses for each execution.  
- **`002`** migrations and GORM patterns.  

## Implementation tasks

Concrete backlog (including DB stubs deferred from **`002`** where still applicable): see [`tasks.md`](./tasks.md). Schema details: [`contracts/scheduled-runs-and-history.md`](./contracts/scheduled-runs-and-history.md).

## Local / Docker

Temporal + Postgres from Compose; worker owns schedule registration or reacts to external triggers. Production may use Cloud Scheduler calling a small endpoint or starting workflows—exact wiring is **`011`/ops**, not required to block schema/history design here.
