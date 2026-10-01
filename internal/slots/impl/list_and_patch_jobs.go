package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	jobschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
	"github.com/google/uuid"
)

// normalizeListStatusFilter maps GET ?status= to prj.stage2_status or prj.stage3_status. Empty means no filter (all rows for that stage list).
func normalizeListStatusFilter(stage int, raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if stage == 2 && s == pipelineschema.Stage2ListFilterEligible {
		return pipelineschema.Stage2ListFilterEligible, nil
	}
	st := pipelineschema.RunJobStatus(s)
	if !st.Valid() {
		return "", fmt.Errorf("invalid status")
	}
	switch stage {
	case 2:
		if st != pipelineschema.RunJobPassedStage2 && st != pipelineschema.RunJobRejectedStage2 && st != pipelineschema.RunJobUnknownStage2 {
			return "", fmt.Errorf("status not valid for stage 2 list")
		}
	case 3:
		if st != pipelineschema.RunJobPassedStage3 && st != pipelineschema.RunJobRejectedStage3 {
			return "", fmt.Errorf("status not valid for stage 3 list")
		}
	default:
		return "", fmt.Errorf("invalid stage")
	}
	return s, nil
}

// ListJobs implements [slots.API.ListJobs].
func (s *Service) ListJobs(ctx context.Context, p slotschema.ListJobsParams) (schema.JobListResponse, error) {
	log := s.methodLog(ctx, "ListJobs")
	log.Debug().Msg("list jobs")
	if s.Jobs == nil {
		return schema.JobListResponse{}, errors.New("slots service: jobs repository is required for job lists")
	}
	u, err := uuid.Parse(strings.TrimSpace(p.SlotID))
	if err != nil {
		return schema.JobListResponse{}, slots.ErrNotFound
	}
	if _, err := s.Repo.GetByID(ctx, u.String()); err != nil {
		return schema.JobListResponse{}, err
	}
	var statusFilter string
	switch p.Stage {
	case 2, 3:
		var ferr error
		statusFilter, ferr = normalizeListStatusFilter(p.Stage, p.StatusQuery)
		if ferr != nil {
			return schema.JobListResponse{}, slots.ErrInvalidJobListQuery
		}
	case 1:
		statusFilter = ""
	default:
		return schema.JobListResponse{}, fmt.Errorf("invalid stage")
	}
	offset := (p.Page - 1) * p.Limit
	var entries []jobschema.JobListEntry
	var total int64
	switch p.Stage {
	case 1:
		entries, total, err = s.Jobs.ListSlotStage1Jobs(ctx, u, offset, p.Limit)
	case 2, 3:
		runID, ok, e := s.Runs.LatestPipelineRunIDForSlot(ctx, u)
		if e != nil {
			return schema.JobListResponse{}, e
		}
		if !ok {
			return schema.JobListResponse{Items: []schema.JobListItem{}, Page: p.Page, Limit: p.Limit, Total: 0}, nil
		}
		if p.Stage == 2 {
			entries, total, err = s.Jobs.ListPipelineRunStage2Jobs(ctx, u, runID, statusFilter, offset, p.Limit)
		} else {
			entries, total, err = s.Jobs.ListPipelineRunStage3Jobs(ctx, u, runID, statusFilter, offset, p.Limit)
		}
	default:
		return schema.JobListResponse{}, fmt.Errorf("invalid stage")
	}
	if err != nil {
		return schema.JobListResponse{}, err
	}
	includePRStatus := p.Stage == 2 || p.Stage == 3
	items := make([]schema.JobListItem, 0, len(entries))
	for _, e := range entries {
		item := slotschema.JobListItemFromEntry(e, includePRStatus, p.Stage2Debug)
		if p.Stage != 3 {
			item.Stage3Rationale = nil
		}
		items = append(items, item)
	}
	return schema.JobListResponse{
		Items: items,
		Page:  p.Page,
		Limit: p.Limit,
		Total: int(total),
	}, nil
}

// PatchJobBucket implements [slots.API.PatchJobBucket].
func (s *Service) PatchJobBucket(ctx context.Context, p slotschema.PatchJobBucketParams) (*schema.PatchJobBucketResponse, error) {
	log := s.methodLog(ctx, "PatchJobBucket")
	log.Debug().Msg("patch job bucket")
	if s.Runs == nil {
		return nil, errors.New("slots service: pipeline runs repository is required for bucket patch")
	}
	u, err := uuid.Parse(strings.TrimSpace(p.SlotID))
	if err != nil {
		return nil, slots.ErrNotFound
	}
	jid := strings.TrimSpace(p.JobID)
	if jid == "" {
		return nil, slots.ErrNotFound
	}
	if _, err := s.Repo.GetByID(ctx, u.String()); err != nil {
		return nil, err
	}
	runID, ok, err := s.Runs.LatestPipelineRunIDForSlot(ctx, u)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, slots.ErrNotFound
	}
	passed := p.Bucket == schema.JobBucketPassed
	switch p.Stage {
	case 2:
		err = s.Runs.ManualPatchStage2Bucket(ctx, runID, jid, passed)
	case 3:
		err = s.Runs.ManualPatchStage3Bucket(ctx, runID, jid, passed)
	default:
		return nil, fmt.Errorf("invalid stage for patch")
	}
	if errors.Is(err, pipeline.ErrManualPatchNotInScope) {
		return nil, slots.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &schema.PatchJobBucketResponse{JobID: jid, Bucket: p.Bucket}, nil
}
