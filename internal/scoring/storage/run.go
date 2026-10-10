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
}

// TableName implements schema.Tabler.
func (ProfileRun) TableName() string { return "profile_runs" }

func newProfileRun(run schema.Run) ProfileRun {
	row := ProfileRun{
		ID:             run.ID,
		ProfileID:      run.ProfileID,
		Status:         string(run.Status),
		StartedAt:      run.StartedAt.UTC(),
		JobsScored:     run.JobsScored,
		SourcesSkipped: run.SourcesSkipped,
		IdempotencyKey: run.IdempotencyKey.String(),
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
		ID:             m.ID,
		ProfileID:      m.ProfileID,
		Status:         schema.RunStatus(m.Status),
		StartedAt:      m.StartedAt,
		FinishedAt:     m.FinishedAt,
		JobsScored:     m.JobsScored,
		SourcesSkipped: m.SourcesSkipped,
		IdempotencyKey: key,
	}
	return run, nil
}
