package impl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// DuePollable returns RESOLVED + ACTIVE companies past their interval,
// oldest LastPolledAt first, capped. Interval and default cap come from config.
func (s *Service) DuePollable(ctx context.Context, now time.Time, limit int) ([]schema.Company, error) {
	if s == nil || s.companies == nil {
		return nil, errors.New("companies: service not configured")
	}
	if limit < 1 {
		limit = s.cfg.PollMaxPerRun
	}
	if limit < 1 {
		limit = config.DefaultCompaniesPollMaxPerRun
	}
	hours := s.cfg.PollIntervalHours
	if hours < 1 {
		hours = config.DefaultCompaniesPollIntervalHours
	}
	dueBefore := now.UTC().Add(-time.Duration(hours) * time.Hour)
	return s.companies.ListDuePollable(ctx, dueBefore, limit)
}

// Poll fetches one board and applies the PollOutcome.
// COMPLETE upserts and closes; PARTIAL upserts and closes nothing; EMPTY bumps
// empty_poll_streak (5 → INACTIVE); NOT_MODIFIED touches only last_polled_at;
// DEAD_SLUG bumps poll_fail_count (3 → PENDING, slug kept).
func (s *Service) Poll(ctx context.Context, companyKey string) (schema.PollResult, error) {
	if s == nil || s.companies == nil {
		return schema.PollResult{}, errors.New("companies: service not configured")
	}
	c, err := s.companies.GetByKey(ctx, companyKey)
	if err != nil {
		return schema.PollResult{}, err
	}
	if c.ResolutionStatus != schema.ResolutionStatusResolved {
		return schema.PollResult{}, companies.ErrNotPollable
	}
	if !c.ATS.Pollable() {
		return schema.PollResult{}, companies.ErrNotPollable
	}
	fetcher, ok := s.fetchers[c.ATS]
	if !ok || fetcher == nil {
		return schema.PollResult{}, companies.ErrNotPollable
	}

	result, err := fetcher.FetchBoard(ctx, c.ATSSlug, c.Name, c.BoardETag)
	if err != nil {
		return schema.PollResult{}, mapPollFetchError(s.log, c.Key, err)
	}

	now := s.now()
	switch result.Outcome {
	case schema.PollOutcomeNotModified:
		if err := s.companies.MarkPolled(ctx, schema.MarkPolledParams{
			Key:             c.Key,
			LastPolledAt:    now,
			BoardETag:       preferETag(result.ETag, c.BoardETag),
			PollFailCount:   0,
			EmptyPollStreak: c.EmptyPollStreak,
			PollState:       c.PollState,
		}); err != nil {
			return schema.PollResult{}, err
		}
		return schema.PollResult{Outcome: schema.PollOutcomeNotModified}, nil

	case schema.PollOutcomeComplete:
		upserted, closed, err := s.applyJobs(ctx, c.Key, result, true)
		if err != nil {
			return schema.PollResult{}, err
		}
		if err := s.companies.MarkPolled(ctx, schema.MarkPolledParams{
			Key:             c.Key,
			LastPolledAt:    now,
			BoardETag:       preferETag(result.ETag, c.BoardETag),
			PollFailCount:   0,
			EmptyPollStreak: 0,
			PollState:       schema.PollStateActive,
		}); err != nil {
			return schema.PollResult{}, err
		}
		return schema.PollResult{
			Outcome:      schema.PollOutcomeComplete,
			JobsUpserted: upserted,
			JobsClosed:   closed,
		}, nil

	case schema.PollOutcomePartial:
		upserted, _, err := s.applyJobs(ctx, c.Key, result, false)
		if err != nil {
			return schema.PollResult{}, err
		}
		if err := s.companies.MarkPolled(ctx, schema.MarkPolledParams{
			Key:             c.Key,
			LastPolledAt:    now,
			BoardETag:       preferETag(result.ETag, c.BoardETag),
			PollFailCount:   0,
			EmptyPollStreak: 0,
			PollState:       c.PollState,
		}); err != nil {
			return schema.PollResult{}, err
		}
		return schema.PollResult{
			Outcome:      schema.PollOutcomePartial,
			JobsUpserted: upserted,
		}, nil

	case schema.PollOutcomeEmpty:
		// Success: slug stays verified. Do not close — I7 keeps the aggregator
		// path when the board is empty but aggregators still list jobs.
		s.logEmptyBoardFallback(ctx, c.Key)
		streak := c.EmptyPollStreak + 1
		pollState := c.PollState
		if streak >= emptyPollStreakToInactive {
			pollState = schema.PollStateInactive
		}
		if err := s.companies.MarkPolled(ctx, schema.MarkPolledParams{
			Key:             c.Key,
			LastPolledAt:    now,
			BoardETag:       preferETag(result.ETag, c.BoardETag),
			PollFailCount:   0,
			EmptyPollStreak: streak,
			PollState:       pollState,
		}); err != nil {
			return schema.PollResult{}, err
		}
		return schema.PollResult{Outcome: schema.PollOutcomeEmpty}, nil

	case schema.PollOutcomeDeadSlug:
		failCount := c.PollFailCount + 1
		if err := s.companies.MarkPolled(ctx, schema.MarkPolledParams{
			Key:             c.Key,
			LastPolledAt:    now,
			BoardETag:       c.BoardETag,
			PollFailCount:   failCount,
			EmptyPollStreak: c.EmptyPollStreak,
			PollState:       c.PollState,
		}); err != nil {
			return schema.PollResult{}, err
		}
		if failCount >= deadSlugStreakToPending {
			// Re-queue resolve; keep previous slug and resolution_method as history.
			if err := s.companies.MarkResolution(ctx, schema.MarkResolutionParams{
				Key:              c.Key,
				ResolutionStatus: schema.ResolutionStatusPending,
				ResolutionMethod: c.ResolutionMethod,
				ATS:              c.ATS,
				ATSSlug:          c.ATSSlug,
				BoardURL:         c.BoardURL,
				NextResolveAt:    nil,
			}); err != nil {
				return schema.PollResult{}, err
			}
		}
		return schema.PollResult{Outcome: schema.PollOutcomeDeadSlug}, nil

	default:
		return schema.PollResult{}, fmt.Errorf("companies: unknown poll outcome %q", result.Outcome)
	}
}

// ListPendingResolve returns PENDING companies due for resolve.
func (s *Service) ListPendingResolve(ctx context.Context, now time.Time, limit int) ([]schema.Company, error) {
	if s == nil || s.companies == nil {
		return nil, errors.New("companies: service not configured")
	}
	if limit < 1 {
		limit = s.cfg.PollMaxPerRun
	}
	if limit < 1 {
		limit = config.DefaultCompaniesPollMaxPerRun
	}
	return s.companies.ListPendingResolve(ctx, now.UTC(), limit)
}

// RecordPollTransportFailure increments poll_fail_count after transport retries
// exhaust. The company is otherwise untouched; no vacancy is closed.
func (s *Service) RecordPollTransportFailure(ctx context.Context, companyKey string) error {
	if s == nil || s.companies == nil {
		return errors.New("companies: service not configured")
	}
	c, err := s.companies.GetByKey(ctx, companyKey)
	if err != nil {
		return err
	}
	lastPolled := s.now()
	if c.LastPolledAt != nil {
		lastPolled = *c.LastPolledAt
	}
	s.logger().WarnContext(ctx, "companies: poll transport exhausted",
		"failure_stage", schema.FailureStagePoll.String(),
		"company_key", companyKey,
		"poll_fail_count", c.PollFailCount+1,
	)
	return s.companies.MarkPolled(ctx, schema.MarkPolledParams{
		Key:             c.Key,
		LastPolledAt:    lastPolled,
		BoardETag:       c.BoardETag,
		PollFailCount:   c.PollFailCount + 1,
		EmptyPollStreak: c.EmptyPollStreak,
		PollState:       c.PollState,
	})
}

func (s *Service) applyJobs(ctx context.Context, companyKey string, result schema.BoardResult, closeAbsent bool) (upserted, closed int, err error) {
	if s.jobs == nil {
		if len(result.Jobs) == 0 && !closeAbsent {
			return 0, 0, nil
		}
		return 0, 0, errors.New("companies: jobs store is required for poll upsert")
	}
	// Drop known posted_at older than PostedAtRejectDays before upsert and seen.
	// UpsertBoardJobs applies the same rule; filtering here keeps stale ids out of
	// CloseAbsent / ReopenPresent so COMPLETE polls close existing board_seen rows.
	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(scoringschema.PostedAtRejectDays) * 24 * time.Hour)
	fresh := make([]jobdata.Job, 0, len(result.Jobs))
	skippedStale := 0
	for _, j := range result.Jobs {
		if !j.PostedAt.IsZero() && j.PostedAt.UTC().Before(cutoff) {
			skippedStale++
			continue
		}
		fresh = append(fresh, j)
	}
	if skippedStale > 0 {
		s.logger().InfoContext(ctx, "companies: skipped stale board jobs",
			"company_key", companyKey,
			"skipped_stale", skippedStale,
		)
	}
	if err := s.jobs.UpsertBoardJobs(ctx, companyKey, fresh); err != nil {
		return 0, 0, fmt.Errorf("companies: upsert board jobs: %w", err)
	}
	upserted = len(fresh)
	seen := make([]string, 0, len(fresh))
	for _, j := range fresh {
		if j.ID != "" {
			seen = append(seen, j.ID)
		}
	}
	if _, err := s.jobs.ReopenPresent(ctx, companyKey, seen); err != nil {
		return 0, 0, fmt.Errorf("companies: reopen present: %w", err)
	}
	if closeAbsent {
		n, err := s.jobs.CloseAbsent(ctx, companyKey, seen)
		if err != nil {
			return 0, 0, fmt.Errorf("companies: close absent: %w", err)
		}
		closed = int(n)
	}
	return upserted, closed, nil
}

// logEmptyBoardFallback implements I7: when a complete empty poll finds zero
// board jobs but the aggregator still has open rows for the company, keep the
// aggregator path and log the discrepancy.
func (s *Service) logEmptyBoardFallback(ctx context.Context, companyKey string) {
	if s.jobs == nil {
		return
	}
	n, err := s.jobs.CountOpenByCompanyKeyRef(ctx, companyKey)
	if err != nil {
		s.logger().WarnContext(ctx, "companies: aggregator job count failed",
			"failure_stage", schema.FailureStagePoll.String(),
			"company_key", companyKey,
			"error", err.Error(),
		)
		return
	}
	if n <= 0 {
		return
	}
	s.logger().WarnContext(ctx, "companies: empty board while aggregator still has jobs",
		"failure_stage", schema.FailureStagePoll.String(),
		"company_key", companyKey,
		"board_jobs", 0,
		"aggregator_jobs", n,
	)
}

func preferETag(next, prev string) string {
	if next != "" {
		return next
	}
	return prev
}

func (s *Service) logger() *slog.Logger {
	if s != nil && s.log != nil {
		return s.log
	}
	return slog.Default()
}
