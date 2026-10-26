package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testCompaniesDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	stmt := `CREATE TABLE companies (
		key TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		aliases TEXT NOT NULL DEFAULT '{}',
		website TEXT NOT NULL DEFAULT '',
		ats TEXT NOT NULL DEFAULT '',
		ats_slug TEXT NOT NULL DEFAULT '',
		board_url TEXT NOT NULL DEFAULT '',
		board_etag TEXT NOT NULL DEFAULT '',
		resolution_status TEXT NOT NULL,
		resolution_method TEXT NOT NULL DEFAULT '',
		poll_state TEXT NOT NULL DEFAULT 'ACTIVE',
		next_resolve_at TIMESTAMP,
		first_seen_at TIMESTAMP NOT NULL,
		last_seen_at TIMESTAMP NOT NULL,
		last_polled_at TIMESTAMP,
		poll_fail_count INTEGER NOT NULL DEFAULT 0,
		empty_poll_streak INTEGER NOT NULL DEFAULT 0,
		needs_review INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`
	if err := db.Exec(stmt).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func sampleCompany(key, name string) schema.Company {
	seen := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	return schema.Company{
		Key:              key,
		Name:             name,
		Website:          "",
		ResolutionStatus: schema.ResolutionStatusPending,
		PollState:        schema.PollStateActive,
		FirstSeenAt:      seen,
		LastSeenAt:       seen,
	}
}

func TestRepository_Upsert_insertAndUpdate(t *testing.T) {
	ctx := context.Background()
	db := testCompaniesDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	c := sampleCompany("acme", "Acme")
	if err := repo.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByKey(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Acme" || got.ResolutionStatus != schema.ResolutionStatusPending {
		t.Fatalf("got = %#v", got)
	}
	firstSeen := got.FirstSeenAt

	c.Name = "Acme Inc"
	c.Website = "https://acme.example"
	c.Aliases = []string{"acme.com"}
	c.LastSeenAt = firstSeen.Add(time.Hour)
	c.FirstSeenAt = firstSeen.Add(2 * time.Hour) // must not overwrite
	if err := repo.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByKey(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Acme Inc" || got.Website != "https://acme.example" {
		t.Fatalf("updated fields: %#v", got)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "acme.com" {
		t.Fatalf("aliases = %#v", got.Aliases)
	}
	if !got.FirstSeenAt.Equal(firstSeen) {
		t.Fatalf("first_seen_at = %v, want %v", got.FirstSeenAt, firstSeen)
	}
}

func TestRepository_FindByAlias(t *testing.T) {
	ctx := context.Background()
	db := testCompaniesDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	c := sampleCompany("yld", "YLD")
	c.Aliases = []string{"yld.com", "yld ltd"}
	if err := repo.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByAlias(ctx, "yld.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "yld" {
		t.Fatalf("key = %q", got.Key)
	}

	_, err = repo.FindByAlias(ctx, "missing")
	if !errors.Is(err, companies.ErrCompanyNotFound) {
		t.Fatalf("err = %v, want ErrCompanyNotFound", err)
	}
}

func TestRepository_FindByWebsite(t *testing.T) {
	ctx := context.Background()
	db := testCompaniesDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	c := sampleCompany("storyblok", "Storyblok")
	c.Website = "https://www.storyblok.com"
	if err := repo.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByWebsite(ctx, "https://www.storyblok.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "storyblok" {
		t.Fatalf("key = %q", got.Key)
	}
}

func TestRepository_ListDuePollable_orderingAndCap(t *testing.T) {
	ctx := context.Background()
	db := testCompaniesDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	dueBefore := now.Add(-6 * time.Hour)

	insert := func(key string, lastPolled *time.Time, status schema.ResolutionStatus, poll schema.PollState) {
		t.Helper()
		c := sampleCompany(key, key)
		c.ResolutionStatus = status
		c.PollState = poll
		c.ATS = schema.ATSGreenhouse
		c.ATSSlug = key
		if err := repo.Upsert(ctx, c); err != nil {
			t.Fatal(err)
		}
		if lastPolled != nil {
			if err := db.Exec(`UPDATE companies SET last_polled_at = ? WHERE key = ?`, *lastPolled, key).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	old := now.Add(-24 * time.Hour)
	mid := now.Add(-12 * time.Hour)
	fresh := now.Add(-1 * time.Hour) // not due
	insert("never", nil, schema.ResolutionStatusResolved, schema.PollStateActive)
	insert("old", &old, schema.ResolutionStatusResolved, schema.PollStateActive)
	insert("mid", &mid, schema.ResolutionStatusResolved, schema.PollStateActive)
	insert("fresh", &fresh, schema.ResolutionStatusResolved, schema.PollStateActive)
	insert("pending", nil, schema.ResolutionStatusPending, schema.PollStateActive)
	insert("inactive", &old, schema.ResolutionStatusResolved, schema.PollStateInactive)

	got, err := repo.ListDuePollable(ctx, dueBefore, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Key != "never" {
		t.Fatalf("first = %q, want never (nulls first)", got[0].Key)
	}
	if got[1].Key != "old" {
		t.Fatalf("second = %q, want old", got[1].Key)
	}

	// Under the raised default cap every due company fits; order must stay
	// NULLS FIRST, then oldest first (fresh / pending / inactive stay out).
	got, err = repo.ListDuePollable(ctx, dueBefore, config.DefaultCompaniesPollMaxPerRun)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, len(got))
	for i, c := range got {
		keys[i] = c.Key
	}
	want := []string{"never", "old", "mid"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %v, want %v", keys, want)
		}
	}
}

func TestRepository_ListPendingResolve(t *testing.T) {
	ctx := context.Background()
	db := testCompaniesDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	due := sampleCompany("due", "Due")
	if err := repo.Upsert(ctx, due); err != nil {
		t.Fatal(err)
	}
	future := sampleCompany("future", "Future")
	if err := repo.Upsert(ctx, future); err != nil {
		t.Fatal(err)
	}
	next := now.Add(24 * time.Hour)
	if err := db.Exec(`UPDATE companies SET next_resolve_at = ? WHERE key = ?`, next, "future").Error; err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListPendingResolve(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "due" {
		t.Fatalf("got = %#v", got)
	}
}

func TestRepository_MarkPolledAndMarkResolution(t *testing.T) {
	ctx := context.Background()
	db := testCompaniesDB(t)
	repo := NewRepository(pgsql.NewGetter(db))

	if err := repo.Upsert(ctx, sampleCompany("acme", "Acme")); err != nil {
		t.Fatal(err)
	}
	polledAt := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	if err := repo.MarkPolled(ctx, schema.MarkPolledParams{
		Key:             "acme",
		LastPolledAt:    polledAt,
		BoardETag:       `"etag-1"`,
		PollFailCount:   0,
		EmptyPollStreak: 1,
		PollState:       schema.PollStateActive,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByKey(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.BoardETag != `"etag-1"` || got.EmptyPollStreak != 1 || got.LastPolledAt == nil || !got.LastPolledAt.Equal(polledAt) {
		t.Fatalf("after MarkPolled: %#v", got)
	}

	if err := repo.MarkResolution(ctx, schema.MarkResolutionParams{
		Key:              "acme",
		ResolutionStatus: schema.ResolutionStatusResolved,
		ResolutionMethod: schema.ResolutionMethodApplyURL,
		ATS:              schema.ATSGreenhouse,
		ATSSlug:          "Acme",
		BoardURL:         "https://boards.greenhouse.io/Acme",
	}); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByKey(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.ResolutionStatus != schema.ResolutionStatusResolved || got.ATS != schema.ATSGreenhouse || got.ATSSlug != "Acme" {
		t.Fatalf("after MarkResolution: %#v", got)
	}
}
