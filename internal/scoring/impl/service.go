package impl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

// Service is the scoring API handlers call, and the score pass the workflow activity calls.
type Service struct {
	profiles profiles.Store
	runs     runStore
	jobs     jobWindow
	starter  scoring.RunStarter
	now      func() time.Time
}

type runStore interface {
	CreateRun(ctx context.Context, run schema.Run) (schema.Run, error)
	RunByIdempotencyKey(ctx context.Context, key uuid.UUID) (schema.Run, error)
	RunningRun(ctx context.Context, profileID string) (schema.Run, error)
	LatestRun(ctx context.Context, profileID string) (schema.Run, error)
	GetRun(ctx context.Context, id int64) (schema.Run, error)
	FinishRun(ctx context.Context, id int64, status schema.RunStatus, jobsScored, sourcesSkipped int, finishedAt time.Time) error
	UpsertMatch(ctx context.Context, m schema.Match) error
	List(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error)
	SetUserStatus(ctx context.Context, profileID, jobID string, status schema.UserStatus) (schema.ListedJob, error)
}

type jobWindow interface {
	ListFirstSeenSince(ctx context.Context, since time.Time) ([]jobdata.Job, error)
}

// New wires the profile file store, scoring persistence, the jobs repository, and the workflow starter.
func New(profilesStore profiles.Store, runs runStore, jobs jobWindow, starter scoring.RunStarter) *Service {
	return &Service{
		profiles: profilesStore,
		runs:     runs,
		jobs:     jobs,
		starter:  starter,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

var (
	_ scoring.API = (*Service)(nil)
	_ runStore    = (*storage.Repository)(nil)
	_ jobWindow   = jobs.JobRepository(nil)
)

// StartRun inserts a RUNNING row and starts the workflow.
// The same idempotency key returns the existing run. A second open run is rejected.
func (s *Service) StartRun(ctx context.Context, profileID string, idempotencyKey uuid.UUID) (schema.Run, error) {
	if _, err := s.profiles.Get(ctx, profileID); err != nil {
		return schema.Run{}, err
	}
	existing, err := s.replay(ctx, profileID, idempotencyKey)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, storage.ErrRunNotFound) {
		return schema.Run{}, err
	}
	if _, err := s.runs.RunningRun(ctx, profileID); err == nil {
		return schema.Run{}, scoring.ErrRunAlreadyRunning
	} else if !errors.Is(err, storage.ErrRunNotFound) {
		return schema.Run{}, err
	}

	created, err := s.runs.CreateRun(ctx, schema.Run{
		ProfileID:      profileID,
		Status:         schema.RunStatusRunning,
		StartedAt:      s.now(),
		IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, storage.ErrDuplicateIdempotencyKey) {
		return s.replay(ctx, profileID, idempotencyKey)
	}
	if err != nil {
		return schema.Run{}, err
	}
	if s.starter == nil {
		return schema.Run{}, errors.New("scoring: run starter is required")
	}
	if err := s.starter.Start(ctx, profileID, created.ID); err != nil {
		finished := s.now()
		if finErr := s.runs.FinishRun(ctx, created.ID, schema.RunStatusFailed, 0, 0, finished); finErr != nil {
			return schema.Run{}, errors.Join(err, finErr)
		}
		if errors.Is(err, scoring.ErrRunAlreadyRunning) {
			return schema.Run{}, scoring.ErrRunAlreadyRunning
		}
		return schema.Run{}, err
	}
	return created, nil
}

func (s *Service) replay(ctx context.Context, profileID string, key uuid.UUID) (schema.Run, error) {
	existing, err := s.runs.RunByIdempotencyKey(ctx, key)
	if err != nil {
		return schema.Run{}, err
	}
	if existing.ProfileID != profileID {
		return schema.Run{}, scoring.ErrIdempotencyKeyConflict
	}
	return existing, nil
}

// LatestRun returns the newest run for the profile.
func (s *Service) LatestRun(ctx context.Context, profileID string) (schema.Run, error) {
	run, err := s.runs.LatestRun(ctx, profileID)
	if errors.Is(err, storage.ErrRunNotFound) {
		return schema.Run{}, scoring.ErrNoRun
	}
	if err != nil {
		return schema.Run{}, err
	}
	return run, nil
}

// ListJobs lists matches. An empty bucket means PASSED. An empty user status means NEW.
func (s *Service) ListJobs(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error) {
	if p.Bucket == "" {
		p.Bucket = schema.BucketPassed
	}
	if p.UserStatus == "" {
		p.UserStatus = schema.UserStatusNew
	}
	return s.runs.List(ctx, p)
}

// SetUserStatus updates one match. A job this profile has not scored is out of scope.
func (s *Service) SetUserStatus(ctx context.Context, profileID, jobID string, status schema.UserStatus) (schema.ListedJob, error) {
	job, err := s.runs.SetUserStatus(ctx, profileID, jobID, status)
	if errors.Is(err, storage.ErrMatchNotFound) {
		return schema.ListedJob{}, scoring.ErrJobNotInScope
	}
	if err != nil {
		return schema.ListedJob{}, err
	}
	return job, nil
}
