package storage

import (
	"context"
	"errors"
	"sort"
	"strconv"
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

func TestRepository_VerdictsByHashes(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	insertJob(t, db, "job-a")
	insertJob(t, db, "job-b")
	insertJob(t, db, "job-stale")
	insertJob(t, db, "job-dual")
	insertJob(t, db, "job-other-model")
	insertJob(t, db, "job-other-pv")

	factsA := sampleFacts()
	factsA.Office.Countries = []string{"IN"}
	factsB := sampleFacts()
	factsB.Office.Countries = []string{"US"}
	factsCurrent := sampleFacts()
	factsCurrent.Office.Countries = []string{"DE"}

	for _, v := range []schema.Verdict{
		{JobID: "job-a", PromptVersion: schema.PromptVersion, Model: "haiku", InputHash: "hash-a", Facts: factsA},
		{JobID: "job-b", PromptVersion: schema.PromptVersion, Model: "haiku", InputHash: "hash-b", Facts: factsB},
		{JobID: "job-stale", PromptVersion: schema.PromptVersion, Model: "haiku", InputHash: "hash-old", Facts: sampleFacts()},
		{JobID: "job-dual", PromptVersion: schema.PromptVersion, Model: "haiku", InputHash: "hash-old", Facts: sampleFacts()},
		{JobID: "job-dual", PromptVersion: schema.PromptVersion, Model: "haiku", InputHash: "hash-current", Facts: factsCurrent},
		{JobID: "job-other-model", PromptVersion: schema.PromptVersion, Model: "other", InputHash: "hash-om", Facts: sampleFacts()},
		{JobID: "job-other-pv", PromptVersion: schema.PromptVersion + 1, Model: "haiku", InputHash: "hash-opv", Facts: sampleFacts()},
	} {
		if err := repo.Insert(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.VerdictsByHashes(ctx, "haiku", schema.PromptVersion, map[string]string{
		"job-a":           "hash-a",
		"job-b":           "hash-b",
		"job-stale":       "hash-new",
		"job-dual":        "hash-current",
		"job-other-model": "hash-om",
		"job-other-pv":    "hash-opv",
		"job-missing":     "hash-missing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3 (a, b, dual)", len(got))
	}
	if got["job-a"].Facts.Office.Countries[0] != "IN" {
		t.Fatalf("job-a facts = %#v", got["job-a"].Facts.Office)
	}
	if got["job-b"].Facts.Office.Countries[0] != "US" {
		t.Fatalf("job-b facts = %#v", got["job-b"].Facts.Office)
	}
	if got["job-dual"].InputHash != "hash-current" || got["job-dual"].Facts.Office.Countries[0] != "DE" {
		t.Fatalf("job-dual = %#v, want current hash", got["job-dual"])
	}
	for _, id := range []string{"job-stale", "job-other-model", "job-other-pv", "job-missing"} {
		if _, ok := got[id]; ok {
			t.Fatalf("%s must be omitted (pending)", id)
		}
	}
}

func TestRepository_VerdictsByHashes_empty(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	got, err := repo.VerdictsByHashes(ctx, "haiku", schema.PromptVersion, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %#v, want empty map", got)
	}

	got, err = repo.VerdictsByHashes(ctx, "haiku", schema.PromptVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("nil hashes: got = %#v, want empty map", got)
	}
}

func TestRepository_VerdictsByHashes_chunks(t *testing.T) {
	ctx := context.Background()
	db := testEnrichDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	const n = 1200
	hashes := make(map[string]string, n)
	for i := 0; i < n; i++ {
		id := "job-" + strconv.Itoa(i)
		hash := "hash-" + strconv.Itoa(i)
		insertJob(t, db, id)
		hashes[id] = hash
		if err := repo.Insert(ctx, schema.Verdict{
			JobID:         id,
			PromptVersion: schema.PromptVersion,
			Model:         "haiku",
			InputHash:     hash,
			Facts:         sampleFacts(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.VerdictsByHashes(ctx, "haiku", schema.PromptVersion, hashes)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("len(got) = %d, want %d (chunking must return all matches)", len(got), n)
	}
}
