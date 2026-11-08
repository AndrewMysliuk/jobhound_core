package schema

// EnrichBatchInput is the Temporal child workflow payload.
// Empty JobIDs means drain mode: the workflow pages ListMissing until none remain.
// Activities re-read title, description, raw location, and raw salary; the payload does not carry the text.
type EnrichBatchInput struct {
	JobIDs        []string
	PromptVersion int
	Model         string
	// StoredSoFar carries the verdict count across ContinueAsNew runs so the
	// final log line reports the whole drain, not just the last run.
	StoredSoFar int
}

// ListMissingInput is the ListMissing activity payload.
// Empty Model or PromptVersion means the activity uses the service defaults.
// Limit > 0 caps how many ids are returned (Temporal payload budget).
type ListMissingInput struct {
	PromptVersion int
	Model         string
	Limit         int
}

// ListMissingResult is the missing-id list plus the prompt/model used to compute it.
type ListMissingResult struct {
	JobIDs        []string
	PromptVersion int
	Model         string
}

// SubmitBatchResult is returned by SubmitBatch after creating one Anthropic Message Batch.
// SubmittedIDs is the batch membership; the custom_id map is derived from it by
// the activity that needs it instead of travelling through Temporal payloads.
type SubmitBatchResult struct {
	BatchID      string
	SubmittedIDs []string
}

// PollBatchInput is one status check for a batch. It carries the batch id only:
// the workflow repeats this activity until the batch ends, and every activity
// input is written to workflow history, so per-job data must not travel with it.
type PollBatchInput struct {
	BatchID string
}

// BatchCounts mirrors request_counts from the Message Batches API. It is the
// per-request breakdown of one batch and is carried on every status check so a
// long poll loop shows progress instead of a bare "not ended yet".
type BatchCounts struct {
	Processing int
	Succeeded  int
	Errored    int
	Canceled   int
	Expired    int
}

// Total is every request in the batch across all terminal and non-terminal states.
func (c BatchCounts) Total() int {
	return c.Processing + c.Succeeded + c.Errored + c.Canceled + c.Expired
}

// Done is every request that reached a terminal state.
func (c BatchCounts) Done() int {
	return c.Succeeded + c.Errored + c.Canceled + c.Expired
}

// PollBatchResult is one status check. Results are loaded once by FetchResults.
// Status is the raw processing_status and Counts is request_counts: both are
// written to workflow history, so the Temporal UI shows how far a batch got.
type PollBatchResult struct {
	Ended  bool
	Status string
	Counts BatchCounts
}

// FetchResultsInput loads the JSONL results of an ended batch exactly once.
// JobIDs is the batch membership returned by SubmitBatch; the activity rebuilds
// the custom_id to job id map from it.
type FetchResultsInput struct {
	BatchID string
	JobIDs  []string
}

// FetchResultsResult maps job id to model text for succeeded results only.
type FetchResultsResult struct {
	Texts map[string]string
}

// StoreVerdictsInput is the coerced-reply write after a batch ends.
type StoreVerdictsInput struct {
	Texts         map[string]string
	PromptVersion int
	Model         string
	// AcceptUnknownOnSchemaFail stores every answer unknown for a schema-invalid
	// item instead of returning an error. The activity sets this only after
	// malformed-body retries are exhausted.
	AcceptUnknownOnSchemaFail bool
}

// StoreVerdictsResult lists job ids that received a row in this call.
type StoreVerdictsResult struct {
	StoredIDs []string
}

// RescoreJobsInput is the post-store rescore for matches of the given jobs.
type RescoreJobsInput struct {
	JobIDs []string
}
