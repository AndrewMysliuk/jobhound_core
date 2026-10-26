package impl

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const validUnknownReplyJSON = `{
  "office": {"countries": [], "regions": [], "quote": null, "unknown": true},
  "hiring_scope": {"value": "unknown", "countries": [], "regions": [], "quote": null},
  "work_authorization": {"value": "unknown", "countries": [], "quote": null},
  "timezone": {"value": "unknown", "zones": [], "quote": null},
  "compensation": {"currency": "", "min": null, "max": null, "period": "", "quote": null, "unknown": true},
  "work_mode": {"value": "unknown", "quote": null},
  "employment_load": {"value": "unknown", "quote": null},
  "engagement": {"value": "unknown", "quote": null}
}`

type memJobs map[string]jobdata.Job

func (m memJobs) GetByID(_ context.Context, id string) (jobdata.Job, error) {
	job, ok := m[id]
	if !ok {
		return jobdata.Job{}, errors.New("job not found")
	}
	return job, nil
}

func (m memJobs) ListEnrichCandidates(_ context.Context, since, postedAfter time.Time) ([]jobdata.Job, error) {
	out := make([]jobdata.Job, 0, len(m))
	for _, job := range m {
		if job.ClosedAt != nil {
			continue
		}
		if job.FirstSeenAt.UTC().Before(since) {
			continue
		}
		if !job.PostedAt.IsZero() && job.PostedAt.UTC().Before(postedAfter) {
			continue
		}
		out = append(out, job)
	}
	return out, nil
}

func recentSeen() time.Time {
	return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
}

func withRecentSeen(job jobdata.Job) jobdata.Job {
	if job.FirstSeenAt.IsZero() {
		job.FirstSeenAt = recentSeen()
	}
	return job
}

func testStoreService(t *testing.T, jobs memJobs) *Service {
	t.Helper()
	for id, job := range jobs {
		jobs[id] = withRecentSeen(job)
	}
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			sources TEXT NOT NULL DEFAULT '[]',
			title TEXT NOT NULL DEFAULT '',
			company TEXT NOT NULL DEFAULT '',
			company_key TEXT NOT NULL DEFAULT '',
			company_website TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			apply_url TEXT,
			description TEXT NOT NULL DEFAULT '',
			posted_at TIMESTAMP,
			location TEXT NOT NULL DEFAULT '{}',
			openings TEXT NOT NULL DEFAULT '[]',
			salary_raw TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			position TEXT,
			first_seen_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE job_verdicts (
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			prompt_version INTEGER NOT NULL,
			model TEXT NOT NULL,
			input_hash TEXT NOT NULL,
			facts TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			UNIQUE (job_id, prompt_version, model, input_hash)
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	for id, job := range jobs {
		seen := job.FirstSeenAt
		if err := db.Exec(`
			INSERT INTO jobs (
				id, sources, title, company, url, description, location, tags,
				first_seen_at, last_seen_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id,
			`["himalayas"]`,
			job.Title,
			"Acme",
			"https://example.test/"+id,
			job.Description,
			`{"type":"remote","raw":"`+job.Location.Raw+`"}`,
			`[]`,
			seen, seen, seen, seen,
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := storage.NewRepository(pgsql.NewGetter(db))
	svc := New(repo, jobs, nil, "claude-test", 0, schema.WindowDays)
	svc.now = func() time.Time { return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC) }
	return svc
}

func testServiceWithMinChars(t *testing.T, jobs memJobs, minChars int) *Service {
	t.Helper()
	svc := testStoreService(t, jobs)
	svc.minDescriptionChars = minChars
	return svc
}

func longDescription(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func TestMissingIDs_filtersShortDescription(t *testing.T) {
	t.Parallel()

	const (
		shortID = "job-short"
		longID  = "job-long"
		model   = "claude-test"
	)
	jobs := memJobs{
		shortID: {
			ID:          shortID,
			Title:       "Engineer",
			Description: "Go Python React",
			Location:    jobdata.Location{Raw: "Remote"},
		},
		longID: {
			ID:          longID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	ctx := context.Background()

	got, err := svc.MissingIDs(ctx, model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != longID {
		t.Fatalf("MissingIDs = %v, want only %q", got, longID)
	}
}

func TestMissingIDs_zeroMinDisablesFilter(t *testing.T) {
	t.Parallel()

	const shortID = "job-short"
	jobs := memJobs{
		shortID: {
			ID:          shortID,
			Title:       "Engineer",
			Description: "short",
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testServiceWithMinChars(t, jobs, 0)
	got, err := svc.MissingIDs(context.Background(), "claude-test", schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != shortID {
		t.Fatalf("MissingIDs = %v, want %q when min=0", got, shortID)
	}
}

func TestMissingIDs_skipsClosed(t *testing.T) {
	t.Parallel()

	const (
		openID   = "job-open"
		closedID = "job-closed"
		model    = "claude-test"
	)
	closedAt := recentSeen()
	jobs := memJobs{
		openID: {
			ID:          openID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
		},
		closedID: {
			ID:          closedID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			ClosedAt:    &closedAt,
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	got, err := svc.MissingIDs(context.Background(), model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != openID {
		t.Fatalf("MissingIDs = %v, want only open %q", got, openID)
	}
}

func TestMissingIDs_skipsOutsideWindow(t *testing.T) {
	t.Parallel()

	const (
		freshID = "job-fresh"
		oldID   = "job-old"
		model   = "claude-test"
	)
	jobs := memJobs{
		freshID: {
			ID:          freshID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			FirstSeenAt: recentSeen(),
		},
		oldID: {
			ID:          oldID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			FirstSeenAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	got, err := svc.MissingIDs(context.Background(), model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != freshID {
		t.Fatalf("MissingIDs = %v, want only fresh %q", got, freshID)
	}
}

func TestMissingIDs_zeroWindowUsesDefault(t *testing.T) {
	t.Parallel()

	const jobID = "job-in-default-window"
	jobs := memJobs{
		jobID: {
			ID:          jobID,
			Title:       "Engineer",
			Description: "short",
			Location:    jobdata.Location{Raw: "Remote"},
			FirstSeenAt: recentSeen(),
		},
	}
	svc := testStoreService(t, jobs)
	svc.windowDays = 0
	got, err := svc.MissingIDs(context.Background(), "claude-test", schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != jobID {
		t.Fatalf("MissingIDs = %v, want %q when windowDays=0 (default)", got, jobID)
	}
}

func TestMissingIDs_afterStoreVerdictsNewHashNotMissing(t *testing.T) {
	t.Parallel()

	const (
		jobID = "job-rehash"
		model = "claude-test"
	)
	jobs := memJobs{
		jobID: {
			ID:          jobID,
			Title:       "Engineer",
			Description: "Remote worldwide first text",
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testStoreService(t, jobs)
	ctx := context.Background()

	missing, err := svc.MissingIDs(ctx, model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != jobID {
		t.Fatalf("before store MissingIDs = %v, want %q", missing, jobID)
	}

	if _, err := svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts:         map[string]string{jobID: validUnknownReplyJSON},
		PromptVersion: schema.PromptVersion,
		Model:         model,
	}); err != nil {
		t.Fatal(err)
	}
	missing, err = svc.MissingIDs(ctx, model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("after first store MissingIDs = %v, want empty", missing)
	}

	updated := jobs[jobID]
	updated.Description = "Remote worldwide rewritten board text that changes the input hash"
	jobs[jobID] = updated

	missing, err = svc.MissingIDs(ctx, model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != jobID {
		t.Fatalf("after description change MissingIDs = %v, want %q", missing, jobID)
	}

	if _, err := svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts:         map[string]string{jobID: validUnknownReplyJSON},
		PromptVersion: schema.PromptVersion,
		Model:         model,
	}); err != nil {
		t.Fatal(err)
	}
	missing, err = svc.MissingIDs(ctx, model, schema.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("after store with new input_hash MissingIDs = %v, want empty", missing)
	}
}

func TestStatuses_shortDescriptionOff(t *testing.T) {
	t.Parallel()

	const (
		shortID = "job-short"
		longID  = "job-long"
	)
	jobs := memJobs{
		shortID: {
			ID:          shortID,
			Title:       "Engineer",
			Description: "labels only",
			Location:    jobdata.Location{Raw: "Remote"},
		},
		longID: {
			ID:          longID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	got, err := svc.Statuses(context.Background(), []string{shortID, longID})
	if err != nil {
		t.Fatal(err)
	}
	if got[shortID] != schema.FactsStatusOff {
		t.Fatalf("short status = %q, want off", got[shortID])
	}
	if got[longID] != schema.FactsStatusPending {
		t.Fatalf("long status = %q, want pending", got[longID])
	}
}

func TestStatuses_zeroMinDisablesFilter(t *testing.T) {
	t.Parallel()

	const shortID = "job-short"
	jobs := memJobs{
		shortID: {
			ID:          shortID,
			Title:       "Engineer",
			Description: "short",
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testServiceWithMinChars(t, jobs, 0)
	got, err := svc.Statuses(context.Background(), []string{shortID})
	if err != nil {
		t.Fatal(err)
	}
	if got[shortID] != schema.FactsStatusPending {
		t.Fatalf("status = %q, want pending when min=0", got[shortID])
	}
}

func TestStatuses_closedWithoutVerdictOff(t *testing.T) {
	t.Parallel()

	const closedID = "job-closed"
	closedAt := recentSeen()
	jobs := memJobs{
		closedID: {
			ID:          closedID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			ClosedAt:    &closedAt,
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	got, err := svc.Statuses(context.Background(), []string{closedID})
	if err != nil {
		t.Fatal(err)
	}
	if got[closedID] != schema.FactsStatusOff {
		t.Fatalf("status = %q, want off for closed without verdict", got[closedID])
	}
}

func TestStatuses_closedWithVerdictReady(t *testing.T) {
	t.Parallel()

	const (
		closedID = "job-closed-ready"
		model    = "claude-test"
	)
	closedAt := recentSeen()
	jobs := memJobs{
		closedID: {
			ID:          closedID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			ClosedAt:    &closedAt,
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	ctx := context.Background()
	if _, err := svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts:         map[string]string{closedID: validUnknownReplyJSON},
		PromptVersion: schema.PromptVersion,
		Model:         model,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Statuses(ctx, []string{closedID})
	if err != nil {
		t.Fatal(err)
	}
	if got[closedID] != schema.FactsStatusReady {
		t.Fatalf("status = %q, want ready for closed with current-hash verdict", got[closedID])
	}
}

func TestStatuses_stalePostedAt(t *testing.T) {
	t.Parallel()

	const (
		staleID = "job-stale-posted"
		freshID = "job-fresh-posted"
		readyID = "job-stale-ready"
		model   = "claude-test"
	)
	// svc.now is 2026-10-25; PostedAtMaxAgeDays=30 → postedAfter 2026-09-25.
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	stalePosted := now.Add(-60 * 24 * time.Hour)
	freshPosted := now.Add(-10 * 24 * time.Hour)

	jobs := memJobs{
		staleID: {
			ID:          staleID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			PostedAt:    stalePosted,
		},
		freshID: {
			ID:          freshID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			PostedAt:    freshPosted,
		},
		readyID: {
			ID:          readyID,
			Title:       "Engineer",
			Description: longDescription(schema.MinDescriptionChars),
			Location:    jobdata.Location{Raw: "Remote"},
			PostedAt:    stalePosted,
		},
	}
	svc := testServiceWithMinChars(t, jobs, schema.MinDescriptionChars)
	ctx := context.Background()
	if _, err := svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts:         map[string]string{readyID: validUnknownReplyJSON},
		PromptVersion: schema.PromptVersion,
		Model:         model,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Statuses(ctx, []string{staleID, freshID, readyID})
	if err != nil {
		t.Fatal(err)
	}
	if got[staleID] != schema.FactsStatusOff {
		t.Fatalf("stale without verdict = %q, want off", got[staleID])
	}
	if got[freshID] != schema.FactsStatusPending {
		t.Fatalf("fresh without verdict = %q, want pending", got[freshID])
	}
	if got[readyID] != schema.FactsStatusReady {
		t.Fatalf("stale with ready verdict = %q, want ready", got[readyID])
	}
}

func TestStoreVerdictsSchemaInvalidRetriesThenUnknown(t *testing.T) {
	t.Parallel()

	const (
		goodID = "job-good"
		badID  = "job-bad"
		model  = "claude-test"
	)
	jobs := memJobs{
		goodID: {
			ID:          goodID,
			Title:       "Engineer",
			Description: "Remote worldwide",
			Location:    jobdata.Location{Raw: "Remote"},
		},
		badID: {
			ID:          badID,
			Title:       "Engineer",
			Description: "Broken reply fixture",
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testStoreService(t, jobs)
	ctx := context.Background()

	invalid := `{"office":{"countries":[],"regions":[],"quote":null,"unknown":true}}`
	out, err := svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts: map[string]string{
			goodID: validUnknownReplyJSON,
			badID:  invalid,
		},
		PromptVersion: schema.PromptVersion,
		Model:         model,
	})
	if err == nil {
		t.Fatal("expected schema error on first failure")
	}
	if !strings.Contains(err.Error(), "model reply schema") {
		t.Fatalf("error = %v, want schema validation failure", err)
	}
	sort.Strings(out.StoredIDs)
	if len(out.StoredIDs) != 1 || out.StoredIDs[0] != goodID {
		t.Fatalf("StoredIDs = %v, want only %q (bad item must not drop the rest)", out.StoredIDs, goodID)
	}
	goodHash := InputHash(jobs[goodID].Title, jobs[goodID].Description, jobs[goodID].Location.Raw, "")
	if _, err := svc.repo.Verdict(ctx, goodID, model, schema.PromptVersion, goodHash); err != nil {
		t.Fatalf("good job row missing: %v", err)
	}
	badHash := InputHash(jobs[badID].Title, jobs[badID].Description, jobs[badID].Location.Raw, "")
	if _, err := svc.repo.Verdict(ctx, badID, model, schema.PromptVersion, badHash); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("bad job must stay pending on first failure, err=%v", err)
	}

	out, err = svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts: map[string]string{
			goodID: validUnknownReplyJSON,
			badID:  invalid,
		},
		PromptVersion:             schema.PromptVersion,
		Model:                     model,
		AcceptUnknownOnSchemaFail: true,
	})
	if err != nil {
		t.Fatalf("StoreVerdicts after retries: %v", err)
	}
	sort.Strings(out.StoredIDs)
	if len(out.StoredIDs) != 2 {
		t.Fatalf("StoredIDs = %v, want both jobs", out.StoredIDs)
	}
	got, err := svc.repo.Verdict(ctx, badID, model, schema.PromptVersion, badHash)
	if err != nil {
		t.Fatalf("bad job row after exhausted retries: %v", err)
	}
	if !got.Facts.Office.Unknown || got.Facts.HiringScope.Value != unknownAnswer {
		t.Fatalf("schema-invalid after retries must be all-unknown, got %#v", got.Facts)
	}
}

func TestReadyByJobs(t *testing.T) {
	t.Parallel()

	const (
		readyID = "job-ready"
		staleID = "job-stale"
		model   = "claude-test"
	)
	jobs := memJobs{
		readyID: {
			ID:          readyID,
			Title:       "Ready Eng",
			Description: "Remote worldwide ready",
			Location:    jobdata.Location{Raw: "Remote"},
		},
		staleID: {
			ID:          staleID,
			Title:       "Stale Eng",
			Description: "Remote worldwide stale",
			Location:    jobdata.Location{Raw: "Remote"},
		},
	}
	svc := testStoreService(t, jobs)
	ctx := context.Background()

	if _, err := svc.StoreVerdicts(ctx, schema.StoreVerdictsInput{
		Texts:         map[string]string{readyID: validUnknownReplyJSON},
		PromptVersion: schema.PromptVersion,
		Model:         model,
	}); err != nil {
		t.Fatal(err)
	}
	staleHash := InputHash("old-title", jobs[staleID].Description, jobs[staleID].Location.Raw, "")
	if err := svc.repo.Insert(ctx, schema.Verdict{
		JobID:         staleID,
		PromptVersion: schema.PromptVersion,
		Model:         model,
		InputHash:     staleHash,
		Facts:         unknownModelReply(),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.ReadyByJobs(ctx, []jobdata.Job{
		jobs[readyID],
		jobs[staleID],
		{ID: "", Title: "skip"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[readyID].JobID != readyID {
		t.Fatalf("ready missing: %#v", got)
	}
	if _, ok := got[staleID]; ok {
		t.Fatal("stale hash must be omitted (pending)")
	}
}
