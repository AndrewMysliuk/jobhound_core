# Contract: environment variables (manual search workflow)

**Feature**: `008-manual-search-workflow`  
**Consumers**: Temporal worker / future `009` triggers — no dedicated env surface for this epic alone.

**008**: This epic does **not** introduce slot-specific `JOBHOUND_*` names beyond what prior epics already defined. Reuse Temporal (`JOBHOUND_TEMPORAL_*`, task queue, namespace — see [`../../003-temporal-orchestration/contracts/environment.md`](../../003-temporal-orchestration/contracts/environment.md)), database, Redis, ingest, and pipeline keys.

**Stage-3 batch cap**: Normative default **20** jobs per pipeline-run execution is documented in [`../spec.md`](../spec.md). It is configurable via **`JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN`** in [`internal/config`](../../../internal/config/pipeline.go) (same default as [`internal/pipeline/utils`](../../../internal/pipeline/utils/stage3_cap.go)); worker passes this into pipeline activities (`cmd/worker`).
