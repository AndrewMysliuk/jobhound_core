// Package impl implements slot use cases for the public HTTP API (009).
package impl

import (
	"context"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	manualworkflows "github.com/andrewmysliuk/jobhound_core/internal/manual/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotstorage "github.com/andrewmysliuk/jobhound_core/internal/slots/storage"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

var _ slots.API = (*Service)(nil)

// Service wires Postgres slots with Temporal stage-1 ingest (INGEST_SOURCES).
type Service struct {
	Repo      *slotstorage.Repository
	Jobs      jobs.JobRepository
	Runs      pipeline.PipelineRunRepository
	Profiles  profileTextLoader
	Temporal  slots.WorkflowTemporal
	TaskQueue string
	SourceIDs []string
}

// profileTextLoader loads trimmed global profile text for stage 3 (009).
type profileTextLoader interface {
	GetText(ctx context.Context) (string, error)
}

// NewService constructs a slot service. SourceIDs must be non-empty (same set as the worker’s collectors).
func NewService(repo *slotstorage.Repository, jobRepo jobs.JobRepository, runs pipeline.PipelineRunRepository, profiles profileTextLoader, tc slots.WorkflowTemporal, taskQueue string, sourceIDs []string) *Service {
	return &Service{
		Repo:      repo,
		Jobs:      jobRepo,
		Runs:      runs,
		Profiles:  profiles,
		Temporal:  tc,
		TaskQueue: taskQueue,
		SourceIDs: append([]string(nil), sourceIDs...),
	}
}

func ingestWorkflowID(slotID uuid.UUID) string {
	return "pubapi-slot-ingest-" + slotID.String()
}

// List implements [slots.API.List].
func (s *Service) List(ctx context.Context) (schema.SlotsListResponse, error) {
	rows, err := s.Repo.List(ctx)
	if err != nil {
		return schema.SlotsListResponse{}, err
	}
	out := make([]schema.SlotListItem, 0, len(rows))
	for _, row := range rows {
		card, err := s.card(ctx, row)
		if err != nil {
			return schema.SlotsListResponse{}, err
		}
		out = append(out, schema.SlotListItem{
			ID:        card.ID,
			Name:      card.Name,
			CreatedAt: card.CreatedAt,
			Stage1:    schema.StageCompact{State: card.Stage1.State},
			Stage2:    schema.StageCompact{State: card.Stage2.State},
			Stage3:    schema.StageCompact{State: card.Stage3.State},
		})
	}
	return schema.SlotsListResponse{Slots: out}, nil
}

// Create implements [slots.API.Create].
func (s *Service) Create(ctx context.Context, name string) (*schema.SlotCard, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, slots.ErrInvalidSlotName
	}
	n, err := s.Repo.Count(ctx)
	if err != nil {
		return nil, err
	}
	if n >= slotstorage.MaxSlots {
		return nil, slots.ErrSlotLimitReached
	}
	id := uuid.New()
	if err := s.Repo.Create(ctx, id, name); err != nil {
		return nil, err
	}
	in := manualschema.ManualSlotRunWorkflowInput{
		SlotID:          id,
		Kind:            manualschema.RunKindIngestSources,
		SourceIDs:       append([]string(nil), s.SourceIDs...),
		ExplicitRefresh: false,
	}
	if err := in.Validate(); err != nil {
		_ = s.Repo.Delete(ctx, id.String())
		return nil, err
	}
	_, err = s.Temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                 ingestWorkflowID(id),
		TaskQueue:          s.TaskQueue,
		WorkflowRunTimeout: manualworkflows.DefaultManualSlotRunWorkflowTimeout,
	}, manualschema.ManualSlotRunWorkflowName, in)
	if err != nil {
		_ = s.Repo.Delete(ctx, id.String())
		return nil, err
	}
	row, err := s.Repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, err
	}
	return s.card(ctx, row)
}

// Get implements [slots.API.Get].
func (s *Service) Get(ctx context.Context, slotID string) (*schema.SlotCard, error) {
	row, err := s.Repo.GetByID(ctx, slotID)
	if err != nil {
		return nil, err
	}
	return s.card(ctx, row)
}

// Delete implements [slots.API.Delete].
func (s *Service) Delete(ctx context.Context, slotID string) error {
	u, err := uuid.Parse(strings.TrimSpace(slotID))
	if err != nil {
		return slots.ErrNotFound
	}
	_ = s.Temporal.TerminateWorkflow(ctx, ingestWorkflowID(u), "", "slot deleted")
	_ = s.Temporal.TerminateWorkflow(ctx, stage2WorkflowID(u), "", "slot deleted")
	_ = s.Temporal.TerminateWorkflow(ctx, stage3WorkflowID(u), "", "slot deleted")
	return s.Repo.Delete(ctx, u.String())
}

func (s *Service) card(ctx context.Context, row slotstorage.Slot) (*schema.SlotCard, error) {
	uid, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, err
	}
	st1 := stage1FromDescribe(s.Temporal.DescribeWorkflow(ctx, ingestWorkflowID(uid), ""))
	st2 := stage2FromDescribe(s.Temporal.DescribeWorkflow(ctx, stage2WorkflowID(uid), ""))
	st3 := stage3FromDescribe(s.Temporal.DescribeWorkflow(ctx, stage3WorkflowID(uid), ""))
	return &schema.SlotCard{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt.UTC(),
		Stage1:    st1,
		Stage2:    st2,
		Stage3:    st3,
	}, nil
}
