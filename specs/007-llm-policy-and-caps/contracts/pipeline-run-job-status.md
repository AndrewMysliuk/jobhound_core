# Contract: pipeline run job status & persistence

**Feature**: `007-llm-policy-and-caps`  
**Purpose**: Freeze **enum values**, **where they live**, **allowed transitions**, **SQL shape**, and **cap** behaviour for implementation and migrations.

**Related**: `specs/004-pipeline-stages/contracts/pipeline-stages.md` (stage semantics unchanged); `specs/002-postgres-gorm-migrations/contracts/jobs-schema.md` (`jobs` base table); `specs/006-cache-and-ingest/spec.md` (ingest + retention). Later migrations may add columns to **`pipeline_runs`** (e.g. schedule, workflow ids) without changing the PK or table name.

### Alignment (spec **`007`** + `004`)

- **`spec.md` acceptance**: Cap **N** as a named code constant → §2 here and **`plan.md`** D3; **`PASSED_STAGE_1`** on **`jobs`**, no **`REJECTED_STAGE_1`** → §1.1 and §3; per-run statuses and transitions → §1.2–§1.3 and §5; at most **N** distinct **`job_id`** per execution and no duplicate stage-3 send in one execution → §2; retention / no dangling per-run rows → §7.
- **`004`**: Stages 1–3 **behaviour** (broad filter, keywords, LLM scoring on **`domain.Job`**) are **unchanged**. This contract adds **persistence** only: canonical ingest completion is **`PASSED_STAGE_1`** on **`jobs`** (wired with **`006`**); stage 2/3 outcomes for a given run live in **`pipeline_run_jobs`**. Jobs **dropped** in memory by stage 1 or 2 in **`004`** are not represented as a **`REJECTED_STAGE_1`** row — that value **does not exist**, consistent with **`004`**’s “omit from the filtered list” model.

---

## 1. Status enums (logical)

### 1.1 Stage 1 on `jobs` (single row per vacancy)

| Value | Meaning |
| ----- | ------- |
| `PASSED_STAGE_1` | Broad stage 1 / canonical ingest succeeded; normalized vacancy stored on **`jobs`**. |

There is **no** `REJECTED_STAGE_1`.

**Storage**: a dedicated column on **`jobs`** (implementation name: e.g. **`stage1_status`** or **`ingest_stage1_status`**) using a Postgres **`enum`** or **`text`** with a **check** constraint — must accept at least **`PASSED_STAGE_1`** for v1. Legacy rows may be **`NULL`** until backfilled; product rules for NULL are **implementation-defined** but should align with **`006`** ingest completion.

### 1.2 Stages 2–3 per `(job_id, pipeline_run_id)`

| Value | Meaning |
| ----- | ------- |
| `REJECTED_STAGE_2` | For **this** pipeline run, keyword stage **2** did not pass. |
| `PASSED_STAGE_2` | Stages 1 and 2 passed; stage 3 has **not** yet produced a terminal outcome — includes **cap backlog** (waiting for a slot in a stage-3 batch). |
| `PASSED_STAGE_3` | Stage 3 completed with a **pass** outcome. |
| `REJECTED_STAGE_3` | Stage 3 completed with a **reject** outcome. |

### 1.3 Allowed transitions (within one pipeline run)

```
PASSED_STAGE_1 → (PASSED_STAGE_2 | REJECTED_STAGE_2)
PASSED_STAGE_2 → (PASSED_STAGE_3 | REJECTED_STAGE_3)
```

- **`PASSED_STAGE_1`** is recorded on **`jobs`** when the vacancy enters the store after successful broad stage 1; stage 2 writes **per-run** rows for 2/3 outcomes.
- **Repeat** stage 3 for the same job in the **same** run (rescoring / manual) is **not in scope for v1**.

---

## 2. Cap **N** (before stage 3)

| Rule | Detail |
| ---- | ------ |
| Value | **5** initially — **named constant** in code (see **`plan.md`** D3). |
| Scope | Only pairs already in **`PASSED_STAGE_2`** when **this** pipeline-run **execution** builds the stage-3 batch. |
| Limit | At most **N** distinct **`job_id`** values sent to stage 3 in **one** execution of that run. |
| Ordering | **Which** **N** rows — **implementation-defined** (document in code). |
| Backlog | Rows not selected remain **`PASSED_STAGE_2`** until a later feature processes them (out of scope for v1 except as backlog). |
| Idempotency | Within **one** execution, the same **`job_id`** **must not** be sent to stage 3 **twice**. |

---

## 3. SQL — `jobs` extension

Add a column (**normative name** for v1; **must** match migration + GORM):

| Column | Type | Notes |
| ------ | ---- | ----- |
| `stage1_status` | `text` + `CHECK` (or Postgres `enum`) | Allowed non-null value: **`PASSED_STAGE_1`** only; **`NULL`** for legacy rows until backfill / **`006`** ingest sets the column. |

---

## 4. SQL — `pipeline_runs` (minimal, **owned by `007`**)

**`007`** migrations **create** this table. It exists solely to give **`pipeline_run_id`** a stable FK target; other epics may add nullable columns later.

| Column | Type | Notes |
| ------ | ---- | ----- |
| `id` | `uuid` or `bigserial` | PK — referenced as **`pipeline_run_id`** in child table. |
| `created_at` | `timestamptz` | Recommended. |

---


## 5. SQL — `pipeline_run_jobs` (per-run join)

**Table name**: **`pipeline_run_jobs`** (normative unless repo-wide rename with spec update).

| Column | Type | Notes |
| ------ | ---- | ----- |
| `pipeline_run_id` | FK → `pipeline_runs(id)` | **ON DELETE CASCADE** or restrict per product choice; document in migration. |
| `job_id` | FK → `jobs(id)` | **ON DELETE CASCADE** — **required** so retention hard-deletes do not leave dangling rows (`006`). |
| `status` | `enum` / `text` + check | One of **`REJECTED_STAGE_2`**, **`PASSED_STAGE_2`**, **`PASSED_STAGE_3`**, **`REJECTED_STAGE_3`**. |
| **Primary key** | **`(pipeline_run_id, job_id)`** | Recommended. |

**Indexes**: at least `(pipeline_run_id, status)` for loading **`PASSED_STAGE_2`** candidates for cap selection.

---

## 6. GORM & domain

- GORM models live only under **`internal/.../storage/`**; **`internal/domain`** does not import GORM.
- Enum strings in Go should match this contract **or** map with a small conversion layer documented next to the model.

---

## 7. Cleanup

On **hard-delete** of a **`jobs`** row, dependent **`pipeline_run_jobs`** rows **must** be removed (**CASCADE** or explicit delete in the same retention path). No dangling FKs.
