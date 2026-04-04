# Contract: HTTP public API (UI-facing)

**Feature**: `009-http-public-api`  
**Status**: **Draft** — normative detail for implementers; [`spec.md`](../spec.md) remains the product source of truth; resolve conflicts by updating **both** deliberately.

**Consumers**: `cmd/api`, browser UI, tests. **Non-consumers**: `cmd/agent` debug HTTP (dev-only; different surface).

## 1. Purpose

Freeze **routes**, **status codes**, **error `code` strings**, **JSON field names**, and **mapping to `008`** (`ManualSlotRunWorkflow`, run kinds — see [`../../008-manual-search-workflow/contracts/manual-workflow.md`](../../008-manual-search-workflow/contracts/manual-workflow.md)) so implementation matches **`spec.md`** without re-deriving rules from prose.

## 2. Global rules

| Topic | Rule |
|-------|------|
| Prefix | All paths under **`/api/v1`**. |
| Content-Type | Request and response bodies: **`application/json`**. |
| IDs | **`slot_id`**, **`job_id`**: string UUIDs if DB stores UUIDs. |
| Auth | **None** in MVP; no session headers. |
| CORS | Required; allowed origins from config — [`environment.md`](./environment.md). |
| Slot cap | **≤ 3** slots (single implicit user). |
| Stage 1 re-run | **No** HTTP path to re-start ingest for an **existing** `slot_id`; only **`POST /slots`** starts stage 1 for a **new** slot. |

### 2.1 Error envelope (all non-2xx JSON errors)

```json
{
  "error": {
    "code": "machine_readable_snake",
    "message": "Human-readable explanation"
  }
}
```

| HTTP | Use when |
|------|----------|
| **400** | Malformed JSON, invalid body/query. |
| **404** | Slot or job not found / not in scope. |
| **409** | Slot cap, stage already running, other business conflict (see §6). |
| **422** | Optional: e.g. `max_jobs` semantically unusable — if unused, document **200**/empty run behavior in code + tests (**[`plan.md`](../plan.md)** D5). |
| **500** | Internal failure; **no** stack traces or secrets in body. |

### 2.2 `StageState` (enum string)

| Value | Meaning |
|-------|---------|
| `idle` | No run in progress for that stage. |
| `running` | Workflow for this stage in progress. |
| `succeeded` | Last run completed successfully. |
| `failed` | Last run failed; stage object may carry `error`. |

**List vs card**: In **`GET /slots`**, each list item’s `stage_1` … `stage_3` may include only **`{ "state": "…" }`**. In **`GET /slots/{slot_id}`** and **`POST /slots`**, each `stage_*` is the **full** object (§4.2).

### 2.3 Stage object (full card)

```json
{
  "state": "running",
  "started_at": "2026-04-04T12:00:05Z",
  "finished_at": null,
  "error": null
}
```

**`error` on `failed`**: pick **one** representation project-wide — either **`{ "code": "…", "message": "…" }`** or a **string** — and use it consistently in **`schema/`** + docs (**[`plan.md`](../plan.md)** D3).

## 3. Route table

| Method | Path | Success | Notes |
|--------|------|---------|--------|
| `GET` | `/api/v1/slots` | **200** | No pagination; max 3 items. |
| `POST` | `/api/v1/slots` | **201** | Starts stage 1 ingest; body **`{ "name": string }`**. |
| `GET` | `/api/v1/slots/{slot_id}` | **200** | Full slot card. |
| `DELETE` | `/api/v1/slots/{slot_id}` | **204** | Hard delete; empty body. |
| `GET` | `/api/v1/profile` | **200** | Global stage-3 profile text. |
| `PUT` | `/api/v1/profile` | **200** | Body **`{ "text": string }`**. |
| `POST` | `/api/v1/slots/{slot_id}/stages/2/run` | **202** | Body **`include`**, **`exclude`** string arrays (required keys). |
| `POST` | `/api/v1/slots/{slot_id}/stages/3/run` | **202** | Body **`max_jobs`** int **1–100**. |
| `GET` | `/api/v1/slots/{slot_id}/stages/1/jobs` | **200** | Query: **`page`**, **`limit`**. |
| `GET` | `/api/v1/slots/{slot_id}/stages/2/jobs` | **200** | Optional **`bucket=passed|failed`**. |
| `GET` | `/api/v1/slots/{slot_id}/stages/3/jobs` | **200** | Optional **`bucket`**. |
| `PATCH` | `/api/v1/slots/{slot_id}/stages/{stage}/jobs/{job_id}` | **200** or **204** | **`stage`** ∈ **`2`**, **`3`** only. |

**There is no** `POST /api/v1/slots/{slot_id}/stages/1/run`.

## 4. Response shapes (frozen field names)

### 4.1 `GET /api/v1/slots` — **200**

Top-level: **`slots`** (array).

List item:

| Field | Type | Notes |
|-------|------|--------|
| `id` | string | `slot_id` |
| `name` | string | Broad string from client |
| `created_at` | string (RFC3339) | |
| `stage_1` | object | At least **`state`**. |
| `stage_2` | object | At least **`state`**. |
| `stage_3` | object | At least **`state`**. |

### 4.2 `GET /api/v1/slots/{slot_id}` and `POST /api/v1/slots` — **200** / **201**

| Field | Type |
|-------|------|
| `id` | string |
| `name` | string |
| `created_at` | string (RFC3339) |
| `stage_1` | full stage object (§2.3) |
| `stage_2` | full stage object |
| `stage_3` | full stage object |

**No embedded job arrays** on the slot resource.

### 4.3 `POST /api/v1/slots` — **409** `slot_limit_reached`

**Frozen shape** (machine-parsable UI):

```json
{
  "error": {
    "code": "slot_limit_reached",
    "message": "…"
  },
  "limit": 3
}
```

Top-level **`limit`** is **required** for this response. Mirror in **`schema/`** and tests.

### 4.4 Profile — `GET`/`PUT` **200**

| Field | Type |
|-------|------|
| `text` | string |
| `updated_at` | string (RFC3339) |

### 4.5 Stage run accepted — **202**

```json
{
  "slot_id": "uuid",
  "stage": 2
}
```

(`stage` is **2** or **3**.)

### 4.6 Job list — **200**

| Field | Type |
|-------|------|
| `items` | array of job row |
| `page` | int (from 1) |
| `limit` | int |
| `total` | int (total matching query) |

Job row (minimum per [`spec.md`](../spec.md)):

| Field | Type | Notes |
|-------|------|--------|
| `job_id` | string | |
| `title` | string | |
| `company` | string | |
| `source_id` | string | |
| `apply_url` | string | |
| `first_seen_at` | string (RFC3339) | |
| `posted_at` | string (RFC3339) | |
| `stage_3_rationale` | string or null | Fill when LLM text exists; stages 1–2: **`null`** or omit — **one style** (**[`plan.md`](../plan.md)** D4). |

**Sort**: **`posted_at` DESC**, then **`job_id` ASC** (stable).  
**Pagination**: **`page`** (from **1**), **`limit`** (default + **max 100** — document default in code, e.g. 50).

### 4.7 `PATCH …/jobs/{job_id}` — body

```json
{
  "bucket": "passed"
}
```

Allowed: **`passed`**, **`failed`**.

## 5. MVP business rules (HTTP-visible)

| Rule | Behavior |
|------|----------|
| **`POST …/stages/2/run`** | Before recompute: **delete** all stage **2 and 3** data for the slot (incl. manual marks in that scope); recompute **stage 2 only**; **do not** start stage 3 in the same request. |
| **`POST …/stages/3/run`** | Delete **stage 3 only**; recompute from current stage-2 outcome. |
| **Concurrency** | At most **one** active run **per stage** (2 or 3) per `slot_id`. Second `POST` while that stage is **`running`** → **409**, code **`stage_already_running`**. |
| **Effective `max_jobs`** | **min**(HTTP request **1–100**, pipeline policy from **`007`** / config). |

**Invalidation storage**: follow [`../../008-manual-search-workflow/contracts/filter-invalidation.md`](../../008-manual-search-workflow/contracts/filter-invalidation.md) — stage-2 run uses **`InvalidateStage2And3SnapshotsForSlot`** (or equivalent) before persisting new filters; profile **`PUT`** implies stage-3 invalidation per product draft §5 (engine); client calls **`POST …/stages/3/run`** when ready.

## 6. Machine-readable `error.code` values (minimum set)

| `code` | Typical HTTP | When |
|--------|--------------|------|
| `slot_limit_reached` | **409** | More than 3 slots on create. |
| `stage_already_running` | **409** | Stage 2 or 3 workflow already running for slot. |
| *(validation)* | **400** | Invalid JSON, bad types, out-of-range `max_jobs`, invalid `bucket`, invalid path `stage`. |

Add more stable codes as needed (e.g. `invalid_page`); prefer reusing **400** with clear `message` for simple validation.

## 7. Mapping to `008` (Temporal)

| HTTP | Run kind (`manual-workflow.md` §3) | Notes |
|------|-----------------------------------|--------|
| **`POST /api/v1/slots`** | **`INGEST_SOURCES`** | Parallel ingest per configured source; **`name`** only from client; sources from backend config. |
| **`POST …/stages/2/run`** | **`PIPELINE_STAGE2`** | Pass **`include`**, **`exclude`** into workflow input per `internal/manual/schema`. |
| **`POST …/stages/3/run`** | **`PIPELINE_STAGE3`** | Pass **`max_jobs`** (effective batch after min with policy). |

**Workflow**: start **`ManualSlotRunWorkflow`** (registered name per [`manual-workflow.md`](../../008-manual-search-workflow/contracts/manual-workflow.md) §4.3) with the appropriate **`RunKind`** and fields — **do not** use **`PIPELINE_STAGE2_THEN_STAGE3`** for the default browser flow (separate HTTP calls).

**Stage state**: handlers / `impl` derive **`running`** / **`succeeded`** / **`failed`** from Temporal workflow status + optional DB mirrors as implemented; **`spec.md`** is the UX truth.

## 8. Related

- [`../spec.md`](../spec.md)  
- [`../plan.md`](../plan.md), [`../tasks.md`](../tasks.md)  
- [`../../008-manual-search-workflow/contracts/manual-workflow.md`](../../008-manual-search-workflow/contracts/manual-workflow.md)  
- [`../../008-manual-search-workflow/contracts/filter-invalidation.md`](../../008-manual-search-workflow/contracts/filter-invalidation.md)  
- [`./environment.md`](./environment.md)
