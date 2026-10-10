package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testJobsDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	stmt := `CREATE TABLE jobs (
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
		salary_raw TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '[]',
		position TEXT,
		first_seen_at TIMESTAMP NOT NULL,
		last_seen_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`
	if err := db.Exec(stmt).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRepository_SaveIngest_mergesSourcesKeepsFirstSeen(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	first := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme Inc",
		URL:      "https://himalayas.example/jobs/1",
		ApplyURL: "https://agg.example/apply/1",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&first); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, first); err != nil {
		t.Fatal(err)
	}
	past := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := db.Exec(
		`UPDATE jobs SET first_seen_at = ?, last_seen_at = ? WHERE id = ?`,
		past, past, first.ID,
	).Error; err != nil {
		t.Fatal(err)
	}

	second := jobdata.Job{
		Source:   "builtin",
		Title:    "  engineer ",
		Company:  "acme llc",
		URL:      "https://builtin.example/jobs/9",
		ApplyURL: "https://builtin.example/apply/9",
		Location: jobdata.Location{Raw: " berlin "},
	}
	if err := domainutils.AssignStableID(&second); err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("ids differ: %q vs %q", first.ID, second.ID)
	}
	if _, err := repo.SaveIngest(ctx, second); err != nil {
		t.Fatal(err)
	}

	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM jobs`).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	got, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 2 || got.Sources[0] != "himalayas" || got.Sources[1] != "builtin" {
		t.Fatalf("sources %#v", got.Sources)
	}
	if got.ApplyURL != first.ApplyURL || got.URL != first.URL {
		t.Fatalf("aggregator urls changed: url %q apply %q", got.URL, got.ApplyURL)
	}
	if !got.FirstSeenAt.Equal(past) {
		t.Fatalf("first_seen_at = %s, want %s", got.FirstSeenAt, past)
	}
	if !got.LastSeenAt.After(past) {
		t.Fatalf("last_seen_at = %s, want after %s", got.LastSeenAt, past)
	}
}

func TestRepository_SaveIngest_atsURLWins(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	agg := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://himalayas.example/jobs/1",
		ApplyURL: "https://agg.example/apply/1",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&agg); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, agg); err != nil {
		t.Fatal(err)
	}

	ats := agg
	ats.Source = "greenhouse"
	ats.Sources = nil
	ats.URL = "https://boards.greenhouse.io/acme/jobs/1"
	ats.ApplyURL = "https://boards.greenhouse.io/acme/jobs/1/apply"
	if _, err := repo.SaveIngest(ctx, ats); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, agg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != ats.URL || got.ApplyURL != ats.ApplyURL {
		t.Fatalf("ats urls: url %q apply %q", got.URL, got.ApplyURL)
	}

	later := agg
	later.Source = "builtin"
	later.URL = "https://builtin.example/jobs/9"
	later.ApplyURL = "https://builtin.example/apply/9"
	if _, err := repo.SaveIngest(ctx, later); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, agg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != ats.URL || got.ApplyURL != ats.ApplyURL {
		t.Fatalf("aggregator overwrote ats: url %q apply %q", got.URL, got.ApplyURL)
	}
	if len(got.Sources) != 3 {
		t.Fatalf("sources %#v", got.Sources)
	}
}
