package storage

import (
	"context"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
	jobschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
)

func TestRepository_BackfillCompanyKeyRef_linksAggregatorJob(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	// Aggregator ingest leaves company_key_ref NULL even when the company is RESOLVED.
	job := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://himalayas.example/jobs/1",
		ApplyURL: "https://agg.example/apply/1",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&job); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, job); err != nil {
		t.Fatal(err)
	}
	if job.CompanyKey == "" {
		t.Fatal("expected company_key from AssignStableID")
	}

	before, err := repo.CountOpenByCompanyKeyRef(ctx, job.CompanyKey)
	if err != nil {
		t.Fatal(err)
	}
	if before != 0 {
		t.Fatalf("count before backfill = %d, want 0", before)
	}

	// Discover path: after merge-or-insert, backfill by content company_key.
	n, err := repo.BackfillCompanyKeyRef(ctx, job.CompanyKey, []string{job.CompanyKey})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("backfill rows = %d, want 1", n)
	}

	var row Job
	if err := db.Where("id = ?", job.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CompanyKeyRef == nil || *row.CompanyKeyRef != job.CompanyKey {
		t.Fatalf("company_key_ref = %v, want %q", row.CompanyKeyRef, job.CompanyKey)
	}
	if row.ClosedAt != nil || row.ClosedBy != "" {
		t.Fatalf("closed columns touched: closed_at=%v closed_by=%q", row.ClosedAt, row.ClosedBy)
	}
	if row.BoardSeenAt != nil {
		t.Fatalf("backfill must not set board_seen_at: %v", row.BoardSeenAt)
	}

	after, err := repo.CountOpenByCompanyKeyRef(ctx, job.CompanyKey)
	if err != nil {
		t.Fatal(err)
	}
	if after != 1 {
		t.Fatalf("count after backfill = %d, want 1", after)
	}

	// Idempotent: second discover pass does not re-write already-linked rows.
	n2, err := repo.BackfillCompanyKeyRef(ctx, job.CompanyKey, []string{job.CompanyKey})
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second backfill rows = %d, want 0", n2)
	}
}

func TestRepository_SaveIngest_preservesATSClose(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	job := jobdata.Job{
		Source:   "himalayas",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://himalayas.example/jobs/1",
		ApplyURL: "https://agg.example/apply/1",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&job); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, job); err != nil {
		t.Fatal(err)
	}

	companyKey := "acme"
	closedAt := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	if err := db.Exec(
		`UPDATE jobs SET company_key_ref = ?, closed_at = ?, closed_by = ? WHERE id = ?`,
		companyKey, closedAt, string(jobschema.ClosedByATS), job.ID,
	).Error; err != nil {
		t.Fatal(err)
	}

	again := job
	again.Source = "builtin"
	again.Sources = nil
	again.URL = "https://builtin.example/jobs/9"
	again.ApplyURL = "https://builtin.example/apply/9"
	again.CompanyWebsite = "https://acme.example"
	again.Location = jobdata.Location{Raw: "Remote"}
	if _, err := repo.SaveIngest(ctx, again); err != nil {
		t.Fatal(err)
	}

	var row Job
	if err := db.Where("id = ?", job.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClosedAt == nil || !row.ClosedAt.Equal(closedAt) {
		t.Fatalf("closed_at = %v, want %v", row.ClosedAt, closedAt)
	}
	if row.ClosedBy != string(jobschema.ClosedByATS) {
		t.Fatalf("closed_by = %q, want ATS", row.ClosedBy)
	}
	if row.CompanyKeyRef == nil || *row.CompanyKeyRef != companyKey {
		t.Fatalf("company_key_ref = %v", row.CompanyKeyRef)
	}
	if row.CompanyWebsite != "https://acme.example" {
		t.Fatalf("company_website = %q", row.CompanyWebsite)
	}
	if !row.LastSeenAt.After(closedAt) {
		t.Fatalf("last_seen_at = %v, want after close", row.LastSeenAt)
	}
}

func TestRepository_ReopenPresent_clearsATSClose(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	job := jobdata.Job{
		Source:   "greenhouse",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://boards.greenhouse.io/acme/jobs/1",
		ApplyURL: "https://boards.greenhouse.io/acme/jobs/1",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&job); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, job); err != nil {
		t.Fatal(err)
	}

	companyKey := "acme"
	closedAt := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	if err := db.Exec(
		`UPDATE jobs SET company_key_ref = ?, closed_at = ?, closed_by = ? WHERE id = ?`,
		companyKey, closedAt, string(jobschema.ClosedByATS), job.ID,
	).Error; err != nil {
		t.Fatal(err)
	}

	n, err := repo.ReopenPresent(ctx, companyKey, []string{job.ID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}

	var row Job
	if err := db.Where("id = ?", job.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClosedAt != nil {
		t.Fatalf("closed_at = %v, want nil", row.ClosedAt)
	}
	if row.ClosedBy != "" {
		t.Fatalf("closed_by = %q, want empty", row.ClosedBy)
	}
}

func TestRepository_CloseAbsent_closesMissing(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	boardJob := func(title, url string) jobdata.Job {
		t.Helper()
		job := jobdata.Job{
			Source:   "greenhouse",
			Title:    title,
			Company:  "Acme",
			URL:      url,
			ApplyURL: url,
			Location: jobdata.Location{Raw: "Berlin"},
		}
		if err := domainutils.AssignStableID(&job); err != nil {
			t.Fatal(err)
		}
		return job
	}

	keep := boardJob("Keep", "https://boards.greenhouse.io/acme/jobs/keep")
	gone := boardJob("Gone", "https://boards.greenhouse.io/acme/jobs/gone")
	if err := repo.UpsertBoardJobs(ctx, "acme", []jobdata.Job{keep, gone}); err != nil {
		t.Fatal(err)
	}

	n, err := repo.CloseAbsent(ctx, "acme", []string{keep.ID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}

	var keepRow, goneRow Job
	if err := db.Where("id = ?", keep.ID).First(&keepRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", gone.ID).First(&goneRow).Error; err != nil {
		t.Fatal(err)
	}
	if keepRow.ClosedAt != nil {
		t.Fatalf("keep closed_at = %v", keepRow.ClosedAt)
	}
	if goneRow.ClosedAt == nil || goneRow.ClosedBy != string(jobschema.ClosedByATS) {
		t.Fatalf("gone closed_at=%v closed_by=%q", goneRow.ClosedAt, goneRow.ClosedBy)
	}
}

// R1.4 (a): company_key_ref from discover does not authorize CloseAbsent.
func TestRepository_CloseAbsent_skipsAggregatorOnlyLinked(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	agg := jobdata.Job{
		Source:   "himalayas",
		Title:    "Aggregator Only",
		Company:  "Acme",
		URL:      "https://himalayas.example/jobs/agg",
		ApplyURL: "https://agg.example/apply/agg",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&agg); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIngest(ctx, agg); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.BackfillCompanyKeyRef(ctx, "acme", []string{agg.CompanyKey}); err != nil {
		t.Fatal(err)
	}

	board := jobdata.Job{
		Source:   "greenhouse",
		Title:    "Board Role",
		Company:  "Acme",
		URL:      "https://boards.greenhouse.io/acme/jobs/board",
		ApplyURL: "https://boards.greenhouse.io/acme/jobs/board",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&board); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertBoardJobs(ctx, "acme", []jobdata.Job{board}); err != nil {
		t.Fatal(err)
	}

	// Complete poll that lists neither: only the board-seen row closes.
	n, err := repo.CloseAbsent(ctx, "acme", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1 (board-seen only)", n)
	}

	var aggRow, boardRow Job
	if err := db.Where("id = ?", agg.ID).First(&aggRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", board.ID).First(&boardRow).Error; err != nil {
		t.Fatal(err)
	}
	if aggRow.ClosedAt != nil {
		t.Fatalf("aggregator-only closed_at=%v, want nil", aggRow.ClosedAt)
	}
	if aggRow.BoardSeenAt != nil {
		t.Fatalf("aggregator-only board_seen_at=%v, want nil", aggRow.BoardSeenAt)
	}
	if boardRow.ClosedAt == nil || boardRow.ClosedBy != string(jobschema.ClosedByATS) {
		t.Fatalf("board-seen closed_at=%v closed_by=%q", boardRow.ClosedAt, boardRow.ClosedBy)
	}
}

func TestRepository_UpsertBoardJobs_skipsStalePostedAt(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Now().UTC()
	day := 24 * time.Hour

	boardJob := func(title, url string, postedAt time.Time) jobdata.Job {
		t.Helper()
		job := jobdata.Job{
			Source:   "greenhouse",
			Title:    title,
			Company:  "Acme",
			URL:      url,
			ApplyURL: url,
			Location: jobdata.Location{Raw: "Berlin"},
			PostedAt: postedAt,
		}
		if err := domainutils.AssignStableID(&job); err != nil {
			t.Fatal(err)
		}
		return job
	}

	stale := boardJob("Stale", "https://boards.greenhouse.io/acme/jobs/stale", now.Add(-60*day))
	fresh := boardJob("Fresh", "https://boards.greenhouse.io/acme/jobs/fresh", now.Add(-10*day))
	unknown := boardJob("Unknown", "https://boards.greenhouse.io/acme/jobs/unknown", time.Time{})

	if err := repo.UpsertBoardJobs(ctx, "acme", []jobdata.Job{stale, fresh, unknown}); err != nil {
		t.Fatal(err)
	}

	var staleCount int64
	if err := db.Model(&Job{}).Where("id = ?", stale.ID).Count(&staleCount).Error; err != nil {
		t.Fatal(err)
	}
	if staleCount != 0 {
		t.Fatalf("stale job written, count=%d", staleCount)
	}

	var freshRow, unknownRow Job
	if err := db.Where("id = ?", fresh.ID).First(&freshRow).Error; err != nil {
		t.Fatalf("fresh job missing: %v", err)
	}
	if err := db.Where("id = ?", unknown.ID).First(&unknownRow).Error; err != nil {
		t.Fatalf("unknown posted_at job missing: %v", err)
	}
	if freshRow.BoardSeenAt == nil || unknownRow.BoardSeenAt == nil {
		t.Fatal("expected board_seen_at on written rows")
	}
	if unknownRow.PostedAt != nil {
		t.Fatalf("unknown posted_at = %v, want nil", unknownRow.PostedAt)
	}
}

func TestRepository_UpsertBoardJobs_preservesCloseOnFreshUpsert(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Now().UTC()

	closed := jobdata.Job{
		Source:   "greenhouse",
		Title:    "Closed Role",
		Company:  "Acme",
		URL:      "https://boards.greenhouse.io/acme/jobs/closed",
		ApplyURL: "https://boards.greenhouse.io/acme/jobs/closed",
		Location: jobdata.Location{Raw: "Berlin"},
		PostedAt: now.Add(-10 * 24 * time.Hour),
	}
	if err := domainutils.AssignStableID(&closed); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertBoardJobs(ctx, "acme", []jobdata.Job{closed}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CloseAbsent(ctx, "acme", nil); err != nil {
		t.Fatal(err)
	}
	var before Job
	if err := db.Where("id = ?", closed.ID).First(&before).Error; err != nil {
		t.Fatal(err)
	}
	if before.ClosedAt == nil || before.ClosedBy != string(jobschema.ClosedByATS) || before.BoardSeenAt == nil {
		t.Fatalf("setup: closed_at=%v closed_by=%q board_seen_at=%v", before.ClosedAt, before.ClosedBy, before.BoardSeenAt)
	}
	closedAt := *before.ClosedAt
	boardSeenAt := *before.BoardSeenAt

	freshSibling := jobdata.Job{
		Source:   "greenhouse",
		Title:    "Open Role",
		Company:  "Acme",
		URL:      "https://boards.greenhouse.io/acme/jobs/open",
		ApplyURL: "https://boards.greenhouse.io/acme/jobs/open",
		Location: jobdata.Location{Raw: "Berlin"},
		PostedAt: now.Add(-5 * 24 * time.Hour),
	}
	if err := domainutils.AssignStableID(&freshSibling); err != nil {
		t.Fatal(err)
	}
	// Re-upsert the closed vacancy with fresh posted_at: close columns must stick.
	again := closed
	again.PostedAt = now.Add(-5 * 24 * time.Hour)
	if err := repo.UpsertBoardJobs(ctx, "acme", []jobdata.Job{again, freshSibling}); err != nil {
		t.Fatal(err)
	}

	var after Job
	if err := db.Where("id = ?", closed.ID).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ClosedAt == nil || !after.ClosedAt.Equal(closedAt) {
		t.Fatalf("closed_at = %v, want %v", after.ClosedAt, closedAt)
	}
	if after.ClosedBy != string(jobschema.ClosedByATS) {
		t.Fatalf("closed_by = %q, want ATS", after.ClosedBy)
	}
	if after.BoardSeenAt == nil {
		t.Fatal("board_seen_at cleared")
	}
	if !after.BoardSeenAt.After(boardSeenAt) && !after.BoardSeenAt.Equal(boardSeenAt) {
		// board_seen_at is refreshed on upsert; either equal (same clock) or after is fine
		t.Fatalf("board_seen_at = %v, want >= %v", after.BoardSeenAt, boardSeenAt)
	}
}

func TestRepository_closeProtectionSequence(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	job := jobdata.Job{
		Source:   "greenhouse",
		Title:    "Engineer",
		Company:  "Acme",
		URL:      "https://boards.greenhouse.io/acme/jobs/1",
		ApplyURL: "https://boards.greenhouse.io/acme/jobs/1",
		Location: jobdata.Location{Raw: "Berlin"},
	}
	if err := domainutils.AssignStableID(&job); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertBoardJobs(ctx, "acme", []jobdata.Job{job}); err != nil {
		t.Fatal(err)
	}

	// Complete poll: vacancy absent → close (board_seen_at set by UpsertBoardJobs).
	if _, err := repo.CloseAbsent(ctx, "acme", nil); err != nil {
		t.Fatal(err)
	}
	var row Job
	if err := db.Where("id = ?", job.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClosedAt == nil || row.ClosedBy != string(jobschema.ClosedByATS) {
		t.Fatalf("after close: closed_at=%v closed_by=%q", row.ClosedAt, row.ClosedBy)
	}
	closedAt := *row.ClosedAt

	// Aggregator lag: upsert must leave close intact.
	lag := job
	lag.Source = "builtin"
	lag.Sources = nil
	lag.URL = "https://builtin.example/jobs/1"
	if _, err := repo.SaveIngest(ctx, lag); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", job.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClosedAt == nil || !row.ClosedAt.Equal(closedAt) || row.ClosedBy != string(jobschema.ClosedByATS) {
		t.Fatalf("after aggregator upsert: closed_at=%v closed_by=%q", row.ClosedAt, row.ClosedBy)
	}
	if row.BoardSeenAt == nil {
		t.Fatal("aggregator upsert cleared board_seen_at")
	}

	// Complete poll listing it again → reopen.
	n, err := repo.ReopenPresent(ctx, "acme", []string{job.ID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("reopen rows = %d, want 1", n)
	}
	var reopened Job
	if err := db.Where("id = ?", job.ID).First(&reopened).Error; err != nil {
		t.Fatal(err)
	}
	if reopened.ClosedAt != nil || reopened.ClosedBy != "" {
		t.Fatalf("after reopen: closed_at=%v closed_by=%q", reopened.ClosedAt, reopened.ClosedBy)
	}
}
