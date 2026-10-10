package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

func TestStartRun(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	key := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	other := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	t.Run("starts one run", func(t *testing.T) {
		store := &fakeRuns{}
		starter := &fakeStarter{}
		svc := newTestService(store, starter, now)
		got, err := svc.StartRun(context.Background(), "andrew", key)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != 1 || got.Status != schema.RunStatusRunning || !got.StartedAt.Equal(now) {
			t.Fatalf("run = %#v", got)
		}
		if starter.calls != 1 || starter.runID != 1 {
			t.Fatalf("starter calls=%d runID=%d", starter.calls, starter.runID)
		}
	})

	t.Run("second open run", func(t *testing.T) {
		store := &fakeRuns{}
		starter := &fakeStarter{}
		svc := newTestService(store, starter, now)
		if _, err := svc.StartRun(context.Background(), "andrew", key); err != nil {
			t.Fatal(err)
		}
		_, err := svc.StartRun(context.Background(), "andrew", other)
		if !errors.Is(err, scoring.ErrRunAlreadyRunning) {
			t.Fatalf("err = %v", err)
		}
		if starter.calls != 1 {
			t.Fatalf("starter calls = %d", starter.calls)
		}
	})

	t.Run("idempotency replay", func(t *testing.T) {
		store := &fakeRuns{}
		starter := &fakeStarter{}
		svc := newTestService(store, starter, now)
		first, err := svc.StartRun(context.Background(), "andrew", key)
		if err != nil {
			t.Fatal(err)
		}
		again, err := svc.StartRun(context.Background(), "andrew", key)
		if err != nil {
			t.Fatal(err)
		}
		if again.ID != first.ID {
			t.Fatalf("replay id = %d, want %d", again.ID, first.ID)
		}
		if starter.calls != 1 {
			t.Fatalf("starter calls = %d", starter.calls)
		}
	})

	t.Run("key reused for another profile", func(t *testing.T) {
		store := &fakeRuns{}
		starter := &fakeStarter{}
		svc := newTestService(store, starter, now)
		if _, err := svc.StartRun(context.Background(), "andrew", key); err != nil {
			t.Fatal(err)
		}
		store.runs[0].Status = schema.RunStatusSucceeded
		_, err := svc.StartRun(context.Background(), "other", key)
		if !errors.Is(err, scoring.ErrIdempotencyKeyConflict) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestLatestRun_none(t *testing.T) {
	svc := newTestService(&fakeRuns{}, &fakeStarter{}, time.Now().UTC())
	_, err := svc.LatestRun(context.Background(), "andrew")
	if !errors.Is(err, scoring.ErrNoRun) {
		t.Fatalf("err = %v", err)
	}
}

func TestListJobs_defaults(t *testing.T) {
	store := &fakeRuns{}
	svc := newTestService(store, &fakeStarter{}, time.Now().UTC())
	if _, err := svc.ListJobs(context.Background(), schema.ListJobsParams{ProfileID: "andrew"}); err != nil {
		t.Fatal(err)
	}
	if store.listed.Bucket != schema.BucketPassed || store.listed.UserStatus != schema.UserStatusNew {
		t.Fatalf("params = %#v", store.listed)
	}
}

func TestSetUserStatus_notInScope(t *testing.T) {
	svc := newTestService(&fakeRuns{}, &fakeStarter{}, time.Now().UTC())
	_, err := svc.SetUserStatus(context.Background(), "andrew", "job-1", schema.UserStatusHidden)
	if !errors.Is(err, scoring.ErrJobNotInScope) {
		t.Fatalf("err = %v", err)
	}
}

func TestScore(t *testing.T) {
	started := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	window := time.Duration(schema.ScoreWindowDays) * 24 * time.Hour
	profile := profileschema.Profile{
		ID:           "andrew",
		Name:         "Andrew",
		ExcludeTitle: []string{"recruiter"},
		Queries:      []string{"vue"},
	}
	old := jobdata.Job{
		ID:          "old",
		Title:       "Vue Engineer",
		Description: "vue",
		FirstSeenAt: started.Add(-window).Add(-time.Second),
	}
	edge := jobdata.Job{
		ID:          "edge",
		Title:       "Vue Engineer",
		Description: "vue",
		FirstSeenAt: started.Add(-window),
	}
	rejected := jobdata.Job{
		ID:          "rejected",
		Title:       "Recruiter",
		Description: "vue",
		FirstSeenAt: started.Add(-24 * time.Hour),
	}
	fresh := jobdata.Job{
		ID:          "fresh",
		Title:       "Vue Engineer",
		Description: "vue",
		FirstSeenAt: started.Add(-24 * time.Hour),
	}
	hidden := schema.Match{
		ProfileID:  "andrew",
		JobID:      fresh.ID,
		Bucket:     schema.BucketPassed,
		Score:      4,
		UserStatus: schema.UserStatusHidden,
		RunID:      1,
	}

	tests := []struct {
		name      string
		jobs      []jobdata.Job
		seed      []schema.Match
		upsertErr error
		wantErr   bool
		wantN     int
		want      []schema.Match
	}{
		{
			name:  "older than 10 days is not scored",
			jobs:  []jobdata.Job{old, edge},
			wantN: 1,
			want: []schema.Match{{
				JobID:      edge.ID,
				Bucket:     schema.BucketPassed,
				Score:      1,
				UserStatus: schema.UserStatusNew,
			}},
		},
		{
			name:  "rejected row is stored",
			jobs:  []jobdata.Job{rejected},
			wantN: 1,
			want: []schema.Match{{
				JobID:      rejected.ID,
				Bucket:     schema.BucketRejected,
				Score:      0,
				UserStatus: schema.UserStatusNew,
				Signals: []schema.Signal{{
					Code:  schema.SignalCutExcludeTitle,
					Terms: []string{"recruiter"},
				}},
			}},
		},
		{
			name:  "hidden stays hidden after rescore",
			jobs:  []jobdata.Job{fresh},
			seed:  []schema.Match{hidden},
			wantN: 1,
			want: []schema.Match{{
				JobID:      fresh.ID,
				Bucket:     schema.BucketPassed,
				Score:      1,
				UserStatus: schema.UserStatusHidden,
			}},
		},
		{
			name:      "failed upsert keeps existing matches",
			jobs:      []jobdata.Job{fresh},
			seed:      []schema.Match{hidden},
			upsertErr: errors.New("write failed"),
			wantErr:   true,
			want: []schema.Match{{
				JobID:      fresh.ID,
				Bucket:     schema.BucketPassed,
				Score:      4,
				UserStatus: schema.UserStatusHidden,
				RunID:      1,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeRuns{
				runs: []schema.Run{{
					ID:        7,
					ProfileID: "andrew",
					Status:    schema.RunStatusRunning,
					StartedAt: started,
				}},
				matches:   append([]schema.Match(nil), tt.seed...),
				upsertErr: tt.upsertErr,
			}
			jobs := &fakeJobs{jobs: tt.jobs}
			svc := New(fakeProfiles{profile: profile}, store, jobs, nil)
			n, err := svc.Score(context.Background(), "andrew", 7)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if n != tt.wantN {
				t.Fatalf("scored = %d, want %d", n, tt.wantN)
			}
			wantSince := started.Add(-window)
			if !jobs.since.Equal(wantSince) {
				t.Fatalf("since = %s, want %s", jobs.since, wantSince)
			}
			if len(store.matches) != len(tt.want) {
				t.Fatalf("matches = %#v, want %#v", store.matches, tt.want)
			}
			for i, want := range tt.want {
				got := store.matches[i]
				if got.JobID != want.JobID || got.Bucket != want.Bucket || got.Score != want.Score || got.UserStatus != want.UserStatus {
					t.Fatalf("match = %#v, want %#v", got, want)
				}
				if want.RunID != 0 && got.RunID != want.RunID {
					t.Fatalf("run id = %d, want %d", got.RunID, want.RunID)
				}
				if want.Signals != nil && !sameSignals(got.Signals, want.Signals) {
					t.Fatalf("signals = %#v, want %#v", got.Signals, want.Signals)
				}
			}
		})
	}
}

func sameSignals(got, want []schema.Signal) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i].Code != want[i].Code || len(got[i].Terms) != len(want[i].Terms) {
			return false
		}
		for j := range want[i].Terms {
			if got[i].Terms[j] != want[i].Terms[j] {
				return false
			}
		}
	}
	return true
}

func newTestService(store *fakeRuns, starter *fakeStarter, now time.Time) *Service {
	svc := New(fakeProfiles{}, store, nil, starter)
	svc.now = func() time.Time { return now }
	return svc
}

type fakeProfiles struct {
	profile profileschema.Profile
}

func (fakeProfiles) List(context.Context) ([]profileschema.Profile, error) {
	return nil, nil
}

func (f fakeProfiles) Get(_ context.Context, id string) (profileschema.Profile, error) {
	if id == "missing" {
		return profileschema.Profile{}, profiles.ErrProfileNotFound
	}
	if f.profile.ID == id {
		return f.profile, nil
	}
	return profileschema.Profile{ID: id, Name: id}, nil
}

type fakeStarter struct {
	calls int
	runID int64
	err   error
}

func (f *fakeStarter) Start(_ context.Context, _ string, runID int64) error {
	f.calls++
	f.runID = runID
	return f.err
}

type fakeJobs struct {
	jobs  []jobdata.Job
	since time.Time
}

func (f *fakeJobs) ListFirstSeenSince(_ context.Context, since time.Time) ([]jobdata.Job, error) {
	f.since = since
	out := make([]jobdata.Job, 0, len(f.jobs))
	for _, job := range f.jobs {
		if job.FirstSeenAt.UTC().Before(since) {
			continue
		}
		out = append(out, job)
	}
	return out, nil
}

type fakeRuns struct {
	runs      []schema.Run
	matches   []schema.Match
	nextID    int64
	listed    schema.ListJobsParams
	upsertErr error
}

func (f *fakeRuns) CreateRun(_ context.Context, run schema.Run) (schema.Run, error) {
	for _, existing := range f.runs {
		if existing.IdempotencyKey == run.IdempotencyKey {
			return schema.Run{}, storage.ErrDuplicateIdempotencyKey
		}
	}
	f.nextID++
	run.ID = f.nextID
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *fakeRuns) RunByIdempotencyKey(_ context.Context, key uuid.UUID) (schema.Run, error) {
	for _, run := range f.runs {
		if run.IdempotencyKey == key {
			return run, nil
		}
	}
	return schema.Run{}, storage.ErrRunNotFound
}

func (f *fakeRuns) RunningRun(_ context.Context, profileID string) (schema.Run, error) {
	var found schema.Run
	ok := false
	for _, run := range f.runs {
		if run.ProfileID == profileID && run.Status == schema.RunStatusRunning && run.ID >= found.ID {
			found = run
			ok = true
		}
	}
	if !ok {
		return schema.Run{}, storage.ErrRunNotFound
	}
	return found, nil
}

func (f *fakeRuns) GetRun(_ context.Context, id int64) (schema.Run, error) {
	for _, run := range f.runs {
		if run.ID == id {
			return run, nil
		}
	}
	return schema.Run{}, storage.ErrRunNotFound
}

func (f *fakeRuns) LatestRun(_ context.Context, profileID string) (schema.Run, error) {
	var found schema.Run
	ok := false
	for _, run := range f.runs {
		if run.ProfileID == profileID && run.ID >= found.ID {
			found = run
			ok = true
		}
	}
	if !ok {
		return schema.Run{}, storage.ErrRunNotFound
	}
	return found, nil
}

func (f *fakeRuns) FinishRun(_ context.Context, id int64, status schema.RunStatus, jobsScored, sourcesSkipped int, finishedAt time.Time) error {
	for i := range f.runs {
		if f.runs[i].ID != id {
			continue
		}
		f.runs[i].Status = status
		f.runs[i].JobsScored = jobsScored
		f.runs[i].SourcesSkipped = sourcesSkipped
		t := finishedAt
		f.runs[i].FinishedAt = &t
		return nil
	}
	return storage.ErrRunNotFound
}

func (f *fakeRuns) UpsertMatch(_ context.Context, m schema.Match) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	for i := range f.matches {
		if f.matches[i].ProfileID == m.ProfileID && f.matches[i].JobID == m.JobID {
			status := f.matches[i].UserStatus
			f.matches[i] = m
			f.matches[i].UserStatus = status
			return nil
		}
	}
	if m.UserStatus == "" {
		m.UserStatus = schema.UserStatusNew
	}
	f.matches = append(f.matches, m)
	return nil
}

func (f *fakeRuns) List(_ context.Context, p schema.ListJobsParams) (schema.JobPage, error) {
	f.listed = p
	return schema.JobPage{}, nil
}

func (f *fakeRuns) SetUserStatus(context.Context, string, string, schema.UserStatus) (schema.ListedJob, error) {
	return schema.ListedJob{}, storage.ErrMatchNotFound
}
