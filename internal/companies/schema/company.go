package schema

import (
	"time"

	domainschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Company is one row in the companies table.
type Company struct {
	Key              string // immutable; domain if known at birth, else normalized name
	Name             string
	Aliases          []string
	Website          string
	ATS              ATS
	ATSSlug          string // verbatim; NEVER normalized
	BoardURL         string
	BoardETag        string
	ResolutionStatus ResolutionStatus
	ResolutionMethod ResolutionMethod
	PollState        PollState
	NextResolveAt    *time.Time
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	LastPolledAt     *time.Time
	PollFailCount    int
	EmptyPollStreak  int
	NeedsReview      bool // blocklist `review` pattern hit, or an outsized board
}

// UpsertCompanyParams is the discover-path input from an aggregator vacancy.
type UpsertCompanyParams struct {
	Name     string
	Website  string
	ApplyURL string
}

// UpsertCompanyResult reports what UpsertFromAggregator did.
type UpsertCompanyResult struct {
	Key     string
	Created bool
	Merged  bool
	Blocked bool
}

// ListFilter selects companies for List. Empty enum fields mean no filter.
// NeedsReview nil means no filter; non-nil filters on the bool.
type ListFilter struct {
	ResolutionStatus ResolutionStatus
	PollState        PollState
	ATS              ATS
	NeedsReview      *bool
	Page             int
	Limit            int
}

// BoardResult is what a fetcher returns. Outcome is as load-bearing as Jobs.
type BoardResult struct {
	Jobs    []domainschema.Job
	Outcome PollOutcome
	ETag    string
}

// ResolveCompanyPayload starts ResolveCompanyWorkflow.
type ResolveCompanyPayload struct {
	CompanyKey string
}

// PollCompaniesPayload starts PollCompaniesWorkflow.
type PollCompaniesPayload struct {
	RunID string
}

// PollResult is one poll attempt: outcome plus counter deltas for the run.
type PollResult struct {
	Outcome      PollOutcome
	JobsUpserted int
	JobsClosed   int
}

// UpsertCompaniesFromJobsInput is the discover activity input.
type UpsertCompaniesFromJobsInput struct {
	RunID int64
}

// UpsertCompaniesFromJobsResult is discover-path counter deltas.
type UpsertCompaniesFromJobsResult struct {
	Discovered int
	Merged     int
	Blocked    int
}

// ListPendingResolveInput caps how many PENDING companies to return.
type ListPendingResolveInput struct {
	Limit int
}

// ListPendingResolveResult is company keys ready for ResolveCompanyWorkflow.
type ListPendingResolveResult struct {
	CompanyKeys []string
}

// ResolveCompanyResult is the resolve activity outcome for run counters.
type ResolveCompanyResult struct {
	CompanyKey string
	Status     ResolutionStatus
	Skipped    bool
}

// ListDuePollableInput caps the poll list for one run.
type ListDuePollableInput struct {
	Limit int
}

// PollTarget is one due company plus its ATS (for per-ATS concurrency).
type PollTarget struct {
	CompanyKey string
	ATS        ATS
}

// ListDuePollableResult is the poll queue plus the concurrency knob from config.
type ListDuePollableResult struct {
	Targets        []PollTarget
	ATSConcurrency int
}

// PollCompanyInput identifies one company to poll.
type PollCompanyInput struct {
	CompanyKey string
}

// PollCompanyResult is one poll activity outcome (Skipped = business skip).
type PollCompanyResult struct {
	Outcome      PollOutcome
	JobsUpserted int
	JobsClosed   int
	Skipped      bool
}

// RecordPollFailureInput bumps poll_fail_count after transport retries exhaust.
type RecordPollFailureInput struct {
	CompanyKey string
}

// PollCompaniesResult aggregates poll counters for the parent profile run.
type PollCompaniesResult struct {
	CompaniesPolled  int
	PollsNotModified int
	PollsPartial     int
	PollFailures     int
	JobsUpsertedATS  int
	JobsClosed       int
}

// MarkPolledParams updates poll bookkeeping after a board fetch attempt.
type MarkPolledParams struct {
	Key             string
	LastPolledAt    time.Time
	BoardETag       string
	PollFailCount   int
	EmptyPollStreak int
	PollState       PollState
}

// MarkResolutionParams commits a resolve outcome for one company.
type MarkResolutionParams struct {
	Key              string
	ResolutionStatus ResolutionStatus
	ResolutionMethod ResolutionMethod
	ATS              ATS
	ATSSlug          string
	BoardURL         string
	NextResolveAt    *time.Time
}
