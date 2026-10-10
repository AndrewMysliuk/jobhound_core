package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testScoringDB(t *testing.T) *gorm.DB {
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
			salary_raw TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			position TEXT,
			first_seen_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE profile_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMP NOT NULL,
			finished_at TIMESTAMP,
			jobs_scored INTEGER NOT NULL DEFAULT 0,
			sources_skipped INTEGER NOT NULL DEFAULT 0,
			idempotency_key TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE profile_matches (
			profile_id TEXT NOT NULL,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			bucket TEXT NOT NULL CHECK (bucket IN ('PASSED', 'REJECTED')),
			score INTEGER NOT NULL,
			signals TEXT NOT NULL,
			user_status TEXT NOT NULL DEFAULT 'NEW' CHECK (user_status IN ('NEW', 'HIDDEN')),
			run_id INTEGER NOT NULL REFERENCES profile_runs(id),
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (profile_id, job_id)
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func insertJob(t *testing.T, db *gorm.DB, id, title string, firstSeen time.Time) {
	t.Helper()
	err := db.Exec(`
		INSERT INTO jobs (
			id, sources, title, company, url, apply_url, location, tags,
			first_seen_at, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		`["himalayas"]`,
		title,
		"Acme",
		"https://example.test/"+id,
		"https://example.test/"+id+"/apply",
		`{"type":"remote","regions":["europe"],"countries":["DE"],"timezone":"","raw":"Berlin"}`,
		`[]`,
		firstSeen, firstSeen, firstSeen, firstSeen,
	).Error
	if err != nil {
		t.Fatal(err)
	}
}

func TestRepository_UpsertMatch_overwritesScoreKeepsHidden(t *testing.T) {
	ctx := context.Background()
	db := testScoringDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	seen := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	insertJob(t, db, "job-1", "Engineer", seen)

	firstRun, err := repo.CreateRun(ctx, schema.Run{
		ProfileID:      "andrew",
		Status:         schema.RunStatusRunning,
		StartedAt:      seen,
		IdempotencyKey: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	secondRun, err := repo.CreateRun(ctx, schema.Run{
		ProfileID:      "andrew",
		Status:         schema.RunStatusSucceeded,
		StartedAt:      seen.Add(time.Hour),
		JobsScored:     1,
		IdempotencyKey: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.UpsertMatch(ctx, schema.Match{
		ProfileID:  "andrew",
		JobID:      "job-1",
		Bucket:     schema.BucketPassed,
		Score:      4,
		Signals:    []schema.Signal{{Code: schema.SignalQuery, Points: 1, Terms: []string{"vue"}}},
		UserStatus: schema.UserStatusHidden,
		RunID:      firstRun.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.UpsertMatch(ctx, schema.Match{
		ProfileID:  "andrew",
		JobID:      "job-1",
		Bucket:     schema.BucketRejected,
		Score:      0,
		Signals:    []schema.Signal{{Code: schema.SignalCutExcludeTitle, Points: 0, Terms: []string{"recruiter"}}},
		UserStatus: schema.UserStatusNew,
		RunID:      secondRun.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	var row ProfileMatch
	if err := db.Where("profile_id = ? AND job_id = ?", "andrew", "job-1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Score != 0 {
		t.Fatalf("score = %d, want 0", row.Score)
	}
	if row.Bucket != string(schema.BucketRejected) {
		t.Fatalf("bucket = %s, want REJECTED", row.Bucket)
	}
	if row.UserStatus != string(schema.UserStatusHidden) {
		t.Fatalf("user_status = %s, want HIDDEN", row.UserStatus)
	}
	if row.RunID != secondRun.ID {
		t.Fatalf("run_id = %d, want %d", row.RunID, secondRun.ID)
	}
	signals, err := decodeSignals(row.Signals)
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 || signals[0].Code != schema.SignalCutExcludeTitle || len(signals[0].Terms) != 1 || signals[0].Terms[0] != "recruiter" {
		t.Fatalf("signals = %#v", signals)
	}
}

func TestRepository_CreateRun_uniqueIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	db := testScoringDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	key := uuid.New()
	started := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	got, err := repo.CreateRun(ctx, schema.Run{
		ProfileID:      "andrew",
		Status:         schema.RunStatusRunning,
		StartedAt:      started,
		JobsScored:     3,
		SourcesSkipped: 1,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == 0 {
		t.Fatal("id = 0")
	}
	var stored ProfileRun
	if err := db.First(&stored, got.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(schema.RunStatusRunning) {
		t.Fatalf("status = %s", stored.Status)
	}
	if stored.JobsScored != 3 || stored.SourcesSkipped != 1 {
		t.Fatalf("counters scored=%d skipped=%d", stored.JobsScored, stored.SourcesSkipped)
	}
	if stored.IdempotencyKey != key.String() {
		t.Fatalf("idempotency_key = %s", stored.IdempotencyKey)
	}
	if stored.FinishedAt != nil {
		t.Fatalf("finished_at = %v", stored.FinishedAt)
	}

	_, err = repo.CreateRun(ctx, schema.Run{
		ProfileID:      "other",
		Status:         schema.RunStatusRunning,
		StartedAt:      started,
		IdempotencyKey: key,
	})
	if err == nil {
		t.Fatal("expected unique idempotency_key violation")
	}
}

func TestRepository_List_filtersAndSorts(t *testing.T) {
	ctx := context.Background()
	db := testScoringDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	run, err := repo.CreateRun(ctx, schema.Run{
		ProfileID:      "andrew",
		Status:         schema.RunStatusSucceeded,
		StartedAt:      time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		IdempotencyKey: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	insertJob(t, db, "job-high", "High", latest)
	insertJob(t, db, "job-tie-new", "Tie new", newer)
	insertJob(t, db, "job-tie-old", "Tie old", older)
	insertJob(t, db, "job-hidden", "Hidden", latest)
	insertJob(t, db, "job-rejected", "Rejected", latest)
	insertJob(t, db, "job-rejected-hidden", "Rejected hidden", latest)
	insertJob(t, db, "job-other", "Other", latest)

	seeds := []schema.Match{
		{ProfileID: "andrew", JobID: "job-high", Bucket: schema.BucketPassed, Score: 9, UserStatus: schema.UserStatusNew, RunID: run.ID},
		{ProfileID: "andrew", JobID: "job-tie-new", Bucket: schema.BucketPassed, Score: 5, UserStatus: schema.UserStatusNew, RunID: run.ID},
		{ProfileID: "andrew", JobID: "job-tie-old", Bucket: schema.BucketPassed, Score: 5, UserStatus: schema.UserStatusNew, RunID: run.ID},
		{ProfileID: "andrew", JobID: "job-hidden", Bucket: schema.BucketPassed, Score: 50, UserStatus: schema.UserStatusHidden, RunID: run.ID},
		{ProfileID: "andrew", JobID: "job-rejected", Bucket: schema.BucketRejected, Score: 100, UserStatus: schema.UserStatusNew, RunID: run.ID},
		{ProfileID: "andrew", JobID: "job-rejected-hidden", Bucket: schema.BucketRejected, Score: 100, UserStatus: schema.UserStatusHidden, RunID: run.ID},
		{ProfileID: "other", JobID: "job-other", Bucket: schema.BucketPassed, Score: 1000, UserStatus: schema.UserStatusNew, RunID: run.ID},
	}
	for _, m := range seeds {
		if err := repo.UpsertMatch(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	defaultPage, err := repo.List(ctx, schema.ListJobsParams{ProfileID: "andrew", Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if defaultPage.Total != 3 || len(defaultPage.Jobs) != 3 {
		t.Fatalf("default total=%d len=%d", defaultPage.Total, len(defaultPage.Jobs))
	}
	gotIDs := []string{defaultPage.Jobs[0].JobID, defaultPage.Jobs[1].JobID, defaultPage.Jobs[2].JobID}
	wantIDs := []string{"job-high", "job-tie-new", "job-tie-old"}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("order = %v, want %v", gotIDs, wantIDs)
		}
	}
	high := defaultPage.Jobs[0]
	if high.Title != "High" || high.Company != "Acme" || high.Score != 9 || high.Bucket != schema.BucketPassed || high.UserStatus != schema.UserStatusNew {
		t.Fatalf("high = %#v", high)
	}
	if high.Location.Raw != "Berlin" || high.Location.Type != "remote" || len(high.Sources) != 1 || high.Sources[0] != "himalayas" {
		t.Fatalf("location/sources = %#v %#v", high.Location, high.Sources)
	}
	if high.ApplyURL != "https://example.test/job-high/apply" {
		t.Fatalf("apply_url = %s", high.ApplyURL)
	}

	rejected, err := repo.List(ctx, schema.ListJobsParams{
		ProfileID: "andrew",
		Bucket:    schema.BucketRejected,
		Page:      1,
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Total != 1 || len(rejected.Jobs) != 1 || rejected.Jobs[0].JobID != "job-rejected" {
		t.Fatalf("rejected = %#v", rejected.Jobs)
	}

	hidden, err := repo.List(ctx, schema.ListJobsParams{
		ProfileID:  "andrew",
		UserStatus: schema.UserStatusHidden,
		Page:       1,
		Limit:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hidden.Total != 1 || len(hidden.Jobs) != 1 || hidden.Jobs[0].JobID != "job-hidden" || hidden.Jobs[0].UserStatus != schema.UserStatusHidden {
		t.Fatalf("hidden = %#v", hidden.Jobs)
	}

	page1, err := repo.List(ctx, schema.ListJobsParams{ProfileID: "andrew", Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 3 || len(page1.Jobs) != 2 || page1.Jobs[0].JobID != "job-high" || page1.Jobs[1].JobID != "job-tie-new" {
		t.Fatalf("page1 = %#v total %d", page1.Jobs, page1.Total)
	}
	page2, err := repo.List(ctx, schema.ListJobsParams{ProfileID: "andrew", Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page2.Total != 3 || len(page2.Jobs) != 1 || page2.Jobs[0].JobID != "job-tie-old" {
		t.Fatalf("page2 = %#v total %d", page2.Jobs, page2.Total)
	}
}

func TestRepository_RunLookupAndSetUserStatus(t *testing.T) {
	ctx := context.Background()
	db := testScoringDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	seen := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	insertJob(t, db, "job-1", "Engineer", seen)

	olderKey := uuid.New()
	newerKey := uuid.New()
	older, err := repo.CreateRun(ctx, schema.Run{
		ProfileID:      "andrew",
		Status:         schema.RunStatusSucceeded,
		StartedAt:      seen,
		IdempotencyKey: olderKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRun(ctx, older.ID, schema.RunStatusSucceeded, 1, 0, seen.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	newer, err := repo.CreateRun(ctx, schema.Run{
		ProfileID:      "andrew",
		Status:         schema.RunStatusRunning,
		StartedAt:      seen.Add(2 * time.Hour),
		IdempotencyKey: newerKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertMatch(ctx, schema.Match{
		ProfileID:  "andrew",
		JobID:      "job-1",
		Bucket:     schema.BucketPassed,
		Score:      2,
		UserStatus: schema.UserStatusNew,
		RunID:      newer.ID,
	}); err != nil {
		t.Fatal(err)
	}

	latest, err := repo.LatestRun(ctx, "andrew")
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != newer.ID {
		t.Fatalf("latest id = %d, want %d", latest.ID, newer.ID)
	}
	open, err := repo.RunningRun(ctx, "andrew")
	if err != nil || open.ID != newer.ID {
		t.Fatalf("running = %#v err=%v", open, err)
	}
	byKey, err := repo.RunByIdempotencyKey(ctx, newerKey)
	if err != nil || byKey.ID != newer.ID {
		t.Fatalf("by key = %#v err=%v", byKey, err)
	}
	if _, err := repo.LatestRun(ctx, "nobody"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("missing latest err = %v", err)
	}

	updated, err := repo.SetUserStatus(ctx, "andrew", "job-1", schema.UserStatusHidden)
	if err != nil {
		t.Fatal(err)
	}
	if updated.UserStatus != schema.UserStatusHidden || updated.Score != 2 {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := repo.SetUserStatus(ctx, "andrew", "missing", schema.UserStatusHidden); !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("missing match err = %v", err)
	}
}
