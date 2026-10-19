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

func (m memJobs) ListFirstSeenSince(_ context.Context, _ time.Time) ([]jobdata.Job, error) {
	out := make([]jobdata.Job, 0, len(m))
	for _, job := range m {
		out = append(out, job)
	}
	return out, nil
}

func testStoreService(t *testing.T, jobs memJobs) *Service {
	t.Helper()
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
	seen := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for id, job := range jobs {
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
	return New(repo, jobs, nil, "claude-test")
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
