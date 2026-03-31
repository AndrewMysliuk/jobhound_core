# Contract: environment variables (LLM policy & caps)

**Feature**: `007-llm-policy-and-caps`  
**Consumers**: Wiring code that runs pipeline executions and stage 3 (`cmd/worker`, `cmd/agent`, tests).

## Cap **N**

**Not an environment variable in v1.** The maximum number of `(job, pipeline_run)` pairs in **`PASSED_STAGE_2`** that may enter stage 3 **per pipeline-run execution** is a **named constant** in code (initial value **5**). See **`plan.md`** (Resolved decision D3) and **`contracts/pipeline-run-job-status.md`**.

If future work moves **N** to `JOBHOUND_*`, add the name and loader only under **`internal/config`** and update this file — do not read env in feature modules ad hoc.

## LLM (stage 3)

Unchanged from **`004`** — real Anthropic calls use the same variables as **`specs/004-pipeline-stages/contracts/environment.md`**:

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_ANTHROPIC_API_KEY` | Yes for **real** Claude calls | API key; omit for mock-only tests. |
| `JOBHOUND_ANTHROPIC_MODEL` | No | Model id; defaults per `internal/config`. |

## Relationship to other contracts

- **Temporal**: `specs/003-temporal-orchestration/contracts/environment.md` — worker address, namespace, task queue.  
- **Database**: `specs/002-postgres-gorm-migrations/contracts/environment.md` — `JOBHOUND_DATABASE_URL` (or equivalent).  
- **Pipeline stages**: `specs/004-pipeline-stages/contracts/environment.md` — stage rule parameters are **not** env; stage 3 key is above.
