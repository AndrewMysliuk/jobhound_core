package storage

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/google/uuid"
)

// ProfileRun is the GORM model for profile_runs.
type ProfileRun struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	ProfileID      string     `gorm:"column:profile_id;type:text;not null"`
	Status         string     `gorm:"column:status;type:text;not null"`
	StartedAt      time.Time  `gorm:"column:started_at;not null"`
	FinishedAt     *time.Time `gorm:"column:finished_at"`
	JobsScored     int        `gorm:"column:jobs_scored;not null;default:0"`
	SourcesSkipped int        `gorm:"column:sources_skipped;not null;default:0"`
	IdempotencyKey string     `gorm:"column:idempotency_key;type:uuid;not null;uniqueIndex"`

	CompaniesDiscovered  int `gorm:"column:companies_discovered;not null;default:0"`
	CompaniesMerged      int `gorm:"column:companies_merged;not null;default:0"`
	CompaniesBlocked     int `gorm:"column:companies_blocked;not null;default:0"`
	CompaniesResolved    int `gorm:"column:companies_resolved;not null;default:0"`
	CompaniesUnsupported int `gorm:"column:companies_unsupported;not null;default:0"`
	CompaniesPolled      int `gorm:"column:companies_polled;not null;default:0"`
	PollsNotModified     int `gorm:"column:polls_not_modified;not null;default:0"`
	PollsPartial         int `gorm:"column:polls_partial;not null;default:0"`
	PollFailures         int `gorm:"column:poll_failures;not null;default:0"`
	JobsUpsertedATS      int `gorm:"column:jobs_upserted_ats;not null;default:0"`
	JobsClosed           int `gorm:"column:jobs_closed;not null;default:0"`
}

// TableName implements schema.Tabler.
func (ProfileRun) TableName() string { return "profile_runs" }

func newProfileRun(run schema.Run) ProfileRun {
	row := ProfileRun{
		ID:                   run.ID,
		ProfileID:            run.ProfileID,
		Status:               string(run.Status),
		StartedAt:            run.StartedAt.UTC(),
		JobsScored:           run.JobsScored,
		SourcesSkipped:       run.SourcesSkipped,
		IdempotencyKey:       run.IdempotencyKey.String(),
		CompaniesDiscovered:  run.CompaniesDiscovered,
		CompaniesMerged:      run.CompaniesMerged,
		CompaniesBlocked:     run.CompaniesBlocked,
		CompaniesResolved:    run.CompaniesResolved,
		CompaniesUnsupported: run.CompaniesUnsupported,
		CompaniesPolled:      run.CompaniesPolled,
		PollsNotModified:     run.PollsNotModified,
		PollsPartial:         run.PollsPartial,
		PollFailures:         run.PollFailures,
		JobsUpsertedATS:      run.JobsUpsertedATS,
		JobsClosed:           run.JobsClosed,
	}
	if run.FinishedAt != nil {
		t := run.FinishedAt.UTC()
		row.FinishedAt = &t
	}
	return row
}

func (m ProfileRun) toDomain() (schema.Run, error) {
	key, err := uuid.Parse(m.IdempotencyKey)
	if err != nil {
		return schema.Run{}, err
	}
	run := schema.Run{
		ID:                   m.ID,
		ProfileID:            m.ProfileID,
		Status:               schema.RunStatus(m.Status),
		StartedAt:            m.StartedAt,
		FinishedAt:           m.FinishedAt,
		JobsScored:           m.JobsScored,
		SourcesSkipped:       m.SourcesSkipped,
		IdempotencyKey:       key,
		CompaniesDiscovered:  m.CompaniesDiscovered,
		CompaniesMerged:      m.CompaniesMerged,
		CompaniesBlocked:     m.CompaniesBlocked,
		CompaniesResolved:    m.CompaniesResolved,
		CompaniesUnsupported: m.CompaniesUnsupported,
		CompaniesPolled:      m.CompaniesPolled,
		PollsNotModified:     m.PollsNotModified,
		PollsPartial:         m.PollsPartial,
		PollFailures:         m.PollFailures,
		JobsUpsertedATS:      m.JobsUpsertedATS,
		JobsClosed:           m.JobsClosed,
	}
	return run, nil
}
