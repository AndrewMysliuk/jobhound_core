package jobs_activities

import (
	"context"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobsschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	jobutils "github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/google/uuid"
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

func (r *retentionJobsStub) DeleteJobsCreatedBeforeUTC(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoff = cutoff
	return r.n, nil
}

func (r *retentionJobsStub) UpsertSlotJob(context.Context, uuid.UUID, string) error { return nil }

func (r *retentionJobsStub) ListSlotJobsPassedStage1(context.Context, uuid.UUID) ([]jobdata.Job, error) {
	return nil, nil
}

func (r *retentionJobsStub) ListPassedStage2JobsForRun(context.Context, int64) ([]jobdata.Job, error) {
	return nil, nil
}

func (r *retentionJobsStub) ListSlotStage1Jobs(context.Context, uuid.UUID, int, int) ([]jobsschema.JobListEntry, int64, error) {
	return nil, 0, nil
}

func (r *retentionJobsStub) ListPipelineRunStage2Jobs(context.Context, uuid.UUID, int64, string, int, int) ([]jobsschema.JobListEntry, int64, error) {
	return nil, 0, nil
}

func (r *retentionJobsStub) ListPipelineRunStage3Jobs(context.Context, uuid.UUID, int64, string, int, int) ([]jobsschema.JobListEntry, int64, error) {
	return nil, 0, nil
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
	stub := &retentionJobsStub{n: 3}
	a := &RetentionActivities{
		Clock: func() time.Time { return fixed },
		Jobs:  stub,
		Log:   logging.Nop(),
	}
	out, err := a.RunJobRetention(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out.Deleted != 3 {
		t.Fatalf("Deleted = %d", out.Deleted)
	}
	wantCutoff := fixed.UTC().Add(-jobutils.Days * 24 * time.Hour)
	if !stub.cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff = %v, want %v", stub.cutoff, wantCutoff)
	}
}
