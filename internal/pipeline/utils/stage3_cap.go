package utils

// MaxStage3JobsPerPipelineRunExecution is the default cap N on distinct job IDs that may enter stage 3
// in a single pipeline-run execution (008 contract: up to 20; overridable via config / activity field).
const MaxStage3JobsPerPipelineRunExecution = 20

// SelectStage3JobIDs returns up to maxPerExecution distinct job IDs from candidates for one stage-3
// batch in a pipeline-run execution. If maxPerExecution <= 0, [MaxStage3JobsPerPipelineRunExecution] is used.
//
// Ordering: candidates must be pre-ordered by product rules (008: jobs.posted_at DESC from
// [pipeline.PipelineRunRepository.ListPassedStage2JobIDs]). The slice is scanned in order; the first
// occurrence of each non-empty job_id is kept; later duplicates are skipped.
// Empty strings are ignored. Same eligible ordering → same selection (deterministic for retries/UI).
//
// exclude holds job IDs already sent to stage 3 in this execution (e.g. activity retries); those
// are omitted so the same job_id is not selected twice in one execution (contract §2 idempotency).
func SelectStage3JobIDs(candidates []string, exclude map[string]struct{}, maxPerExecution int) []string {
	if len(candidates) == 0 {
		return nil
	}
	capN := maxPerExecution
	if capN <= 0 {
		capN = MaxStage3JobsPerPipelineRunExecution
	}
	out := make([]string, 0, capN)
	seen := make(map[string]struct{}, capN)
	for _, id := range candidates {
		if id == "" {
			continue
		}
		if exclude != nil {
			if _, skip := exclude[id]; skip {
				continue
			}
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) >= capN {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
