package jobs_activities

import (
	"context"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobutils "github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

type retentionJobsStub struct {
	cutoff time.Time
	n      int64
}

func (r *retentionJobsStub) Save(context.Context, jobdata.Job) error { return nil }

func (r *retentionJobsStub) SaveIngest(context.Context, jobdata.Job) (bool, error) { return false, nil }

func (r *retentionJobsStub) GetByID(context.Context, string) (jobdata.Job, error) {
	return jobdata.Job{}, nil
}

func (r *retentionJobsStub) ListFirstSeenSince(context.Context, time.Time) ([]jobdata.Job, error) {
	return nil, nil
}

func (r *retentionJobsStub) DeleteJobsLastSeenBeforeUTC(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoff = cutoff
	return r.n, nil
}

var _ jobs.JobRepository = (*retentionJobsStub)(nil)

func TestRunJobRetention_requiresJobs(t *testing.T) {
	a := &RetentionActivities{}
	_, err := a.RunJobRetention(context.Background())
	if err == nil {
		t.Fatal("expected error when Jobs is nil")
	}
}

func TestRunJobRetention_usesClockAndCutoff(t *testing.T) {
	fixed := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	for _, days := range []int{0, config.DefaultJobRetentionDays, 15} {
		stub := &retentionJobsStub{n: 3}
		a := &RetentionActivities{
			Clock:            func() time.Time { return fixed },
			Jobs:             stub,
			Log:              logging.Nop(),
			JobRetentionDays: days,
		}
		out, err := a.RunJobRetention(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if out.Deleted != 3 {
			t.Fatalf("days %d: Deleted = %d", days, out.Deleted)
		}
		wantCutoff := jobutils.CutoffUTC(fixed, config.Config{JobRetentionDays: days})
		if !stub.cutoff.Equal(wantCutoff) {
			t.Fatalf("days %d: cutoff = %v, want %v", days, stub.cutoff, wantCutoff)
		}
	}
}
