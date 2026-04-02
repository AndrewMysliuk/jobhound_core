# Feature: HTTP public API (UI-facing)

**Feature Branch**: `010-http-public-api`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — **§2** (search **slot**; **`slot_id`**; hard delete; immutable **stage-1** broad string after first successful ingest), **§3** (first ingest vs **delta** refresh—shape of triggers shared with **`009`**), **§4** (stage-2/3 local recompute; stage-3 **cap, ordering, eligible pool**; **“process next batch”**-style actions), **§5** (**reset** wipes when filters/profile change—API must not contradict those semantics), **§6** (MVP **no auth**; **schema reserves `user_id`**), **§7** (**API-first**; UI elsewhere).

## Goal

Expose a **stable HTTP API** (REST or a thin RPC layer—pick one stack-wide) so a **separate** product UI can manage **slots**, **profile**, **pipeline triggers**, and **read** slot-scoped vacancies and outcomes. Behavior matches **`009`** workflows and storage in **`002` / `004` / `006` / `007`**; **`010`** is routing, validation, and DTO mapping—not a second definition of stage or reset rules.

Implementation home: thin **`cmd/api`** composition (stdlib `net/http` or chosen router) wiring module **`handlers/`** + **`schema/`** per project conventions—parallel to **`cmd/agent`** debug HTTP, not a fat `main`.

## Scope (MVP API surface — conceptual)

Group into resources; exact paths and method names are implementation detail, but capabilities should exist:

- **Slots**: create (broad keyword string + bound sources as product allows), read, list; enforce **slot limit** (product constant **3–5** per user—MVP may treat “one user” as implicit). **Delete** = **hard delete** all slot-tied rows (draft §2). **Stage-1 broad string** immutable after **first successful** stage-1 ingest for that slot (reject in-place change; user creates another slot).
- **Profile**: read/update **free-text** profile used by stage 3; after profile save, API should **expose or trigger** stage-3-only recompute (draft §4–§5) so clients are not stuck with stale LLM outcomes.
- **Filters / stage parameters**: update **stage-2** and/or **stage-3** (non–stage-1) inputs. Mutations must apply **§5** reset semantics (wipe dependent outcomes, including **manual marks** in the wiped scope) before or as part of the same logical operation; then client or server follows with the appropriate **`009`** run kind (2+3 vs 3-only).
- **Runs / actions**: start or query **user-initiated** pipeline work: stage-1 ingest (full first path vs **delta** later), stage 2+3 recompute, stage 3 only, **compound** “pull new then 2 then 3” if the product exposes one button, and **explicit “process next batch”** for LLM cap backlog (**`007`**). Request/response fields align with **`009`** contracts (workflow id, optional **`pipeline_run_id`**, coarse counts, errors)—**`010`** does not invent parallel payload semantics.
- **Reads**: paginated (or cursor) listing of stage-1 pool, stage-2 passed/failed buckets, stage-3 results, and **manual marks** within the **coarse** passed/failed model (draft §1)—no large ad-hoc status matrix for MVP.
- **Cross-cutting**: JSON request/response shapes; **CORS** where browser clients are expected; deployment notes for GCP when applicable. **Optional OpenAPI** in the same epic or a follow-up—nice-to-have, not a blocker for first vertical.

## Out of scope

- **Frontend** implementation (any repo or folder).
- **Primary** product features deferred by draft §8: **scheduled** auto-runs (**`008`**)—API may later add endpoints that start the same workflows, but MVP vertical is **manual/API-triggered** first.
- **Full auth** (sessions, JWT, OAuth) for MVP; reserve **`user_id`** in payloads/schema where ownership will matter later.
- **Filter snapshot history** / “what filters were last Tuesday” (draft §5).

## Dependencies

- **`002`** Postgres schema and migrations (**slot**, **user** columns, job linkage).  
- **`003`** Temporal client from API process to start **`009`** workflows.  
- **`004` / `006` / `007`** semantics and activity boundaries (API does not re-spec).  
- **`009`** manual workflow contracts—**source of truth** for run kinds and response DTOs **`010`** mirrors.  
- **`008`**, **`011`**—only when extending the API for schedules or ops-focused telemetry surfaces.

## Local / Docker

API server runs on host or Compose service with **`JOBHOUND_DATABASE_URL`**, **`JOBHOUND_TEMPORAL_ADDRESS`**, and other keys documented in **`specs/*/contracts/environment.md`** / **`internal/config`** as implementations land; same Postgres + Temporal as **`cmd/worker`**.

## Related

- [`../000-epic-overview/spec.md`](../000-epic-overview/spec.md) — epic index and phasing.  
- [`../009-manual-search-workflow/spec.md`](../009-manual-search-workflow/spec.md) — workflow payloads and parity with schedules later.
