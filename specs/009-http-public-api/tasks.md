# Tasks: HTTP public API (UI-facing)

**Input**: `spec.md`, `plan.md`, [`contracts/http-public-api.md`](./contracts/http-public-api.md), [`contracts/environment.md`](./contracts/environment.md), [`checklists/requirements.md`](./checklists/requirements.md)  
**Depends on**: `002` (Postgres migrations / models), `003` (Temporal), `004`–`007` (stage semantics, pipeline runs), **`008`** (manual slot workflow, run kinds **`PIPELINE_STAGE2`** / **`PIPELINE_STAGE3`**, [`filter-invalidation.md`](../008-manual-search-workflow/contracts/filter-invalidation.md))  
**Tests**: Handler tests with **`net/http/httptest`** and mocked **`impl`/Temporal client** where needed; default **`go test ./...`** without mandatory Docker; optional `//go:build integration` against Compose + **`bin/worker`**.

## Implementation order

| Order | Section | Rationale |
|-------|---------|-----------|
| 1 | [A](#a-config--environment-contract) | CORS and bind settings are prerequisites for browser clients and local Docker. |
| 2 | [B](#b-cmdapi-skeleton) | Establishes binary, lifecycle, and route registration pattern. |
| 3 | [C](#c-shared-http-plumbing) | Single error envelope and JSON helpers avoid drift across routes. |
| 4 | [D](#d-schema-dtos) | Freeze request/response shapes before handlers. |
| 5 | [E](#e-slots-endpoints) | Core resource; stage-1 start only on **`POST /slots`**. |
| 6 | [F](#f-profile-endpoints) | Isolated CRUD; informs stage-3 UX. |
| 7 | [G](#g-stage-run-endpoints) | Temporal integration + **409** concurrency + mapping to **`008`**. |
| 8 | [H](#h-job-list-endpoints) | Depends on storage queries for slot-scoped lists and pagination. |
| 9 | [I](#i-manual-bucket-patch) | Writes on top of existing stage-2/3 persistence model. |
| 10 | [J](#j-quality-gates) | Release checks. |

---

## A. Config & environment contract

1. [x] **API config keys** — Definition of done: **`JOBHOUND_API_CORS_ORIGINS`** (comma-separated list; default documented for local dev **`http://localhost:5173,http://localhost:3000`** per spec) and listen **host/port** (or equivalent single **`JOBHOUND_API_LISTEN`**) parsed only in **`internal/config`**; **`cmd/api`** receives typed struct.  
2. [x] **`contracts/environment.md` vs code** — Definition of done: table in [`contracts/environment.md`](./contracts/environment.md) matches **`internal/config`** (names, defaults, semantics); checklist [`checklists/requirements.md`](./checklists/requirements.md) env item can be ticked.

## B. `cmd/api` skeleton

1. [x] **Binary & Makefile** — Definition of done: **`make build`** produces **`bin/api`** (or agreed name) alongside agent/worker; **`cmd/api`** is composition only (config load, DB, Temporal client wiring, handler mount).  
2. [x] **Route registry** — Definition of done: **`NewHTTPHandler(...)`** + **`registerRoutes()`** pattern aligned with **`internal/collectors/handlers/debughttp`**, with a one-line package comment that this is the **product** API.

## C. Shared HTTP plumbing

1. [x] **Error envelope** — Definition of done: helper writes **`{"error":{"code","message"}}`**; maps **400 / 404 / 409 / 422 / 500** per **`spec.md`**; no secret leakage on **500**.  
2. [x] **CORS** — Definition of done: preflight and response headers honor configured origins; behavior covered by at least one test or documented manual check.  
3. [x] **JSON read/write** — Definition of done: consistent **`Content-Type: application/json`** handling; invalid JSON → **400**.

## D. Schema DTOs

1. [x] **Slots & stages** — Definition of done: types for list item vs full card, **`StageState`**, stage object with timestamps/`error` (per **`plan.md`** D3).  
2. [x] **Profile** — Definition of done: **`GET`/`PUT`** body and response with **`text`**, **`updated_at`**.  
3. [x] **Stage runs** — Definition of done: **`POST` stage 2** (`include`/`exclude`), **`POST` stage 3** (`max_jobs` **1–100**), **`202`** response shape.  
4. [x] **Job list** — Definition of done: item fields including **`stage_3_rationale`** policy (**`plan.md`** D4); paginated wrapper **`items`**, **`page`**, **`limit`**, **`total`**.  
5. [x] **PATCH bucket** — Definition of done: body **`bucket`**: passed/failed; response choice **200 DTO vs 204** documented and consistent.

## E. Slots endpoints

1. [x] **`GET /api/v1/slots`** — Definition of done: returns all slots (≤ **3**); list items use compact **`stage_*`** (`state` only) per spec.  
2. [x] **`POST /api/v1/slots`** — Definition of done: body **`name`** only (non-empty trimmed); **creates** slot and **starts stage 1** ingest with **`SlotSearchQuery`** = that **`name`** per **`contracts/http-public-api.md`**; response **201** matches full **`GET /slots/{id}`** shape; **409** `slot_limit_reached` with **`limit`: 3**.  
3. [x] **`GET /api/v1/slots/{slot_id}`** — Definition of done: **200** full card; **404** when missing.  
4. [x] **`DELETE /api/v1/slots/{slot_id}`** — Definition of done: **204** hard delete; **404** when missing.  
5. [x] **Tests** — Definition of done: table-driven tests for **404**, **409** cap, and happy path with mocked **`impl`**.

## F. Profile endpoints

1. [x] **`GET /api/v1/profile`** — Definition of done: **200** with **`text`**, **`updated_at`**.  
2. [x] **`PUT /api/v1/profile`** — Definition of done: **200** mirrors GET; persistence and **`updated_at`** behavior correct.  
3. [x] **Tests** — Definition of done: round-trip PUT/GET; invalid body **400**.

## G. Stage run endpoints

1. [x] **`POST …/stages/2/run`** — Definition of done: validates body; triggers **`PIPELINE_STAGE2`** via **`008`** contract (stage **3** not started in same user action); **202** `{slot_id, stage: 2}`; **409** `stage_already_running` when stage **2** already **running**; invalidation semantics for stage **2+3** data per **`008`** / **`filter-invalidation.md`** on entry.  
2. [x] **`POST …/stages/3/run`** — Definition of done: **`max_jobs`** validation; **`PIPELINE_STAGE3`** with effective batch = **min**(policy, request) per spec; **202** `{slot_id, stage: 3}`; **409** for stage **3**; **`422`** or zero-scored path per **`plan.md`** D5.  
3. [x] **Concurrency guard** — Definition of done: at most one active run **per stage (2 or 3) per `slot_id`**; documented race strategy (DB row, Temporal workflow id, or equivalent).  
4. [x] **Tests** — Definition of done: **409** when “running”; **404** unknown slot; mock Temporal **ExecuteWorkflow** (or agreed client) asserts correct run kind and input.

## H. Job list endpoints

1. [x] **Pagination** — Definition of done: **`page`** from **1**, **`limit`** default + **max 100**; **`total`** correct for query.  
2. [x] **Sort** — Definition of done: **`posted_at` DESC**, then **`job_id` ASC** (stable).  
3. [x] **`bucket` query** — Definition of done: omit = full stage list; **`passed`** / **`failed`** filter for stages **2** and **3** only; stage **1** list matches spec (no bucket split required).  
4. [x] **Routes** — Definition of done: **`GET …/stages/1|2|3/jobs`** as in **`spec.md`**; **404** unknown slot.  
5. [x] **Tests** — Definition of done: pagination edges (`page`, `limit` max), empty list, bucket variants with mocked storage.

## I. Manual bucket PATCH

1. [x] **`PATCH …/stages/{2|3}/jobs/{job_id}`** — Definition of done: **`stage`** path restricted to **2** or **3**; **404** when slot or job not in scope for that stage; **`bucket`** passed/failed persisted per coarse model.  
2. [x] **Tests** — Definition of done: **404** out-of-scope; happy path **200** or **204** per schema decision.

## J. Quality gates

1. [x] **`make test` / `go test ./...`** — Definition of done: passes without mandatory network for default tests.  
2. [x] **`make vet` / `make fmt`** — Definition of done: clean for touched packages.
