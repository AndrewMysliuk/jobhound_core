// Package companies is the company-database and ATS-polling module:
// contracts at the root, data model under schema/.
package companies

import (
	"context"
	"errors"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
)

// API is the companies use-case surface. Implementations do not import Temporal.
type API interface {
	// UpsertFromAggregator is the discover path: blocklist check, then
	// merge-or-insert. Never creates a second row for a known company.
	UpsertFromAggregator(ctx context.Context, p schema.UpsertCompanyParams) (schema.UpsertCompanyResult, error)

	// Resolve finds ats + slug for one PENDING company and commits the outcome.
	Resolve(ctx context.Context, companyKey string) (schema.Company, error)

	// DuePollable returns RESOLVED + ACTIVE companies past their interval,
	// oldest LastPolledAt first, capped.
	DuePollable(ctx context.Context, now time.Time, limit int) ([]schema.Company, error)

	// Poll fetches one board and applies the result. Closing happens only on
	// PollOutcomeComplete.
	Poll(ctx context.Context, companyKey string) (schema.PollResult, error)

	// ListPendingResolve returns PENDING companies due for resolve.
	ListPendingResolve(ctx context.Context, now time.Time, limit int) ([]schema.Company, error)

	// RecordPollTransportFailure increments poll_fail_count after Temporal
	// retries for a transport error are exhausted. Nothing is closed.
	RecordPollTransportFailure(ctx context.Context, companyKey string) error

	List(ctx context.Context, f schema.ListFilter) ([]schema.Company, error)
	// Count returns how many companies match f (ignoring page/limit).
	Count(ctx context.Context, f schema.ListFilter) (int, error)
}

// CompanyBoardFetcher is implemented once per pollable ATS.
type CompanyBoardFetcher interface {
	ATS() schema.ATS
	// FetchBoard returns the board plus the outcome. companyName stamps
	// Job.Company: Ashby and Lever boards omit it, and for Greenhouse it is
	// the fallback when the board omits company_name. One fetcher serves every
	// company of its ATS, so the name is per-call, not a client field. etag is
	// the stored BoardETag ("" on first poll); an empty-but-successful board is
	// PollOutcomeEmpty, a 404 is PollOutcomeDeadSlug — never an error.
	FetchBoard(ctx context.Context, slug, companyName, etag string) (schema.BoardResult, error)
}

var (
	ErrCompanyNotFound = errors.New("company not found")
	ErrNotPollable     = errors.New("company ats has no fetcher")
	ErrBoardFormat     = errors.New("board response does not match the expected format")
)
