package impl

import (
	"context"
	"errors"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// Score loads jobs first seen inside ScoreWindowDays before the run started,
// evaluates each one, and upserts the match. It does not delete existing matches.
func (s *Service) Score(ctx context.Context, profileID string, runID int64) (int, error) {
	if s == nil || s.profiles == nil || s.runs == nil || s.jobs == nil {
		return 0, errors.New("scoring: profiles, runs, and jobs are required")
	}
	profile, err := s.profiles.Get(ctx, profileID)
	if err != nil {
		return 0, err
	}
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil {
		return 0, err
	}
	since := run.StartedAt.UTC().Add(-time.Duration(schema.ScoreWindowDays) * 24 * time.Hour)
	listed, err := s.jobs.ListFirstSeenSince(ctx, since)
	if err != nil {
		return 0, err
	}
	scored := 0
	for _, job := range listed {
		if job.FirstSeenAt.UTC().Before(since) {
			continue
		}
		if err := s.runs.UpsertMatch(ctx, matchFrom(profileID, runID, job, Evaluate(profile, job))); err != nil {
			return scored, err
		}
		scored++
	}
	return scored, nil
}

func matchFrom(profileID string, runID int64, job jobdata.Job, out Outcome) schema.Match {
	signals := out.Signals
	if signals == nil {
		signals = []schema.Signal{}
	}
	return schema.Match{
		ProfileID: profileID,
		JobID:     job.ID,
		Bucket:    out.Bucket,
		Score:     out.Score,
		Signals:   signals,
		RunID:     runID,
	}
}
