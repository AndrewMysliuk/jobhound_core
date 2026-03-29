// Package jobs is the jobs module: persistence contracts at the root, storage under storage/.
package jobs

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// JobRepository persists normalized jobs (002 stub; list/search/ingest batch APIs in 006 as needed).
type JobRepository interface {
	Save(ctx context.Context, job domain.Job) error
	GetByID(ctx context.Context, id string) (domain.Job, error)
}
