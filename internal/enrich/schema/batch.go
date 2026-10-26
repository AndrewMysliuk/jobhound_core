package schema

// EnrichBatchInput is the Temporal child workflow payload.
// Empty JobIDs means drain mode: the workflow pages ListMissing until none remain.
// Activities re-read title, description, raw location, and raw salary; the payload does not carry the text.
type EnrichBatchInput struct {
	JobIDs        []string
	PromptVersion int
	Model         string
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
// Callers must pass at most MaxJobsPerSubmit ids so CustomToJob fits Temporal limits.
type SubmitBatchResult struct {
	BatchID      string
	CustomToJob  map[string]string
	SubmittedIDs []string
}

// PollBatchInput is one status check (and, when ended, a results fetch) for a batch.
type PollBatchInput struct {
	BatchID     string
	CustomToJob map[string]string
}

// PollBatchResult is one poll. When Ended is true, Texts maps job id to model text
// for succeeded results only.
type PollBatchResult struct {
	Ended bool
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
