package impl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

// Service is the scoring API handlers call, and the score pass the workflow activity calls.
type Service struct {
	profiles    profiles.Store
	runs        runStore
	jobs        jobWindow
	starter     scoring.RunStarter
	verdicts    verdictLookup
	factsStatus factsStatusLookup
	now         func() time.Time
}

// verdictLookup returns ready verdicts keyed by job id. Omitted ids are pending.
// A nil lookup on Service means every job is pending (phrase-only).
type verdictLookup interface {
	ReadyByJobs(ctx context.Context, jobs []jobdata.Job) (map[string]enrichschema.Verdict, error)
}

// factsStatusLookup is enrich Statuses for page reads. A nil lookup means enrich is off.
type factsStatusLookup interface {
	Statuses(ctx context.Context, jobIDs []string) (map[string]enrichschema.FactsStatus, error)
}

type runStore interface {
	CreateRun(ctx context.Context, run schema.Run) (schema.Run, error)
	RunByIdempotencyKey(ctx context.Context, key uuid.UUID) (schema.Run, error)
	RunningRun(ctx context.Context, profileID string) (schema.Run, error)
	LatestRun(ctx context.Context, profileID string) (schema.Run, error)
	GetRun(ctx context.Context, id int64) (schema.Run, error)
	FinishRun(ctx context.Context, id int64, status schema.RunStatus, jobsScored, sourcesSkipped int, finishedAt time.Time) error
	UpsertMatch(ctx context.Context, m schema.Match) error
	ListMatchesByJobIDs(ctx context.Context, jobIDs []string) ([]schema.Match, error)
	List(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error)
	SetUserStatus(ctx context.Context, profileID, jobID string, status schema.UserStatus) (schema.ListedJob, error)
}

type jobWindow interface {
	GetByID(ctx context.Context, id string) (jobdata.Job, error)
	ListFirstSeenSince(ctx context.Context, since time.Time) ([]jobdata.Job, error)
	ListPostedBefore(ctx context.Context, cutoff time.Time) ([]jobdata.Job, error)
}

// New wires the profile file store, scoring persistence, the jobs repository, and the workflow starter.
// verdicts may be nil: Score then treats every job as pending and adds no fact signals.
// factsStatus may be nil: List and SetUserStatus then set FactsStatus to off without querying.
func New(profilesStore profiles.Store, runs runStore, jobs jobWindow, starter scoring.RunStarter, verdicts verdictLookup, factsStatus factsStatusLookup) *Service {
	return &Service{
		profiles:    profilesStore,
		runs:        runs,
		jobs:        jobs,
		starter:     starter,
		verdicts:    verdicts,
		factsStatus: factsStatus,
		now:         func() time.Time { return time.Now().UTC() },
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

// ListJobs lists matches. An empty bucket means TOP. An empty user status means NEW.
func (s *Service) ListJobs(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error) {
	if p.Bucket == "" {
		p.Bucket = schema.BucketTop
	}
	if p.UserStatus == "" {
		p.UserStatus = schema.UserStatusNew
	}
	page, err := s.runs.List(ctx, p)
	if err != nil {
		return schema.JobPage{}, err
	}
	if err := s.fillFactsStatuses(ctx, page.Jobs); err != nil {
		return schema.JobPage{}, err
	}
	return page, nil
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
	jobs := []schema.ListedJob{job}
	if err := s.fillFactsStatuses(ctx, jobs); err != nil {
		return schema.ListedJob{}, err
	}
	return jobs[0], nil
}

func (s *Service) fillFactsStatuses(ctx context.Context, jobs []schema.ListedJob) error {
	if len(jobs) == 0 {
		return nil
	}
	if s.factsStatus == nil {
		for i := range jobs {
			jobs[i].FactsStatus = enrichschema.FactsStatusOff.String()
		}
		return nil
	}
	ids := make([]string, len(jobs))
	for i, job := range jobs {
		ids[i] = job.JobID
	}
	got, err := s.factsStatus.Statuses(ctx, ids)
	if err != nil {
		return err
	}
	for i := range jobs {
		if st, ok := got[jobs[i].JobID]; ok {
			jobs[i].FactsStatus = st.String()
			continue
		}
		jobs[i].FactsStatus = enrichschema.FactsStatusPending.String()
	}
	return nil
}
