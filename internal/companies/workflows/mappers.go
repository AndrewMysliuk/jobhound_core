package companies_workflows

import (
	"errors"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// IsBusinessSkip reports a companies outcome that must never fail a workflow:
// not pollable, board-format terminal, or company missing.
func IsBusinessSkip(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, companies.ErrNotPollable) ||
		errors.Is(err, companies.ErrBoardFormat) ||
		errors.Is(err, companies.ErrCompanyNotFound) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "company ats has no fetcher") ||
		strings.Contains(msg, "board response does not match") ||
		strings.Contains(msg, "company not found")
}

// ATSCountersFromDiscover maps discover activity counts into a run delta.
func ATSCountersFromDiscover(r schema.UpsertCompaniesFromJobsResult) scoringschema.RunATSCounters {
	return scoringschema.RunATSCounters{
		CompaniesDiscovered: r.Discovered,
		CompaniesMerged:     r.Merged,
		CompaniesBlocked:    r.Blocked,
	}
}

// ATSCountersFromResolve maps one resolve outcome into a run delta.
func ATSCountersFromResolve(r schema.ResolveCompanyResult) scoringschema.RunATSCounters {
	if r.Skipped {
		return scoringschema.RunATSCounters{}
	}
	switch r.Status {
	case schema.ResolutionStatusResolved:
		return scoringschema.RunATSCounters{CompaniesResolved: 1}
	case schema.ResolutionStatusUnsupportedATS:
		return scoringschema.RunATSCounters{CompaniesUnsupported: 1}
	default:
		return scoringschema.RunATSCounters{}
	}
}

// ATSCountersFromPoll maps one poll activity outcome into a run delta.
func ATSCountersFromPoll(r schema.PollCompanyResult) scoringschema.RunATSCounters {
	if r.Skipped {
		return scoringschema.RunATSCounters{}
	}
	delta := scoringschema.RunATSCounters{
		CompaniesPolled: 1,
		JobsUpsertedATS: r.JobsUpserted,
		JobsClosed:      r.JobsClosed,
	}
	switch r.Outcome {
	case schema.PollOutcomeNotModified:
		delta.PollsNotModified = 1
	case schema.PollOutcomePartial:
		delta.PollsPartial = 1
	case schema.PollOutcomeDeadSlug:
		delta.PollFailures = 1
	}
	return delta
}

// ATSCountersFromPollWorkflow maps the poll child aggregate into a run delta.
func ATSCountersFromPollWorkflow(r schema.PollCompaniesResult) scoringschema.RunATSCounters {
	return scoringschema.RunATSCounters{
		CompaniesPolled:  r.CompaniesPolled,
		PollsNotModified: r.PollsNotModified,
		PollsPartial:     r.PollsPartial,
		PollFailures:     r.PollFailures,
		JobsUpsertedATS:  r.JobsUpsertedATS,
		JobsClosed:       r.JobsClosed,
	}
}
