// Package companies_activities hosts Temporal activities for company discover / resolve / poll.
package companies_activities

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	companiesimpl "github.com/andrewmysliuk/jobhound_core/internal/companies/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

const (
	// UpsertCompaniesFromJobsActivityName is the discover pass over recently seen jobs.
	UpsertCompaniesFromJobsActivityName = "UpsertCompaniesFromJobs"
	// ListPendingResolveActivityName lists PENDING companies due for resolve.
	ListPendingResolveActivityName = "ListPendingResolve"
	// ResolveCompanyActivityName resolves one company.
	ResolveCompanyActivityName = "ResolveCompany"
	// ListDuePollableActivityName lists RESOLVED companies due for poll.
	ListDuePollableActivityName = "ListDuePollable"
	// PollCompanyActivityName polls one company board.
	PollCompanyActivityName = "PollCompany"
	// RecordPollFailureActivityName bumps poll_fail_count after transport exhaustion.
	RecordPollFailureActivityName = "RecordPollFailure"

	// PollCompanyMaxAttempts is the Temporal retry budget for transport failures.
	PollCompanyMaxAttempts int32 = 3
)

// Activities runs companies Temporal steps. Temporal SDK stays here, not in impl/.
type Activities struct {
	Companies *companiesimpl.Service
	Jobs      jobs.JobRepository
	Runs      *storage.Repository
	Log       zerolog.Logger
}

// UpsertCompaniesFromJobs discovers companies from jobs last seen at/after the run start.
func (a *Activities) UpsertCompaniesFromJobs(ctx context.Context, in schema.UpsertCompaniesFromJobsInput) (schema.UpsertCompaniesFromJobsResult, error) {
	if a == nil || a.Companies == nil || a.Jobs == nil || a.Runs == nil {
		return schema.UpsertCompaniesFromJobsResult{}, fmt.Errorf("companies activity: companies, jobs, and runs are required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, UpsertCompaniesFromJobsActivityName))
	run, err := a.Runs.GetRun(ctx, in.RunID)
	if err != nil {
		log.Error().Err(err).Str("failure_stage", schema.FailureStageDiscover.String()).Msg("load run failed")
		return schema.UpsertCompaniesFromJobsResult{}, err
	}
	jobsSeen, err := a.Jobs.ListLastSeenSince(ctx, run.StartedAt)
	if err != nil {
		log.Error().Err(err).Str("failure_stage", schema.FailureStageDiscover.String()).Msg("list jobs failed")
		return schema.UpsertCompaniesFromJobsResult{}, err
	}

	type sighting struct {
		name, website, applyURL string
	}
	unique := make(map[string]sighting)
	for _, job := range jobsSeen {
		name := strings.TrimSpace(job.Company)
		if name == "" {
			continue
		}
		key := strings.ToLower(name) + "\n" + strings.TrimSpace(job.CompanyWebsite)
		if _, ok := unique[key]; ok {
			continue
		}
		unique[key] = sighting{name: name, website: job.CompanyWebsite, applyURL: job.ApplyURL}
	}

	var out schema.UpsertCompaniesFromJobsResult
	for _, s := range unique {
		res, err := a.Companies.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{
			Name:     s.name,
			Website:  s.website,
			ApplyURL: s.applyURL,
		})
		if err != nil {
			log.Error().Err(err).
				Str("failure_stage", schema.FailureStageDiscover.String()).
				Str("company", s.name).
				Msg("upsert company failed")
			return out, err
		}
		switch {
		case res.Blocked:
			out.Blocked++
		case res.Created:
			out.Discovered++
		case res.Merged:
			out.Merged++
		}
	}
	log.Info().
		Int("discovered", out.Discovered).
		Int("merged", out.Merged).
		Int("blocked", out.Blocked).
		Msg("discover done")
	return out, nil
}

// ListPendingResolve returns keys of PENDING companies due for resolve.
func (a *Activities) ListPendingResolve(ctx context.Context, in schema.ListPendingResolveInput) (schema.ListPendingResolveResult, error) {
	if a == nil || a.Companies == nil {
		return schema.ListPendingResolveResult{}, fmt.Errorf("companies activity: companies service is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, ListPendingResolveActivityName))
	list, err := a.Companies.ListPendingResolve(ctx, time.Now().UTC(), in.Limit)
	if err != nil {
		log.Error().Err(err).Str("failure_stage", schema.FailureStageResolve.String()).Msg("list pending failed")
		return schema.ListPendingResolveResult{}, err
	}
	keys := make([]string, 0, len(list))
	for _, c := range list {
		if c.Key != "" {
			keys = append(keys, c.Key)
		}
	}
	return schema.ListPendingResolveResult{CompanyKeys: keys}, nil
}

// ResolveCompany resolves one PENDING company. Business skips return Skipped=true.
func (a *Activities) ResolveCompany(ctx context.Context, in schema.ResolveCompanyPayload) (schema.ResolveCompanyResult, error) {
	if a == nil || a.Companies == nil {
		return schema.ResolveCompanyResult{}, fmt.Errorf("companies activity: companies service is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, ResolveCompanyActivityName))
	c, err := a.Companies.Resolve(ctx, in.CompanyKey)
	if err != nil {
		if companiesimpl.IsTerminalCompanyError(err) || errors.Is(err, companies.ErrCompanyNotFound) {
			log.Info().
				Str("failure_stage", schema.FailureStageResolve.String()).
				Str("company_key", in.CompanyKey).
				Str("error", err.Error()).
				Msg("resolve skipped")
			return schema.ResolveCompanyResult{CompanyKey: in.CompanyKey, Skipped: true}, nil
		}
		log.Error().Err(err).
			Str("failure_stage", schema.FailureStageResolve.String()).
			Str("company_key", in.CompanyKey).
			Msg("resolve failed")
		return schema.ResolveCompanyResult{}, err
	}
	return schema.ResolveCompanyResult{
		CompanyKey: c.Key,
		Status:     c.ResolutionStatus,
	}, nil
}

// ListDuePollable returns due RESOLVED companies and the per-ATS concurrency.
func (a *Activities) ListDuePollable(ctx context.Context, in schema.ListDuePollableInput) (schema.ListDuePollableResult, error) {
	if a == nil || a.Companies == nil {
		return schema.ListDuePollableResult{}, fmt.Errorf("companies activity: companies service is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, ListDuePollableActivityName))
	list, err := a.Companies.DuePollable(ctx, time.Now().UTC(), in.Limit)
	if err != nil {
		log.Error().Err(err).Str("failure_stage", schema.FailureStagePoll.String()).Msg("list due failed")
		return schema.ListDuePollableResult{}, err
	}
	targets := make([]schema.PollTarget, 0, len(list))
	for _, c := range list {
		targets = append(targets, schema.PollTarget{CompanyKey: c.Key, ATS: c.ATS})
	}
	concurrency := a.Companies.ATSConcurrency()
	if concurrency < 1 {
		concurrency = 5
	}
	return schema.ListDuePollableResult{Targets: targets, ATSConcurrency: concurrency}, nil
}

// PollCompany polls one board. Business skips return Skipped=true (never a failure).
// Transport errors retry via Temporal; terminal board-format is non-retryable.
func (a *Activities) PollCompany(ctx context.Context, in schema.PollCompanyInput) (schema.PollCompanyResult, error) {
	if a == nil || a.Companies == nil {
		return schema.PollCompanyResult{}, fmt.Errorf("companies activity: companies service is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, PollCompanyActivityName))
	res, err := a.Companies.Poll(ctx, in.CompanyKey)
	if err != nil {
		if companiesimpl.IsTerminalCompanyError(err) || errors.Is(err, companies.ErrCompanyNotFound) {
			log.Info().
				Str("failure_stage", schema.FailureStagePoll.String()).
				Str("company_key", in.CompanyKey).
				Str("error", err.Error()).
				Msg("poll skipped")
			return schema.PollCompanyResult{Skipped: true}, nil
		}
		log.Error().Err(err).
			Str("failure_stage", schema.FailureStagePoll.String()).
			Str("company_key", in.CompanyKey).
			Msg("poll failed")
		return schema.PollCompanyResult{}, err
	}
	return schema.PollCompanyResult{
		Outcome:      res.Outcome,
		JobsUpserted: res.JobsUpserted,
		JobsClosed:   res.JobsClosed,
	}, nil
}

// RecordPollFailure increments poll_fail_count after transport retries exhaust.
func (a *Activities) RecordPollFailure(ctx context.Context, in schema.RecordPollFailureInput) error {
	if a == nil || a.Companies == nil {
		return fmt.Errorf("companies activity: companies service is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, RecordPollFailureActivityName))
	if err := a.Companies.RecordPollTransportFailure(ctx, in.CompanyKey); err != nil {
		log.Error().Err(err).
			Str("failure_stage", schema.FailureStagePoll.String()).
			Str("company_key", in.CompanyKey).
			Msg("record poll failure failed")
		return err
	}
	return nil
}
