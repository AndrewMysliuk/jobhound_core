# Contract: environment variables (LLM policy & caps)

**Feature**: `007-llm-policy-and-caps`  
**Consumers**: Wiring code that runs pipeline executions and stage 3 (`cmd/worker`, `cmd/agent`, tests).

## Cap **N**

The maximum number of `(job, pipeline_run)` pairs in **`PASSED_STAGE_2`** that may enter stage 3 **per pipeline-run execution** is **5** by default via the exported constant **`MaxStage3JobsPerPipelineRunExecution`** in **`internal/pipeline/utils`** (see **`plan.md`** D3 and **`contracts/pipeline-run-job-status.md`**). Selection rules are unchanged when **N** is overridden from the environment.

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN` | No | Positive integer cap **N** for stage 3 per run; unset or invalid uses **5**. Values above **10000** are clamped (safety). Loaded only in **`internal/config`**; **`cmd/worker`** passes it into pipeline activities — feature modules do not call `os.Getenv` for this knob. |

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
