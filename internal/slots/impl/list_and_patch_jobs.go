package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	jobschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	"github.com/google/uuid"
)

// normalizeListStatusFilter maps GET ?status= to a pipeline_run_jobs.status filter. Empty means no filter (all rows for that stage list).
func normalizeListStatusFilter(stage int, raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	st := pipeline.RunJobStatus(s)
	if !st.Valid() {
		return "", fmt.Errorf("invalid status")
	}
	switch stage {
	case 2:
		if st != pipeline.RunJobPassedStage2 && st != pipeline.RunJobRejectedStage2 {
			return "", fmt.Errorf("status not valid for stage 2 list")
		}
	case 3:
		if st != pipeline.RunJobPassedStage3 && st != pipeline.RunJobRejectedStage3 {
			return "", fmt.Errorf("status not valid for stage 3 list")
		}
	default:
		return "", fmt.Errorf("invalid stage")
	}
	return s, nil
}

func jobListItemFromEntry(e jobschema.JobListEntry, includePipelineStatus bool) schema.JobListItem {
	item := schema.JobListItem{
		JobID:           e.Job.ID,
		Title:           e.Job.Title,
		Company:         e.Job.Company,
		SourceID:        e.Job.Source,
		ApplyURL:        e.Job.ApplyURL,
		FirstSeenAt:     e.FirstSeenAt.UTC(),
		Stage3Rationale: e.Stage3Rationale,
	}
	if includePipelineStatus && e.PipelineRunStatus != "" {
		st := e.PipelineRunStatus
		item.Status = &st
	}
	if !e.Job.PostedAt.IsZero() {
		t := e.Job.PostedAt.UTC()
		item.PostedAt = &t
	}
	return item
}

// ListJobs implements [slots.API.ListJobs].
func (s *Service) ListJobs(ctx context.Context, slotID string, stage, page, limit int, statusQuery string) (schema.JobListResponse, error) {
	log := s.methodLog(ctx, "ListJobs")
	log.Debug().Msg("list jobs")
	if s.Jobs == nil {
		return schema.JobListResponse{}, errors.New("slots service: jobs repository is required for job lists")
	}
	u, err := uuid.Parse(strings.TrimSpace(slotID))
	if err != nil {
		return schema.JobListResponse{}, slots.ErrNotFound
	}
	if _, err := s.Repo.GetByID(ctx, u.String()); err != nil {
		return schema.JobListResponse{}, err
	}
	var statusFilter string
	switch stage {
	case 2, 3:
		var ferr error
		statusFilter, ferr = normalizeListStatusFilter(stage, statusQuery)
		if ferr != nil {
			return schema.JobListResponse{}, slots.ErrInvalidJobListQuery
		}
	case 1:
		statusFilter = ""
	default:
		return schema.JobListResponse{}, fmt.Errorf("invalid stage")
	}
	offset := (page - 1) * limit
	var entries []jobschema.JobListEntry
	var total int64
	switch stage {
	case 1:
		entries, total, err = s.Jobs.ListSlotStage1Jobs(ctx, u, offset, limit)
	case 2, 3:
		runID, ok, e := s.Runs.LatestPipelineRunIDForSlot(ctx, u)
		if e != nil {
			return schema.JobListResponse{}, e
		}
		if !ok {
			return schema.JobListResponse{Items: []schema.JobListItem{}, Page: page, Limit: limit, Total: 0}, nil
		}
		if stage == 2 {
			entries, total, err = s.Jobs.ListPipelineRunStage2Jobs(ctx, u, runID, statusFilter, offset, limit)
		} else {
			entries, total, err = s.Jobs.ListPipelineRunStage3Jobs(ctx, u, runID, statusFilter, offset, limit)
		}
	default:
		return schema.JobListResponse{}, fmt.Errorf("invalid stage")
	}
	if err != nil {
		return schema.JobListResponse{}, err
	}
	includePRStatus := stage == 2 || stage == 3
	items := make([]schema.JobListItem, 0, len(entries))
	for _, e := range entries {
		item := jobListItemFromEntry(e, includePRStatus)
		if stage != 3 {
			item.Stage3Rationale = nil
		}
		items = append(items, item)
	}
	return schema.JobListResponse{
		Items: items,
		Page:  page,
		Limit: limit,
		Total: int(total),
	}, nil
}

// PatchJobBucket implements [slots.API.PatchJobBucket].
func (s *Service) PatchJobBucket(ctx context.Context, slotID string, stage int, jobID string, bucket schema.JobBucket) (*schema.PatchJobBucketResponse, error) {
	log := s.methodLog(ctx, "PatchJobBucket")
	log.Debug().Msg("patch job bucket")
	if s.Runs == nil {
		return nil, errors.New("slots service: pipeline runs repository is required for bucket patch")
	}
	u, err := uuid.Parse(strings.TrimSpace(slotID))
	if err != nil {
		return nil, slots.ErrNotFound
	}
	jid := strings.TrimSpace(jobID)
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
	passed := bucket == schema.JobBucketPassed
	switch stage {
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
	return &schema.PatchJobBucketResponse{JobID: jid, Bucket: bucket}, nil
}
