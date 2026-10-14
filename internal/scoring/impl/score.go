package impl

import (
	"context"
	"errors"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// Score loads jobs first seen inside ScoreWindowDays before the run started,
// plus jobs whose posted_at is already older than PostedAtRejectDays.
// It evaluates each one and upserts the match. It does not delete existing matches.
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
	started := run.StartedAt.UTC()
	since := started.Add(-time.Duration(schema.ScoreWindowDays) * 24 * time.Hour)
	postedBefore := started.Add(-time.Duration(PostedAtRejectDaysForProfile(profile)) * 24 * time.Hour)
	listed, err := s.jobs.ListFirstSeenSince(ctx, since)
	if err != nil {
		return 0, err
	}
	stale, err := s.jobs.ListPostedBefore(ctx, postedBefore)
	if err != nil {
		return 0, err
	}
	rejectDays := PostedAtRejectDaysForProfile(profile)
	scored := 0
	for _, job := range scoreJobs(listed, stale) {
		if job.FirstSeenAt.UTC().Before(since) && !postedAtStale(job.PostedAt, started, rejectDays) {
			continue
		}
		if err := s.runs.UpsertMatch(ctx, matchFrom(profileID, runID, job, evaluateAt(profile, job, started))); err != nil {
			return scored, err
		}
		scored++
	}
	return scored, nil
}

func scoreJobs(inWindow, stale []jobdata.Job) []jobdata.Job {
	seen := make(map[string]struct{}, len(inWindow)+len(stale))
	out := make([]jobdata.Job, 0, len(inWindow)+len(stale))
	add := func(job jobdata.Job) {
		if job.ID == "" {
			return
		}
		if _, ok := seen[job.ID]; ok {
			return
		}
		seen[job.ID] = struct{}{}
		out = append(out, job)
	}
	for _, job := range inWindow {
		add(job)
	}
	for _, job := range stale {
		add(job)
	}
	return out
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
