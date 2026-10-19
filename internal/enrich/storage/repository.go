package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound is returned when Verdict has no row for the exact key.
var ErrNotFound = errors.New("verdict not found")

// Repository persists job_verdicts rows.
type Repository struct {
	get pgsql.GormGetter
}

// NewRepository wires enrich persistence.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// Insert stores one coerced reply. A conflict on the unique key is a no-op.
func (r *Repository) Insert(ctx context.Context, v schema.Verdict) error {
	if v.JobID == "" {
		return fmt.Errorf("job id is required")
	}
	if v.Model == "" {
		return fmt.Errorf("model is required")
	}
	if v.InputHash == "" {
		return fmt.Errorf("input hash is required")
	}
	facts, err := json.Marshal(v.Facts)
	if err != nil {
		return err
	}
	createdAt := v.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	row := newJobVerdict(v, facts, createdAt)
	return r.get().WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "job_id"},
				{Name: "prompt_version"},
				{Name: "model"},
				{Name: "input_hash"},
			},
			DoNothing: true,
		}).
		Create(&row).Error
}

// Verdict loads the row for the exact (job_id, prompt_version, model, input_hash) key.
func (r *Repository) Verdict(ctx context.Context, jobID, model string, promptVersion int, inputHash string) (schema.Verdict, error) {
	if jobID == "" {
		return schema.Verdict{}, fmt.Errorf("job id is required")
	}
	if model == "" {
		return schema.Verdict{}, fmt.Errorf("model is required")
	}
	if inputHash == "" {
		return schema.Verdict{}, fmt.Errorf("input hash is required")
	}
	var row JobVerdict
	err := r.get().WithContext(ctx).
		Where("job_id = ? AND prompt_version = ? AND model = ? AND input_hash = ?",
			jobID, promptVersion, model, inputHash).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.Verdict{}, ErrNotFound
		}
		return schema.Verdict{}, err
	}
	return row.toDomain()
}

// MissingIDs returns job ids from currentHashes that have no row for this
// prompt version, model, and that job's current hash. Pending is a missing row.
func (r *Repository) MissingIDs(ctx context.Context, model string, promptVersion int, currentHashes map[string]string) ([]string, error) {
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if len(currentHashes) == 0 {
		return nil, nil
	}

	ready, err := r.readyJobIDs(ctx, model, promptVersion, currentHashes)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(currentHashes)-len(ready))
	for jobID := range currentHashes {
		if _, ok := ready[jobID]; ok {
			continue
		}
		out = append(out, jobID)
	}
	return out, nil
}

// Statuses returns pending or ready for each job id in currentHashes.
// ready means a row exists for this prompt version, model, and current hash.
func (r *Repository) Statuses(ctx context.Context, model string, promptVersion int, currentHashes map[string]string) (map[string]schema.FactsStatus, error) {
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	out := make(map[string]schema.FactsStatus, len(currentHashes))
	if len(currentHashes) == 0 {
		return out, nil
	}

	ready, err := r.readyJobIDs(ctx, model, promptVersion, currentHashes)
	if err != nil {
		return nil, err
	}
	for jobID := range currentHashes {
		if _, ok := ready[jobID]; ok {
			out[jobID] = schema.FactsStatusReady
			continue
		}
		out[jobID] = schema.FactsStatusPending
	}
	return out, nil
}

func (r *Repository) readyJobIDs(ctx context.Context, model string, promptVersion int, currentHashes map[string]string) (map[string]struct{}, error) {
	jobIDs := make([]string, 0, len(currentHashes))
	for jobID, hash := range currentHashes {
		if jobID == "" || hash == "" {
			return nil, fmt.Errorf("job id and input hash are required")
		}
		jobIDs = append(jobIDs, jobID)
	}

	var rows []JobVerdict
	err := r.get().WithContext(ctx).
		Select("job_id", "input_hash").
		Where("prompt_version = ? AND model = ?", promptVersion, model).
		Where("job_id IN ?", jobIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	ready := make(map[string]struct{})
	for _, row := range rows {
		want, ok := currentHashes[row.JobID]
		if !ok || row.InputHash != want {
			continue
		}
		ready[row.JobID] = struct{}{}
	}
	return ready, nil
}
