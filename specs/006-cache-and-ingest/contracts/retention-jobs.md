# Contract: Job retention (hard delete)

**Feature**: `006-cache-and-ingest`  
**Purpose**: Freeze **schedule**, **cutoff rule**, and **FK cleanup** when deleting old `jobs` rows.

**Related**: `specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md` — **`pipeline_run_jobs.job_id`** must use **ON DELETE CASCADE** (or equivalent) so retention does not leave dangling rows.

---

## 1. Cutoff

- Delete rows from **`jobs`** where **`created_at` < (current timestamp in **UTC**) − **7 days**.
- **Hard DELETE** only — no soft-delete for this cleanup.

---

## 2. Schedule

- **Automatic**: run on a **cron** schedule **once per 7 days** in **UTC** (exact minute is implementation-defined; document in worker/ops).
- **Manual**: the **same** delete logic may be triggered by an operator (CLI, admin workflow, or one-off activity) — same SQL/transaction semantics as the scheduled run.

Because the job runs **at most** every 7 days, a row may persist **slightly longer than 7 calendar days** in the worst case (e.g. almost 14 days wall-clock between two runs). Product copy and SLOs should not assume “exactly 7 days on the dot” unless the schedule is tightened later.

---

## 3. Dependent rows

Deleting a **`jobs`** row **must** remove dependent rows that reference **`jobs(id)`**, in particular **`pipeline_run_jobs`** per `007` migrations (**CASCADE** or delete in the same transaction). No dangling foreign keys.

---

## 4. Indexing (optional follow-up)

If deletes become slow, add supporting indexes **as needed** (e.g. on `jobs(created_at)` for the retention query). Not normative in v1 beyond “as needed for performance”.
