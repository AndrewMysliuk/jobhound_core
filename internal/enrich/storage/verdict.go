package storage

import (
	"encoding/json"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

// JobVerdict is the GORM model for job_verdicts.
type JobVerdict struct {
	JobID         string    `gorm:"column:job_id;type:text;not null;uniqueIndex:job_verdicts_job_id_prompt_version_model_input_hash_key,priority:1"`
	PromptVersion int       `gorm:"column:prompt_version;not null;uniqueIndex:job_verdicts_job_id_prompt_version_model_input_hash_key,priority:2"`
	Model         string    `gorm:"column:model;type:text;not null;uniqueIndex:job_verdicts_job_id_prompt_version_model_input_hash_key,priority:3"`
	InputHash     string    `gorm:"column:input_hash;type:text;not null;uniqueIndex:job_verdicts_job_id_prompt_version_model_input_hash_key,priority:4"`
	Facts         []byte    `gorm:"column:facts;type:jsonb;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
}

// TableName implements schema.Tabler.
func (JobVerdict) TableName() string { return "job_verdicts" }

func newJobVerdict(v schema.Verdict, facts []byte, createdAt time.Time) JobVerdict {
	return JobVerdict{
		JobID:         v.JobID,
		PromptVersion: v.PromptVersion,
		Model:         v.Model,
		InputHash:     v.InputHash,
		Facts:         facts,
		CreatedAt:     createdAt.UTC(),
	}
}

func (m JobVerdict) toDomain() (schema.Verdict, error) {
	var facts schema.ModelReply
	if err := json.Unmarshal(m.Facts, &facts); err != nil {
		return schema.Verdict{}, err
	}
	return schema.Verdict{
		JobID:         m.JobID,
		PromptVersion: m.PromptVersion,
		Model:         m.Model,
		InputHash:     m.InputHash,
		Facts:         facts,
		CreatedAt:     m.CreatedAt,
	}, nil
}
