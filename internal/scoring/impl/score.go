package impl

import (
	"context"
	"errors"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
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
	postedBefore := started.Add(-time.Duration(schema.PostedAtRejectDays) * 24 * time.Hour)
	listed, err := s.jobs.ListFirstSeenSince(ctx, since)
	if err != nil {
		return 0, err
	}
	stale, err := s.jobs.ListPostedBefore(ctx, postedBefore)
	if err != nil {
		return 0, err
	}
	rejectDays := schema.PostedAtRejectDays
	toScore := make([]jobdata.Job, 0, len(listed)+len(stale))
	for _, job := range scoreJobs(listed, stale) {
		if job.FirstSeenAt.UTC().Before(since) && !postedAtStale(job.PostedAt, started, rejectDays) {
			continue
		}
		toScore = append(toScore, job)
	}
	verdicts, err := s.loadVerdicts(ctx, toScore)
	if err != nil {
		return 0, err
	}
	scored := 0
	for _, job := range toScore {
		out := evaluateAt(profile, job, started, verdictPtr(verdicts, job.ID))
		if err := s.runs.UpsertMatch(ctx, matchFrom(profileID, runID, job, out)); err != nil {
			return scored, err
		}
		scored++
	}
	return scored, nil
}

// RescoreJobs re-evaluates existing matches for the given jobs after a verdict is stored.
// It is not on scoring.API. UpsertMatch leaves user_status unchanged.
func (s *Service) RescoreJobs(ctx context.Context, jobIDs []string) error {
	if s == nil || s.profiles == nil || s.runs == nil || s.jobs == nil {
		return errors.New("scoring: profiles, runs, and jobs are required")
	}
	if len(jobIDs) == 0 {
		return nil
	}
	matches, err := s.runs.ListMatchesByJobIDs(ctx, jobIDs)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	jobsByID := make(map[string]jobdata.Job, len(jobIDs))
	toLoad := make([]jobdata.Job, 0, len(jobIDs))
	for _, m := range matches {
		if _, ok := jobsByID[m.JobID]; ok {
			continue
		}
		job, err := s.jobs.GetByID(ctx, m.JobID)
		if err != nil {
			return err
		}
		jobsByID[m.JobID] = job
		toLoad = append(toLoad, job)
	}
	verdicts, err := s.loadVerdicts(ctx, toLoad)
	if err != nil {
		return err
	}

	profiles := make(map[string]profileschema.Profile)
	runStarted := make(map[int64]time.Time)
	for _, m := range matches {
		profile, ok := profiles[m.ProfileID]
		if !ok {
			profile, err = s.profiles.Get(ctx, m.ProfileID)
			if err != nil {
				return err
			}
			profiles[m.ProfileID] = profile
		}
		started, ok := runStarted[m.RunID]
		if !ok {
			run, err := s.runs.GetRun(ctx, m.RunID)
			if err != nil {
				return err
			}
			started = run.StartedAt.UTC()
			runStarted[m.RunID] = started
		}
		job := jobsByID[m.JobID]
		out := evaluateAt(profile, job, started, verdictPtr(verdicts, job.ID))
		if err := s.runs.UpsertMatch(ctx, matchFrom(m.ProfileID, m.RunID, job, out)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadVerdicts(ctx context.Context, jobs []jobdata.Job) (map[string]enrichschema.Verdict, error) {
	if s.verdicts == nil || len(jobs) == 0 {
		return nil, nil
	}
	return s.verdicts.ReadyByJobs(ctx, jobs)
}

func verdictPtr(verdicts map[string]enrichschema.Verdict, jobID string) *enrichschema.Verdict {
	if verdicts == nil {
		return nil
	}
	v, ok := verdicts[jobID]
	if !ok {
		return nil
	}
	return &v
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
