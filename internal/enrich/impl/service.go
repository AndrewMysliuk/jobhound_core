package impl

import (
	"context"
	"errors"
	"fmt"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
)

// Service is the enrich use-case surface. It does not import Temporal.
type Service struct {
	repo                *storage.Repository
	jobs                jobWindow
	client              *BatchesClient
	model               string
	minDescriptionChars int
	windowDays          int
	now                 func() time.Time
}

type jobWindow interface {
	GetByID(ctx context.Context, id string) (jobdata.Job, error)
	ListEnrichCandidates(ctx context.Context, since, postedAfter time.Time) ([]jobdata.Job, error)
}

// New wires enrich storage, the jobs window, and the Message Batches client.
// model is the string stored on each verdict row.
// minDescriptionChars skips shorter descriptions in MissingIDs and Statuses;
// 0 disables the length filter (tests / explicit opt-out).
// windowDays is the first_seen_at lookback; empty/0 at config load uses schema.WindowDays,
// and a non-positive value here also falls back to that default.
func New(repo *storage.Repository, jobsRepo jobWindow, client *BatchesClient, model string, minDescriptionChars, windowDays int) *Service {
	return &Service{
		repo:                repo,
		jobs:                jobsRepo,
		client:              client,
		model:               model,
		minDescriptionChars: minDescriptionChars,
		windowDays:          windowDays,
		now:                 func() time.Time { return time.Now().UTC() },
	}
}

// Model is the configured model id stored on each verdict row.
func (s *Service) Model() string {
	if s == nil {
		return ""
	}
	return s.model
}

var (
	_ enrich.API = (*Service)(nil)
	_ jobWindow  = jobs.JobRepository(nil)
)

// MissingIDs returns open jobs in the enrich window with no row for this prompt
// version and model whose current text hash is also missing. Descriptions
// shorter than minDescriptionChars are omitted (they will never get a verdict).
func (s *Service) MissingIDs(ctx context.Context, model string, promptVersion int) ([]string, error) {
	if s == nil || s.repo == nil || s.jobs == nil {
		return nil, errors.New("enrich: repository and jobs are required")
	}
	if model == "" {
		return nil, fmt.Errorf("enrich: model is required")
	}
	all, err := s.jobs.ListEnrichCandidates(ctx, s.enrichSince(), s.postedAfter())
	if err != nil {
		return nil, err
	}
	eligible := filterByMinDescription(all, s.minDescriptionChars)
	hashes, err := currentHashes(eligible)
	if err != nil {
		return nil, err
	}
	return s.repo.MissingIDs(ctx, model, promptVersion, hashes)
}

// SubmitMissing builds batches for the given ids (pages of schema.MaxJobsPerSubmit),
// waits until each ends, and stores succeeded replies.
// Ids that already have a row for the current hash are skipped. Transport errors return without a row.
func (s *Service) SubmitMissing(ctx context.Context, jobIDs []string) error {
	if s == nil || s.client == nil {
		return errors.New("enrich: batches client is required")
	}
	for offset := 0; offset < len(jobIDs); offset += schema.MaxJobsPerSubmit {
		end := offset + schema.MaxJobsPerSubmit
		if end > len(jobIDs) {
			end = len(jobIDs)
		}
		submitted, err := s.SubmitBatch(ctx, jobIDs[offset:end], s.model, schema.PromptVersion)
		if err != nil {
			return err
		}
		if submitted.BatchID == "" {
			continue
		}
		if err := s.client.WaitEnded(ctx, submitted.BatchID, time.Second); err != nil {
			return err
		}
		texts, err := s.client.Results(ctx, submitted.BatchID, submitted.CustomToJob)
		if err != nil {
			return err
		}
		if _, err := s.StoreVerdicts(ctx, schema.StoreVerdictsInput{
			Texts:         texts,
			PromptVersion: schema.PromptVersion,
			Model:         s.model,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Verdict loads the row for the exact key, or enrich.ErrNotFound.
func (s *Service) Verdict(ctx context.Context, jobID, model string, promptVersion int, inputHash string) (schema.Verdict, error) {
	if s == nil || s.repo == nil {
		return schema.Verdict{}, errors.New("enrich: repository is required")
	}
	v, err := s.repo.Verdict(ctx, jobID, model, promptVersion, inputHash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return schema.Verdict{}, enrich.ErrNotFound
		}
		return schema.Verdict{}, err
	}
	return v, nil
}

// Statuses returns pending, ready, or off for each job id.
// Ready means a row exists for the current hash (even if the job is closed or
// outside the enrich window). Off means the description is below
// minDescriptionChars, or the job is closed / outside the window / stale
// posted_at with no ready verdict — same signal as when enrich skips that
// vacancy in MissingIDs.
func (s *Service) Statuses(ctx context.Context, jobIDs []string) (map[string]schema.FactsStatus, error) {
	if s == nil || s.repo == nil || s.jobs == nil {
		return nil, errors.New("enrich: repository and jobs are required")
	}
	if s.model == "" {
		return nil, fmt.Errorf("enrich: model is required")
	}
	out := make(map[string]schema.FactsStatus, len(jobIDs))
	hashes := make(map[string]string, len(jobIDs))
	jobsByID := make(map[string]jobdata.Job, len(jobIDs))
	for _, id := range jobIDs {
		if id == "" {
			continue
		}
		job, err := s.jobs.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if !descriptionEligible(job.Description, s.minDescriptionChars) {
			out[id] = schema.FactsStatusOff
			continue
		}
		jobsByID[id] = job
		hashes[id] = InputHash(job.Title, job.Description, job.Location.Raw, job.SalaryRaw)
	}
	if len(hashes) == 0 {
		return out, nil
	}
	got, err := s.repo.Statuses(ctx, s.model, schema.PromptVersion, hashes)
	if err != nil {
		return nil, err
	}
	since := s.enrichSince()
	postedAfter := s.postedAfter()
	for id, st := range got {
		if st == schema.FactsStatusReady {
			out[id] = schema.FactsStatusReady
			continue
		}
		if !windowEligible(jobsByID[id], since, postedAfter) {
			out[id] = schema.FactsStatusOff
			continue
		}
		out[id] = st
	}
	return out, nil
}

// ReadyByJobs returns ready verdicts keyed by job id. Omitted ids are pending.
func (s *Service) ReadyByJobs(ctx context.Context, jobs []jobdata.Job) (map[string]schema.Verdict, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("enrich: repository is required")
	}
	if s.model == "" {
		return nil, fmt.Errorf("enrich: model is required")
	}
	hashes, err := currentHashes(jobs)
	if err != nil {
		return nil, err
	}
	return s.repo.VerdictsByHashes(ctx, s.model, schema.PromptVersion, hashes)
}

// SubmitBatch creates one Message Batch for ids that still lack a current-hash row.
// Callers should pass at most schema.MaxJobsPerSubmit ids (Temporal payload budget).
// An empty SubmittedIDs / BatchID means nothing was sent.
func (s *Service) SubmitBatch(ctx context.Context, jobIDs []string, model string, promptVersion int) (schema.SubmitBatchResult, error) {
	if s == nil || s.repo == nil || s.jobs == nil || s.client == nil {
		return schema.SubmitBatchResult{}, errors.New("enrich: repository, jobs, and batches client are required")
	}
	if model == "" {
		return schema.SubmitBatchResult{}, fmt.Errorf("enrich: model is required")
	}
	if len(jobIDs) > schema.MaxJobsPerSubmit {
		jobIDs = jobIDs[:schema.MaxJobsPerSubmit]
	}
	items, _, err := s.buildBatchItems(ctx, jobIDs, model, promptVersion)
	if err != nil {
		return schema.SubmitBatchResult{}, err
	}
	if len(items) == 0 {
		return schema.SubmitBatchResult{}, nil
	}
	first, rest, err := firstBatchChunk(items, s.client.model(), s.client.maxTokens(), maxBatchBodyBytes, schema.MaxJobsPerSubmit)
	if err != nil {
		return schema.SubmitBatchResult{}, err
	}
	if len(rest) > 0 {
		return schema.SubmitBatchResult{}, fmt.Errorf("enrich: %d jobs still exceed Message Batches size after capping to %d; shrink descriptions or MaxJobsPerSubmit", len(rest), schema.MaxJobsPerSubmit)
	}
	batchID, customToJob, err := s.client.Create(ctx, first)
	if err != nil {
		return schema.SubmitBatchResult{}, err
	}
	submitted := make([]string, len(first))
	for i, item := range first {
		submitted[i] = item.JobID
	}
	return schema.SubmitBatchResult{
		BatchID:      batchID,
		CustomToJob:  customToJob,
		SubmittedIDs: submitted,
	}, nil
}

// PollBatch checks processing_status once. When ended, it loads JSONL results.
func (s *Service) PollBatch(ctx context.Context, in schema.PollBatchInput) (schema.PollBatchResult, error) {
	if s == nil || s.client == nil {
		return schema.PollBatchResult{}, errors.New("enrich: batches client is required")
	}
	if in.BatchID == "" {
		return schema.PollBatchResult{}, fmt.Errorf("enrich: batch id is required")
	}
	status, err := s.client.Status(ctx, in.BatchID)
	if err != nil {
		return schema.PollBatchResult{}, err
	}
	if status != "ended" {
		return schema.PollBatchResult{Ended: false}, nil
	}
	texts, err := s.client.Results(ctx, in.BatchID, in.CustomToJob)
	if err != nil {
		return schema.PollBatchResult{}, err
	}
	return schema.PollBatchResult{Ended: true, Texts: texts}, nil
}

// StoreVerdicts decodes, coerces, and inserts each succeeded model text.
// A schema-invalid body returns an error so Temporal can retry, unless
// AcceptUnknownOnSchemaFail is set (retries exhausted). Successful inserts in
// the same call are kept; one bad item does not drop the rest.
func (s *Service) StoreVerdicts(ctx context.Context, in schema.StoreVerdictsInput) (schema.StoreVerdictsResult, error) {
	if s == nil || s.repo == nil || s.jobs == nil {
		return schema.StoreVerdictsResult{}, errors.New("enrich: repository and jobs are required")
	}
	if in.Model == "" {
		return schema.StoreVerdictsResult{}, fmt.Errorf("enrich: model is required")
	}
	var stored []string
	var firstErr error
	for jobID, text := range in.Texts {
		if jobID == "" || text == "" {
			continue
		}
		job, err := s.jobs.GetByID(ctx, jobID)
		if err != nil {
			return schema.StoreVerdictsResult{StoredIDs: stored}, err
		}
		matchText := VacancyMatchText(job.Title, job.Description, job.Location.Raw, job.SalaryRaw)
		hash := InputHash(job.Title, job.Description, job.Location.Raw, job.SalaryRaw)
		var facts schema.ModelReply
		reply, err := decodeModelReply([]byte(text))
		if err != nil {
			if !in.AcceptUnknownOnSchemaFail {
				if firstErr == nil {
					firstErr = fmt.Errorf("enrich store job %s: %w", jobID, err)
				}
				continue
			}
			facts = CoerceModelReply(unknownModelReply(), matchText)
		} else {
			facts = CoerceModelReply(reply, matchText)
		}
		v := schema.Verdict{
			JobID:         jobID,
			PromptVersion: in.PromptVersion,
			Model:         in.Model,
			InputHash:     hash,
			Facts:         facts,
			CreatedAt:     s.now(),
		}
		if err := s.repo.Insert(ctx, v); err != nil {
			return schema.StoreVerdictsResult{StoredIDs: stored}, err
		}
		stored = append(stored, jobID)
	}
	if firstErr != nil {
		return schema.StoreVerdictsResult{StoredIDs: stored}, firstErr
	}
	return schema.StoreVerdictsResult{StoredIDs: stored}, nil
}

func (s *Service) buildBatchItems(ctx context.Context, jobIDs []string, model string, promptVersion int) ([]VacancyBatchItem, []string, error) {
	items := make([]VacancyBatchItem, 0, len(jobIDs))
	submitted := make([]string, 0, len(jobIDs))
	seen := make(map[string]struct{}, len(jobIDs))
	for _, id := range jobIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		job, err := s.jobs.GetByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		hash := InputHash(job.Title, job.Description, job.Location.Raw, job.SalaryRaw)
		_, err = s.repo.Verdict(ctx, id, model, promptVersion, hash)
		if err == nil {
			continue
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, err
		}
		items = append(items, VacancyBatchItem{
			JobID:       id,
			Title:       job.Title,
			Description: job.Description,
			LocationRaw: job.Location.Raw,
			SalaryRaw:   job.SalaryRaw,
		})
		submitted = append(submitted, id)
	}
	return items, submitted, nil
}

func (s *Service) enrichSince() time.Time {
	days := s.windowDays
	if days <= 0 {
		days = schema.WindowDays
	}
	return s.now().Add(-time.Duration(days) * 24 * time.Hour)
}

func (s *Service) postedAfter() time.Time {
	return s.now().Add(-time.Duration(schema.PostedAtMaxAgeDays) * 24 * time.Hour)
}

func currentHashes(jobs []jobdata.Job) (map[string]string, error) {
	out := make(map[string]string, len(jobs))
	for _, job := range jobs {
		if job.ID == "" {
			continue
		}
		out[job.ID] = InputHash(job.Title, job.Description, job.Location.Raw, job.SalaryRaw)
	}
	return out, nil
}

// descriptionEligible is true when minChars <= 0 (filter off) or the description
// meets the minimum length.
func descriptionEligible(description string, minChars int) bool {
	if minChars <= 0 {
		return true
	}
	return len(description) >= minChars
}

// windowEligible is true for open jobs first seen at or after since whose
// known posted_at is at or after postedAfter (unknown posted_at stays eligible).
func windowEligible(job jobdata.Job, since, postedAfter time.Time) bool {
	if job.ClosedAt != nil {
		return false
	}
	if job.FirstSeenAt.UTC().Before(since) {
		return false
	}
	if !job.PostedAt.IsZero() && job.PostedAt.UTC().Before(postedAfter) {
		return false
	}
	return true
}

func filterByMinDescription(jobs []jobdata.Job, minChars int) []jobdata.Job {
	if minChars <= 0 {
		return jobs
	}
	out := make([]jobdata.Job, 0, len(jobs))
	for _, job := range jobs {
		if descriptionEligible(job.Description, minChars) {
			out = append(out, job)
		}
	}
	return out
}
