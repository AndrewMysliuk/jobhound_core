package storage

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testEnrichDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	stmts := []string{
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
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func insertJob(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	seen := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	err := db.Exec(`
		INSERT INTO jobs (
			id, sources, title, company, url, location, tags,
			first_seen_at, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		`["himalayas"]`,
		"Engineer",
		"Acme",
		"https://example.test/"+id,
		`{"type":"remote","raw":"Remote"}`,
		`[]`,
		seen, seen, seen, seen,
	).Error
	if err != nil {
		t.Fatal(err)
	}
}

func sampleFacts() schema.ModelReply {
	return schema.ModelReply{
		Office: schema.Office{
			Countries: []string{"IN"},
			Regions:   []string{},
			Unknown:   false,
		},
		HiringScope:       schema.HiringScope{Value: "unknown", Countries: []string{}, Regions: []string{}},
		WorkAuthorization: schema.WorkAuthorization{Value: "unknown", Countries: []string{}},
		Timezone:          schema.TimezoneConstraint{Value: "unknown", Zones: []string{}},
		Compensation:      schema.Compensation{Currency: "INR", Period: "year", Unknown: false},
		WorkMode:          schema.QuotedValue{Value: "remote"},
		EmploymentLoad:    schema.QuotedValue{Value: "unknown"},
		Engagement:        schema.QuotedValue{Value: "unknown"},
	}
}

func TestRepository_Insert_uniqueKeyIsNoOp(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	insertJob(t, db, "job-1")

	first := schema.Verdict{
		JobID:         "job-1",
		PromptVersion: schema.PromptVersion,
		Model:         "haiku",
		InputHash:     "hash-a",
		Facts:         sampleFacts(),
		CreatedAt:     time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := repo.Insert(ctx, first); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Facts.Office.Countries = []string{"US"}
	second.CreatedAt = first.CreatedAt.Add(time.Hour)
	if err := repo.Insert(ctx, second); err != nil {
		t.Fatal(err)
	}

	var n int64
	if err := db.Model(&JobVerdict{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}

	got, err := repo.Verdict(ctx, "job-1", "haiku", schema.PromptVersion, "hash-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Facts.Office.Countries) != 1 || got.Facts.Office.Countries[0] != "IN" {
		t.Fatalf("facts overwritten on conflict: %#v", got.Facts.Office)
	}
	if !got.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("created_at = %v, want %v", got.CreatedAt, first.CreatedAt)
	}
}

func TestRepository_Verdict_notFound(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	_, err := repo.Verdict(ctx, "missing", "haiku", schema.PromptVersion, "hash-a")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestRepository_MissingIDs_skipsCurrentHash(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	insertJob(t, db, "job-ready")
	insertJob(t, db, "job-pending")
	insertJob(t, db, "job-stale")

	if err := repo.Insert(ctx, schema.Verdict{
		JobID:         "job-ready",
		PromptVersion: schema.PromptVersion,
		Model:         "haiku",
		InputHash:     "hash-ready",
		Facts:         sampleFacts(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Insert(ctx, schema.Verdict{
		JobID:         "job-stale",
		PromptVersion: schema.PromptVersion,
		Model:         "haiku",
		InputHash:     "hash-old",
		Facts:         sampleFacts(),
	}); err != nil {
		t.Fatal(err)
	}

	missing, err := repo.MissingIDs(ctx, "haiku", schema.PromptVersion, map[string]string{
		"job-ready":   "hash-ready",
		"job-pending": "hash-pending",
		"job-stale":   "hash-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(missing)
	want := []string{"job-pending", "job-stale"}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Fatalf("missing = %v, want %v", missing, want)
		}
	}
}

func TestRepository_Statuses_pendingAndReady(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	insertJob(t, db, "job-ready")
	insertJob(t, db, "job-pending")

	if err := repo.Insert(ctx, schema.Verdict{
		JobID:         "job-ready",
		PromptVersion: schema.PromptVersion,
		Model:         "haiku",
		InputHash:     "hash-ready",
		Facts:         sampleFacts(),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Statuses(ctx, "haiku", schema.PromptVersion, map[string]string{
		"job-ready":   "hash-ready",
		"job-pending": "hash-pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["job-ready"] != schema.FactsStatusReady {
		t.Fatalf("job-ready = %s, want ready", got["job-ready"])
	}
	if got["job-pending"] != schema.FactsStatusPending {
		t.Fatalf("job-pending = %s, want pending", got["job-pending"])
	}
}
