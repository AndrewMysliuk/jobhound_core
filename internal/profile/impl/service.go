// Package impl implements global profile GET/PUT (009).
package impl

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/profile"
	profilestorage "github.com/andrewmysliuk/jobhound_core/internal/profile/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	slotstorage "github.com/andrewmysliuk/jobhound_core/internal/slots/storage"
	"github.com/google/uuid"
)

var _ profile.API = (*Service)(nil)

// Service loads and updates profile text; PUT invalidates stage-3 snapshots per slot (008 / filter-invalidation.md).
type Service struct {
	Text  *profilestorage.Repository
	Runs  pipeline.PipelineRunRepository
	Slots *slotstorage.Repository
}

// NewService constructs a profile service.
func NewService(text *profilestorage.Repository, runs pipeline.PipelineRunRepository, slots *slotstorage.Repository) *Service {
	return &Service{Text: text, Runs: runs, Slots: slots}
}

// Get implements [profile.API.Get].
func (s *Service) Get(ctx context.Context) (schema.ProfileResponse, error) {
	row, err := s.Text.Get(ctx)
	if err != nil {
		return schema.ProfileResponse{}, err
	}
	return schema.ProfileResponse{Text: row.Text, UpdatedAt: row.UpdatedAt.UTC()}, nil
}

// Put implements [profile.API.Put].
func (s *Service) Put(ctx context.Context, text string) (schema.ProfileResponse, error) {
	updatedAt, err := s.Text.Set(ctx, text)
	if err != nil {
		return schema.ProfileResponse{}, err
	}
	rows, err := s.Slots.List(ctx)
	if err != nil {
		return schema.ProfileResponse{}, err
	}
	for _, sl := range rows {
		u, err := uuid.Parse(sl.ID)
		if err != nil {
			continue
		}
		if _, err := s.Runs.InvalidateStage3SnapshotsForSlot(ctx, u); err != nil {
			return schema.ProfileResponse{}, err
		}
	}
	return schema.ProfileResponse{Text: text, UpdatedAt: updatedAt.UTC()}, nil
}

// GetText returns stored profile text for stage-3 workflow input ([slots.impl.Service]).
func (s *Service) GetText(ctx context.Context) (string, error) {
	row, err := s.Text.Get(ctx)
	if err != nil {
		return "", err
	}
	return row.Text, nil
}
