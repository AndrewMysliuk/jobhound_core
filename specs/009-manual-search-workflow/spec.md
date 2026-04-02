# Feature: Manual search workflow

**Feature Branch**: `009-manual-search-workflow`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — **§2** (search **slot** as unit of work; **`slot_id`**; schema reserves **`user_id`**), **§3** (first ingest vs later **“pull new”** / incremental path—exact trigger shape lives here and in **`011`**), **§4** (stage-3 **cap, ordering, eligible pool, idempotency** apply to **each** manual execution), **§5** (filter/profile **reset wipes** dependent outcomes **before** or **as part of** the same user action; manual workflows **recompute** from PostgreSQL—**no** implicit full re-crawl when only filters changed), **§9** (core vertical: manual/API triggers before schedules **`008`**).

## Goal

Provide **on-demand** orchestration (Temporal workflow **preferred** for parity with **`008`**) that runs the **same pipeline semantics** as scheduled execution: **slot-scoped** stage-1 ingest where requested, then **local** stages 2–3 per **`004`** / **`007`**, driven by an **API**, **CLI**, or internal call—not by a cron.

The outcome is a **stable request/response contract** (DTOs live in module `schema/` when implemented) so **`011`** can expose thin HTTP without redefining behavior.

## Clarifications (vs older “cache and/or fresh fetch” wording)

- **“Manual”** means **user- or operator-initiated**, not **unscheduled** ad hoc SQL. It does **not** bypass **`006`** Redis lock/cooldown or watermark rules; every stage-1 touch for a source uses the **same** coordination path as other slots (draft §3).
- **Fresh fetch** is **not** “re-import the whole universe by default”: later pulls are **incremental / smaller limit** relative to stored state (**`006`**). First successful ingest for a slot still establishes the **immutable** broad keyword string (draft §2).
- **Filter edits** do **not** require re-hitting collectors (draft §5). A manual run after a filter change is either **stage 2+3 only** or **stage 3 only**, depending on what changed—see [contracts/manual-workflow.md](./contracts/manual-workflow.md).
- **`009`** is the **core** way to run the engine before **`008`** exists; **`008`** should reuse activities/workflow steps where possible, differing mainly in **trigger** (schedule vs explicit start).

## Scope

- One or more **Temporal workflows** (or documented equivalent) started with **`slot_id`** (+ reserved **`user_id`** when present) and a **run kind** / payload that selects: stage-1 ingest, stage 2+3 recompute, stage 3 only, and **compound** sequences (e.g. delta ingest then 2 then 3) if the product exposes a single “refresh” action.
- **Activity boundaries** aligned with **`003`**, **`004`**, **`006`**, **`007`**—this epic wires them; it does not redefine stage math or ingest keys.
- **Response shape**: identifiers (`temporal_workflow_run_id`, optional **`pipeline_run_id`**), coarse **counts** (ingest summary, stage-2 split, stage-3 processed vs capped per **`007`**), and **error** summary suitable for UI—details in [contracts/manual-workflow.md](./contracts/manual-workflow.md).

## Out of scope

- **Full public REST** surface and slot CRUD (**`011`**).
- **Schedule definition** and append-only **tick history** (**`008`**)—manual runs may still log to **`pipeline_run`** / job status per **`007`** where that already exists.
- **Telegram** (**`010`**), **observability** beyond what **`012`** adds later.

## Dependencies

- **`003`** Temporal client/worker patterns.  
- **`004`** pure stage logic in activities.  
- **`005`** / **`006`** collectors, normalize, persist, watermarks, Redis **`source_id`** lock.  
- **`007`** caps, ordering, eligible pool, idempotency, **`pipeline_run`** ↔ **slot**.  
- **`002`** migrations as needed for any workflow-specific persistence (prefer reuse of existing run tables).  

## Implementation tasks

Backlog and contract detail: [`tasks.md`](./tasks.md), [`contracts/manual-workflow.md`](./contracts/manual-workflow.md).

## Local / Docker

Start workflows from **`cmd/worker`** tests, **`cmd/agent`** debug hooks, or a minimal internal caller against Compose **Temporal** + **Postgres**; exact HTTP entrypoints are **`011`**.
