package storage

import (
	"context"
	"database/sql"
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
		company_key_ref TEXT,
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
		closed_at TIMESTAMP,
		closed_by TEXT NOT NULL DEFAULT '',
		board_seen_at TIMESTAMP,
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
	if len(got.Openings) != 2 {
		t.Fatalf("openings = %#v", got.Openings)
	}
	if got.Openings[0] != (jobdata.Opening{Country: "berlin", URL: second.URL, ApplyURL: second.ApplyURL}) {
		t.Fatalf("first opening = %#v", got.Openings[0])
	}
	if got.Openings[1] != (jobdata.Opening{Country: "Berlin", URL: first.URL, ApplyURL: first.ApplyURL}) {
		t.Fatalf("second opening = %#v", got.Openings[1])
	}
	if got.URL != got.Openings[0].URL || got.ApplyURL != got.Openings[0].ApplyURL {
		t.Fatalf("parent urls: url %q apply %q", got.URL, got.ApplyURL)
	}
	if got.Location.Raw != "berlin" {
		t.Fatalf("location.raw = %q", got.Location.Raw)
	}
	if !got.FirstSeenAt.Equal(past) {
		t.Fatalf("first_seen_at = %s, want %s", got.FirstSeenAt, past)
	}
	if !got.LastSeenAt.After(past) {
		t.Fatalf("last_seen_at = %s, want after %s", got.LastSeenAt, past)
	}
}

func TestRepository_SaveIngest_firstInsertWritesOneOpening(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	job := jobdata.Job{
		Source:      "himalayas",
		Title:       "Engineer",
		Company:     "Acme Inc",
		URL:         "https://himalayas.example/jobs/1",
		ApplyURL:    "https://agg.example/apply/1",
		Description: "first description",
		Location: jobdata.Location{
			Type:      "remote",
			Timezone:  "UTC",
			Raw:       " United States ",
			Countries: []string{"US", " DE ", ""},
			Regions:   []string{"na", "NA"},
		},
		Openings: []jobdata.Opening{{
			Country:  "Ignore",
			URL:      "https://ignore.example",
			ApplyURL: "https://ignore.example/apply",
		}},
	}
	if err := domainutils.AssignStableID(&job); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "acme\x1eengineer" || strings.Count(got.ID, "\x1e") != 1 {
		t.Fatalf("id = %q", got.ID)
	}
	want := jobdata.Opening{
		Country:  "United States",
		URL:      job.URL,
		ApplyURL: job.ApplyURL,
	}
	if len(got.Openings) != 1 || got.Openings[0] != want {
		t.Fatalf("openings = %#v", got.Openings)
	}
	if got.URL != want.URL || got.ApplyURL != want.ApplyURL {
		t.Fatalf("parent urls: url %q apply %q", got.URL, got.ApplyURL)
	}
	if got.Location.Raw != "United States" || got.Location.Type != "remote" || got.Location.Timezone != "UTC" {
		t.Fatalf("location = %#v", got.Location)
	}
	if len(got.Location.Countries) != 2 || got.Location.Countries[0] != "DE" || got.Location.Countries[1] != "US" {
		t.Fatalf("countries = %#v", got.Location.Countries)
	}
	if len(got.Location.Regions) != 2 || got.Location.Regions[0] != "NA" || got.Location.Regions[1] != "na" {
		t.Fatalf("regions = %#v", got.Location.Regions)
	}
	if got.FirstSeenAt.IsZero() || !got.FirstSeenAt.Equal(got.LastSeenAt) {
		t.Fatalf("seen first %s last %s", got.FirstSeenAt, got.LastSeenAt)
	}
}

func TestRepository_SaveIngest_secondCountryAppends(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	firstPosted := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	secondPosted := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	first := jobdata.Job{
		Source:      "himalayas",
		Title:       "Engineer",
		Company:     "Acme",
		URL:         "https://greece.example/jobs/1",
		ApplyURL:    "https://greece.example/apply/1",
		Description: "keep me",
		PostedAt:    firstPosted,
		Location: jobdata.Location{
			Type:      "remote",
			Timezone:  "UTC",
			Raw:       "Greece",
			Countries: []string{"GR"},
			Regions:   []string{"EU"},
		},
	}
	if err := domainutils.AssignStableID(&first); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, first); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Source = "builtin"
	second.Sources = nil
	second.Openings = nil
	second.URL = "https://us.example/jobs/1"
	second.ApplyURL = "https://us.example/apply/1"
	second.Description = "newer"
	second.PostedAt = secondPosted
	second.Location = jobdata.Location{
		Type:      "office",
		Timezone:  "America/New_York",
		Raw:       "United States",
		Countries: []string{"US"},
		Regions:   []string{"NA"},
	}
	if err := domainutils.AssignStableID(&second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, second); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != first.Description || got.Title != first.Title || got.Company != first.Company {
		t.Fatalf("kept fields description %q title %q company %q", got.Description, got.Title, got.Company)
	}
	if !got.PostedAt.Equal(secondPosted) {
		t.Fatalf("posted_at = %s, want %s", got.PostedAt, secondPosted)
	}
	if got.Location.Type != "remote" || got.Location.Timezone != "UTC" {
		t.Fatalf("location scope = %#v", got.Location)
	}
	if got.Location.Raw != "Greece, United States" {
		t.Fatalf("location.raw = %q", got.Location.Raw)
	}
	if len(got.Location.Countries) != 2 || got.Location.Countries[0] != "GR" || got.Location.Countries[1] != "US" {
		t.Fatalf("countries = %#v", got.Location.Countries)
	}
	if len(got.Location.Regions) != 2 || got.Location.Regions[0] != "EU" || got.Location.Regions[1] != "NA" {
		t.Fatalf("regions = %#v", got.Location.Regions)
	}
	want := []jobdata.Opening{
		{Country: "Greece", URL: first.URL, ApplyURL: first.ApplyURL},
		{Country: "United States", URL: second.URL, ApplyURL: second.ApplyURL},
	}
	if len(got.Openings) != len(want) || got.Openings[0] != want[0] || got.Openings[1] != want[1] {
		t.Fatalf("openings = %#v", got.Openings)
	}
	if got.URL != want[0].URL || got.ApplyURL != want[0].ApplyURL {
		t.Fatalf("parent urls: url %q apply %q", got.URL, got.ApplyURL)
	}

	replay := second
	replay.PostedAt = time.Time{}
	replay.Description = "zero posted"
	if _, err := repo.SaveIngest(ctx, replay); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PostedAt.Equal(secondPosted) || got.Description != first.Description || len(got.Openings) != 2 {
		t.Fatalf("after zero posted_at: posted %s description %q openings %d", got.PostedAt, got.Description, len(got.Openings))
	}
}

func TestRepository_SaveIngest_sameOpeningDoesNotDuplicate(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	first := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://himalayas.example/jobs/1",
		ApplyURL: "https://agg.example/apply/1",
		Location: jobdata.Location{Raw: "Berlin", Countries: []string{"DE"}},
	}
	if err := domainutils.AssignStableID(&first); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, first); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Source = "builtin"
	second.Sources = nil
	second.Location.Raw = " berlin "
	second.Description = "ignored"
	if _, err := repo.SaveIngest(ctx, second); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := jobdata.Opening{Country: "Berlin", URL: first.URL, ApplyURL: first.ApplyURL}
	if len(got.Openings) != 1 || got.Openings[0] != want {
		t.Fatalf("openings = %#v", got.Openings)
	}
	if got.Location.Raw != "Berlin" {
		t.Fatalf("location.raw = %q", got.Location.Raw)
	}
	if len(got.Location.Countries) != 1 || got.Location.Countries[0] != "DE" {
		t.Fatalf("countries = %#v", got.Location.Countries)
	}
}

func TestRepository_SaveIngest_polandAndPLStaySeparate(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	poland := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://board.example/poland",
		ApplyURL: "https://board.example/poland/apply",
		Location: jobdata.Location{Raw: "Poland", Countries: []string{"PL"}},
	}
	if err := domainutils.AssignStableID(&poland); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, poland); err != nil {
		t.Fatal(err)
	}

	pl := poland
	pl.Source = "builtin"
	pl.Sources = nil
	pl.URL = "https://other.example/pl"
	pl.ApplyURL = ""
	pl.Location = jobdata.Location{Raw: "PL", Countries: []string{"PL", " PL "}}
	if _, err := repo.SaveIngest(ctx, pl); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, poland.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []jobdata.Opening{
		{Country: "PL", URL: pl.URL, ApplyURL: ""},
		{Country: "Poland", URL: poland.URL, ApplyURL: poland.ApplyURL},
	}
	if len(got.Openings) != len(want) || got.Openings[0] != want[0] || got.Openings[1] != want[1] {
		t.Fatalf("openings = %#v", got.Openings)
	}
	if got.URL != want[0].URL || got.ApplyURL != "" {
		t.Fatalf("parent urls: url %q apply %q", got.URL, got.ApplyURL)
	}
	if got.Location.Raw != "PL, Poland" {
		t.Fatalf("location.raw = %q", got.Location.Raw)
	}
	if len(got.Location.Countries) != 1 || got.Location.Countries[0] != "PL" {
		t.Fatalf("countries = %#v", got.Location.Countries)
	}

	var apply sql.NullString
	if err := db.Raw(`SELECT apply_url FROM jobs WHERE id = ?`, poland.ID).Scan(&apply).Error; err != nil {
		t.Fatal(err)
	}
	if apply.Valid {
		t.Fatalf("apply_url = %q, want NULL", apply.String)
	}
}

func TestRepository_SaveIngest_fillsEmptyCompanyWebsite(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	first := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme",
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

	portfolio := first
	portfolio.Source = "consider_andreessen_horowitz"
	portfolio.Sources = nil
	portfolio.URL = "https://jobs.a16z.com/jobs/1?gh_jid=1"
	portfolio.ApplyURL = "https://boards.greenhouse.io/acme/jobs/1"
	portfolio.CompanyWebsite = "https://acme.example"
	if _, err := repo.SaveIngest(ctx, portfolio); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompanyWebsite != portfolio.CompanyWebsite {
		t.Fatalf("company_website = %q, want %q", got.CompanyWebsite, portfolio.CompanyWebsite)
	}

	later := portfolio
	later.CompanyWebsite = "https://other.example"
	later.URL = "https://jobs.a16z.com/jobs/2"
	later.ApplyURL = "https://boards.greenhouse.io/acme/jobs/2"
	if _, err := repo.SaveIngest(ctx, later); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompanyWebsite != portfolio.CompanyWebsite {
		t.Fatalf("company_website = %q, want stored %q", got.CompanyWebsite, portfolio.CompanyWebsite)
	}
}

func TestRepository_ListPostedBefore(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	cutoff := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	save := func(title, url string, posted time.Time) string {
		t.Helper()
		job := jobdata.Job{
			Source:      "himalayas",
			Title:       title,
			Company:     "Acme",
			URL:         url,
			Description: "d",
			PostedAt:    posted,
		}
		if err := domainutils.AssignStableID(&job); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.SaveIngest(ctx, job); err != nil {
			t.Fatal(err)
		}
		return job.ID
	}

	oldID := save("Old", "https://example.test/old", cutoff.Add(-time.Second))
	save("Edge", "https://example.test/edge", cutoff)
	save("Fresh", "https://example.test/fresh", cutoff.Add(time.Second))
	save("Undated", "https://example.test/undated", time.Time{})

	got, err := repo.ListPostedBefore(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != oldID {
		ids := make([]string, len(got))
		for i, job := range got {
			ids[i] = job.ID
		}
		t.Fatalf("ids = %v, want [%s]", ids, oldID)
	}
}
