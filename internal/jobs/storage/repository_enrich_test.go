package storage

import (
	"context"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
)

func TestRepository_ListEnrichCandidates(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	since := now.Add(-30 * 24 * time.Hour)
	postedAfter := now.Add(-30 * 24 * time.Hour)
	inWindow := now.Add(-5 * 24 * time.Hour)
	outside := now.Add(-40 * 24 * time.Hour)
	postedFresh := now.Add(-10 * 24 * time.Hour)
	postedStale := now.Add(-60 * 24 * time.Hour)
	closedAt := now.Add(-1 * 24 * time.Hour)

	seed := func(id string, firstSeen time.Time, posted *time.Time, closed *time.Time) {
		t.Helper()
		if err := db.Exec(`
			INSERT INTO jobs (
				id, sources, title, company, url, description, location, openings, tags,
				first_seen_at, last_seen_at, created_at, updated_at
			) VALUES (?, '["himalayas"]', ?, 'Acme', ?, 'desc', '{}', '[]', '[]', ?, ?, ?, ?)`,
			id, "Engineer "+id, "https://example.test/"+id, firstSeen, firstSeen, firstSeen, firstSeen,
		).Error; err != nil {
			t.Fatal(err)
		}
		if posted != nil {
			if err := db.Exec(`UPDATE jobs SET posted_at = ? WHERE id = ?`, posted, id).Error; err != nil {
				t.Fatal(err)
			}
		}
		if closed != nil {
			if err := db.Exec(`UPDATE jobs SET closed_at = ?, closed_by = ? WHERE id = ?`, closed, "ATS", id).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	seed("posted-unknown", inWindow, nil, nil)        // unknown posted_at stays eligible
	seed("posted-fresh", inWindow, &postedFresh, nil) // posted_at within cutoff
	seed("posted-stale", inWindow, &postedStale, nil) // posted_at older than cutoff
	seed("open-out", outside, nil, nil)               // outside first_seen window
	seed("closed-in", inWindow, nil, &closedAt)       // closed

	got, err := repo.ListEnrichCandidates(ctx, since, postedAfter)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{}{
		"posted-unknown": {},
		"posted-fresh":   {},
	}
	if len(got) != len(want) {
		t.Fatalf("ListEnrichCandidates len = %d (%#v), want %d ids %v", len(got), got, len(want), want)
	}
	for _, job := range got {
		if _, ok := want[job.ID]; !ok {
			t.Fatalf("ListEnrichCandidates unexpected id %q in %#v", job.ID, got)
		}
	}
}
