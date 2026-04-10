# Feature: HTTP public API (UI-facing)

**Feature Branch**: `009-http-public-api`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-10  
**Status**: Draft  

**Product narrative**: [`../000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — search **slot** as the unit of work, **§2** (hard delete; immutable broad string after first successful ingest — different broad → new slot), **§5** (resets when stage-2/3 rules or profile change), **§6** (MVP without auth; **`user_id`** reserved), **§7** (API-first).

**Normative for this epic**: this document is the **source of truth for public HTTP** in MVP. If **`008`** or other epics disagree with the triggers and resets below, **align them to this spec** (or change this spec deliberately via product decision).

## Goal

A stable **JSON HTTP API** for a **browser** client (separate UI): slots, stage-3 profile, **stage 2 and 3** run triggers, paginated job lists per stage, and manual bucket corrections. Implementation: thin **`cmd/api`** (stdlib `net/http` or a router—project choice), modules with **`handlers/`** + **`schema/`** per repo conventions.

Stage math, persistence, and Temporal remain **`002` / `004` / `006` / `007` / `008`**; **`009`** defines the **HTTP contract** and mapping to run kinds / activities without re-specifying stage logic.

## MVP rules (normative)

1. **Stage 1 (ingest)** for an **existing** `slot_id` is **not** restarted via the API. The only stage-1 start is **slot creation** (`POST /api/v1/slots`). A different broad string / “new stage 1” → **new slot**.
2. **`POST …/stages/2/run`**: before recompute, **delete** all stage **2 and 3** data (including manual marks in that scope); recompute **stage 2 only** from the current stage-1 pool using body **`include` / `exclude`**. Stage **3** is **not** started by this request.
3. **`POST …/stages/3/run`**: delete **stage 3 only**; recompute from the current stage-2 outcome. Body includes **`max_jobs`** (how many jobs from stage-2 **passed** to score in **this** run)—see below.
4. **Concurrency guard**: at most **one** active run **per stage** (2 or 3) for a given `slot_id`. A second `POST` while that stage is **`running`** → **`409`** with code `stage_already_running`.
5. **Slot cap**: at most **3** slots (MVP, single implicit user). Exceeding → **`409`**, code `slot_limit_reached`.
6. **Ingest sources**: the client sends only **`name`** (non-empty trimmed string). That value is stored on the slot **and** used as the **stage-1 per-source search keyword**: each backend-configured collector runs its **native** search for that string when supported (`005` **`SlotSearchFetcher`** — see **`006`**). **All** sources are the **backend-configured** set; there is **no** per-slot source filter in the API.

## Conventions

- **Prefix**: all routes under **`/api/v1`**.
- **Format**: `Content-Type: application/json`; bodies and responses are JSON.
- **IDs**: `slot_id`, `job_id` as string UUIDs if stored as UUIDs in the DB.
- **CORS**: required for browser clients. Allowed origins are **configuration** (list of strings), extendable without code changes. Env: **`JOBHOUND_API_CORS_ORIGINS`** (e.g. comma-separated); document default **`http://localhost:5173,http://localhost:3000`** for local dev.
- **Auth**: none in MVP; no session headers. DB schema reserves **`user_id`** for later.

### Errors (single envelope)

```json
{
  "error": {
    "code": "machine_readable_snake",
    "message": "Human-readable explanation"
  }
}
```

**HTTP**: **`400`** invalid body/params; **`404`** missing resource; **`409`** conflict (slot cap, stage already running, business rule); **`422`** semantically impossible (optional, e.g. `max_jobs` vs no candidates—implementation choice); **`500`** internal error without leaking secrets.

### `StageState` (enum)

State of the **last finished or current** run for that stage on the slot:

| Value       | Meaning |
|------------|---------|
| `idle`     | No run in progress; not `running`. |
| `running`  | A workflow for this stage is in progress. |
| `succeeded`| Last run completed successfully. |
| `failed`   | Last run failed; see `error` on the stage object. |

In **`GET /slots`**, list items may include only **`state`** under `stage_1` … `stage_3` (omit timestamps/errors) to save payload.

### Stage object in `GET /slots/{slot_id}`

```json
{
  "state": "running",
  "started_at": "2026-04-04T12:00:05Z",
  "finished_at": null,
  "error": null
}
```

On `failed`, `error` is either `{ "code": "…", "message": "…" }` or a string—**pick one style** in implementation and keep it consistent.

**Important**: the slot response has **no** embedded job lists. Listing jobs is only via the **GET jobs** endpoints below.

---

## Resources and methods

### `GET /api/v1/slots`

All slots (no pagination; max 3).

**Response `200`**

```json
{
  "slots": [
    {
      "id": "uuid",
      "name": "golang backend",
      "created_at": "2026-04-04T12:00:00Z",
      "stage_1": { "state": "succeeded" },
      "stage_2": { "state": "idle" },
      "stage_3": { "state": "idle" }
    }
  ]
}
```

---

### `POST /api/v1/slots`

Creates a slot and **immediately** starts **stage 1** (ingest).

**Body**

```json
{
  "name": "golang backend"
}
```

**Semantics**: **`name`** is the slot’s display label **and** the **search query** for stage-1 fetches: it is copied into **`ManualSlotRunWorkflowInput.SlotSearchQuery`** and then into each child **`IngestSourceInput.SlotSearchQuery`** so boards filter listings by this string (per-source wire details in **`005`** `collector.md`). It is **not** only metadata.

**Response `201`**: same shape as **`GET /api/v1/slots/{slot_id}`** (full `stage_*` objects with timestamps as available).

**`409`**: `slot_limit_reached`, include `limit`: `3`.

---

### `GET /api/v1/slots/{slot_id}`

Slot card for **reload** and polling.

**Response `200`**

```json
{
  "id": "uuid",
  "name": "golang backend",
  "created_at": "2026-04-04T12:00:00Z",
  "stage_1": {
    "state": "succeeded",
    "started_at": "…",
    "finished_at": "…",
    "error": null
  },
  "stage_2": { "state": "idle", "started_at": null, "finished_at": null, "error": null },
  "stage_3": { "state": "idle", "started_at": null, "finished_at": null, "error": null }
}
```

**`404`**: slot not found.

---

### `DELETE /api/v1/slots/{slot_id}`

Hard-deletes the slot and all related data (draft **§2**).

**Response `204`** empty body. **`404`** if missing.

---

### `GET /api/v1/profile` / `PUT /api/v1/profile`

Free-text profile for stage 3.

**GET `200`**

```json
{
  "text": "…",
  "updated_at": "2026-04-04T11:00:00Z"
}
```

**PUT** body:

```json
{
  "text": "…"
}
```

**PUT `200`**: same as GET. Per draft **§5**, profile change invalidates stage 3 on the engine side; the client calls **`POST …/stages/3/run`** when it wants a fresh LLM pass.

---

### `POST /api/v1/slots/{slot_id}/stages/2/run`

Runs **stage 2 only** after wiping stages **2 and 3**.

**Body (required)**

```json
{
  "include": ["golang", "kubernetes"],
  "exclude": ["php"]
}
```

Arrays of strings; whether both may be empty is an implementation choice (recommendation: allow empty = “no string filters” if that matches **`004`**).

**Response `202`**

```json
{
  "slot_id": "uuid",
  "stage": 2
}
```

**`409`**: `stage_already_running` for stage 2 on this slot.

**`008` mapping**: run kind **`PIPELINE_STAGE2`** (no stage 3 in the same user action). Invalidation per **`008` / `filter-invalidation.md`** for stage-2 rule changes.

---

### `POST /api/v1/slots/{slot_id}/stages/3/run`

Runs stage 3 after wiping **stage 3** snapshots only.

**Body (required)**

```json
{
  "max_jobs": 20
}
```

- **`max_jobs`**: integer **≥ 1**, **≤ 100** (HTTP upper bound; **`007`** may cap further—the **effective** batch size is the **minimum** of policy and request).
- Required so clients can control cost and tests.

**Response `202`**: `{ "slot_id": "…", "stage": 3 }`.

**`409`**: `stage_already_running` for stage 3.

**`422`**: optional if `max_jobs` is incompatible with zero candidates—otherwise treat as succeeded with zero scored.

**`008` mapping**: **`PIPELINE_STAGE3`** with batch size from **`max_jobs`** (details in **`007`**).

---

### No `POST …/stages/1/run`

There is **no** endpoint to re-run ingest for an existing `slot_id`. Adding one requires changing this spec.

---

### Job lists (pagination)

Query: **`page`** (from 1), **`limit`** (default and **max 100**).

**Sort order**: **`posted_at` descending**, then **`job_id` ascending** (stable).

**Outcome filter** (stages **2 and 3** only): optional query **`bucket`**.

- **Omit `bucket`**: return a **single** list for that stage (all jobs that participate in that stage’s current model), **without** splitting passed/failed in the response; same sort order.
- **`bucket=passed`** or **`bucket=failed`**: return only that branch.

**`GET /api/v1/slots/{slot_id}/stages/1/jobs?page=1&limit=50`** — stage-1 pool.

**`GET /api/v1/slots/{slot_id}/stages/2/jobs?bucket=passed&page=1&limit=50`**  
**`GET /api/v1/slots/{slot_id}/stages/2/jobs?bucket=failed&page=1&limit=50`**  
**`GET /api/v1/slots/{slot_id}/stages/2/jobs?page=1&limit=50`** — no `bucket`, full stage-2 list.

**`GET /api/v1/slots/{slot_id}/stages/3/jobs?...`** — same for stage 3.

**Response `200`**

```json
{
  "items": [
    {
      "job_id": "uuid",
      "title": "…",
      "company": "…",
      "source_id": "…",
      "apply_url": "https://…",
      "first_seen_at": "2026-04-01T10:00:00Z",
      "posted_at": "2026-03-28T00:00:00Z",
      "stage_3_rationale": null
    }
  ],
  "page": 1,
  "limit": 50,
  "total": 134
}
```

**`stage_3_rationale`**: fill when LLM text exists on stage 3; for stages 1–2 use `null` or omit—one style project-wide.

**`404`**: slot not found.

---

### Manual correction (bucket change)

**`PATCH /api/v1/slots/{slot_id}/stages/{stage}/jobs/{job_id}`**

where **`stage`** is **`2`** or **`3`** (stage 1 later if needed).

**Body**

```json
{
  "bucket": "passed"
}
```

or `"failed"`. Reclassifies the outcome for this slot–job pair in the coarse passed/failed model (draft **§1**).

**Response `200`**: row DTO (may match list item) or **`204`**. **`404`** if slot or job is not in scope for that stage.

On snapshot **reset** per **§5**, manual marks in the wiped scope are cleared with automated results (same as product draft).

---

## Out of scope (MVP)

- Frontend implementation.
- Per-run **history** or audit feeds (current state only via **`GET /slots/{id}`**).
- Repeat **stage-1 ingest** on the same slot; **delta / pull-new** on the same slot is **out of this HTTP MVP** until product adds it.
- Full authentication.
- OpenAPI—nice to add next, not a blocker.

## Dependencies

- **`002`**, **`003`**, **`004`**, **`006`**, **`007`**, **`008`** — as before; map HTTP → run kinds per this spec (**no** POST stage 1 for existing slots; **`PIPELINE_STAGE2`** without auto stage 3 in one user action).

## Local / Docker

**`JOBHOUND_DATABASE_URL`**, **`JOBHOUND_TEMPORAL_ADDRESS`**, **`JOBHOUND_API_CORS_ORIGINS`** (and API bind port if needed) in **`internal/config`** and **`specs/*/contracts/environment.md`** as implemented.

## Related

- [`plan.md`](./plan.md), [`tasks.md`](./tasks.md), [`checklists/requirements.md`](./checklists/requirements.md)  
- [`contracts/http-public-api.md`](./contracts/http-public-api.md) (routes, error codes, JSON fields, Temporal mapping), [`contracts/environment.md`](./contracts/environment.md)  
- [`../000-epic-overview/spec.md`](../000-epic-overview/spec.md)  
- [`../008-manual-search-workflow/spec.md`](../008-manual-search-workflow/spec.md) and [`contracts/manual-workflow.md`](../008-manual-search-workflow/contracts/manual-workflow.md) — aligned with this document for HTTP triggers and stage-3 batch sizing.
