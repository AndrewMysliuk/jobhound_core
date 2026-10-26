package impl

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/ats/greenhouse"
	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func TestNormalizeNameAndDomain(t *testing.T) {
	if got := NormalizeName("Acme Inc"); got != "acme" {
		t.Fatalf("name: got %q", got)
	}
	if got := NormalizeDomain("https://www.careers.acme.com/jobs"); got != "acme.com" {
		t.Fatalf("domain: got %q", got)
	}
	if got := ChooseKey("Acme Inc", "https://www.acme.com"); got != "acme.com" {
		t.Fatalf("choose domain: got %q", got)
	}
	if got := ChooseKey("Acme Inc", ""); got != "acme" {
		t.Fatalf("choose name: got %q", got)
	}
}

func TestLoadBlocklist_fromConfigDir(t *testing.T) {
	m, err := LoadBlocklist("../../../config/companies")
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Check(NormalizeName("lemon.io"), ""); !d.Blocked {
		t.Fatal("expected lemon.io blocked from real file")
	}
	if d := m.Check(NormalizeName("lemon.markets"), ""); d.Blocked {
		t.Fatal("lemon.markets must stay unblocked")
	}
}

func TestBlocklist_exactAnonymousReview(t *testing.T) {
	m := NewBlocklistMatcher(schema.Blocklist{
		Exact:     []string{"lemon.io", "Proxify"},
		Anonymous: []string{"Confidential"},
		Review:    []string{"recruit", "consulting services"},
	})
	if d := m.Check(NormalizeName("lemon.io"), ""); !d.Blocked {
		t.Fatal("lemon.io should block")
	}
	if d := m.Check(NormalizeName("lemon.markets"), ""); d.Blocked {
		t.Fatal("lemon.markets must not block")
	}
	if d := m.Check(NormalizeName("Confidential"), ""); !d.Blocked {
		t.Fatal("anonymous should block")
	}
	if d := m.Check(NormalizeName("Talent Recruiters"), ""); d.Blocked || !d.NeedsReview {
		t.Fatalf("review: %+v", d)
	}
	if d := m.Check(NormalizeName("cloudbees"), ""); d.Blocked || d.NeedsReview {
		t.Fatalf("cloudbees: %+v", d)
	}
}

func TestUpsertFromAggregator_blocklistAndMerge(t *testing.T) {
	ctx := context.Background()
	store := newFakeCompanyStore()
	bl := NewBlocklistMatcher(schema.Blocklist{
		Exact:  []string{"lemon.io"},
		Review: []string{"talent"},
	})
	svc := New(store, nil, bl, nil, config.Companies{ResolveRetryDays: 14})

	blocked, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{Name: "lemon.io"})
	if err != nil {
		t.Fatal(err)
	}
	if !blocked.Blocked || blocked.Created || blocked.Merged {
		t.Fatalf("blocked result: %+v", blocked)
	}
	if len(store.byKey) != 0 {
		t.Fatal("blocked must not create a row")
	}

	// Name-form birth, then domain-form sighting → one row + alias.
	r1, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{Name: "YLD"})
	if err != nil || !r1.Created {
		t.Fatalf("create: %+v err=%v", r1, err)
	}
	r2, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{
		Name:    "YLD",
		Website: "https://www.yld.com",
	})
	if err != nil || !r2.Merged || r2.Key != "yld" {
		t.Fatalf("merge: %+v err=%v", r2, err)
	}
	c := store.byKey["yld"]
	if c.Website != "https://www.yld.com" {
		t.Fatalf("website=%q", c.Website)
	}
	if !contains(c.Aliases, "yld.com") {
		t.Fatalf("aliases=%v want yld.com", c.Aliases)
	}
	if len(store.byKey) != 1 {
		t.Fatalf("rows=%d", len(store.byKey))
	}

	// Domain-form birth, then name-only sighting → still one row.
	rDom, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{
		Name:    "Foo Labs",
		Website: "https://foo.example",
	})
	if err != nil || !rDom.Created || rDom.Key != "foo.example" {
		t.Fatalf("domain create: %+v err=%v", rDom, err)
	}
	rName, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{Name: "Foo Labs"})
	if err != nil || !rName.Merged || rName.Key != "foo.example" {
		t.Fatalf("name merge into domain: %+v err=%v", rName, err)
	}

	// Review flag.
	r3, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{Name: "Acme Talent Hub"})
	if err != nil || !r3.Created {
		t.Fatalf("review create: %+v err=%v", r3, err)
	}
	if !store.byKey[r3.Key].NeedsReview {
		t.Fatal("needs_review expected")
	}
}

func TestUpsertFromAggregator_inactiveWakesToActive(t *testing.T) {
	ctx := context.Background()
	store := newFakeCompanyStore()
	store.byKey["acme"] = schema.Company{
		Key:              "acme",
		Name:             "Acme",
		ResolutionStatus: schema.ResolutionStatusResolved,
		PollState:        schema.PollStateInactive,
		EmptyPollStreak:  5,
		ATS:              schema.ATSGreenhouse,
		ATSSlug:          "acme",
	}
	svc := New(store, nil, nil, nil, config.Companies{})
	r, err := svc.UpsertFromAggregator(ctx, schema.UpsertCompanyParams{Name: "Acme"})
	if err != nil || !r.Merged {
		t.Fatalf("%+v %v", r, err)
	}
	c := store.byKey["acme"]
	if c.PollState != schema.PollStateActive || c.EmptyPollStreak != 0 {
		t.Fatalf("wake: state=%s streak=%d", c.PollState, c.EmptyPollStreak)
	}
}

func TestResolve_applyURLUnsupportedAndUnresolved(t *testing.T) {
	ctx := context.Background()
	cfg := config.Companies{ResolveRetryDays: 14}
	fixed := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	t.Run("greenhouse raw host", func(t *testing.T) {
		store := newFakeCompanyStore()
		store.byKey["acme"] = pendingWithBoard("acme", "https://boards.greenhouse.io/AcmeCo/jobs/1")
		svc := New(store, nil, nil, nil, cfg)
		svc.now = func() time.Time { return fixed }
		c, err := svc.Resolve(ctx, "acme")
		if err != nil {
			t.Fatal(err)
		}
		if c.ResolutionStatus != schema.ResolutionStatusResolved ||
			c.ATS != schema.ATSGreenhouse ||
			c.ATSSlug != "AcmeCo" ||
			c.ResolutionMethod != schema.ResolutionMethodApplyURL {
			t.Fatalf("%+v", c)
		}
		// Slug casing preserved.
		if store.byKey["acme"].ATSSlug != "AcmeCo" {
			t.Fatalf("slug=%q", store.byKey["acme"].ATSSlug)
		}
	})

	t.Run("unsupported short-circuit", func(t *testing.T) {
		store := newFakeCompanyStore()
		store.byKey["wd"] = pendingWithBoard("wd", "https://acme.myworkdayjobs.com/en-US/Careers")
		svc := New(store, nil, nil, nil, cfg)
		svc.now = func() time.Time { return fixed }
		c, err := svc.Resolve(ctx, "wd")
		if err != nil {
			t.Fatal(err)
		}
		if c.ResolutionStatus != schema.ResolutionStatusUnsupportedATS ||
			c.ATS != schema.ATSWorkday ||
			c.NextResolveAt != nil {
			t.Fatalf("%+v", c)
		}
	})

	t.Run("redirect then greenhouse", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://jobs.lever.co/redirected-slug/xyz", http.StatusFound)
		}))
		defer ts.Close()
		store := newFakeCompanyStore()
		store.byKey["redir"] = pendingWithBoard("redir", ts.URL+"/go")
		svc := New(store, nil, nil, nil, cfg)
		svc.now = func() time.Time { return fixed }
		// httptest final host is the test server after redirect to lever — custom
		// transport that rewrites the lever target is awkward; parse the
		// Location via a client that does not follow, then feed parseATSURL.
		// Instead: point BoardURL at a server that 302s to a path we control
		// by serving lever-shaped final URL on the same server is impossible.
		// Use a round-tripper that returns a final Request URL of jobs.lever.co.
		svc.http = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				final, _ := http.NewRequest(http.MethodGet, "https://jobs.lever.co/redirected-slug/xyz", nil)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    final,
					Header:     make(http.Header),
				}, nil
			}),
		}
		c, err := svc.Resolve(ctx, "redir")
		if err != nil {
			t.Fatal(err)
		}
		if c.ResolutionStatus != schema.ResolutionStatusResolved ||
			c.ATS != schema.ATSLever ||
			c.ATSSlug != "redirected-slug" {
			t.Fatalf("%+v", c)
		}
	})

	t.Run("unresolved sets next_resolve_at", func(t *testing.T) {
		store := newFakeCompanyStore()
		store.byKey["unk"] = pendingWithBoard("unk", "https://careers.example.com/jobs/1")
		svc := New(store, nil, nil, nil, cfg)
		svc.now = func() time.Time { return fixed }
		svc.http = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
					Header:     make(http.Header),
				}, nil
			}),
		}
		c, err := svc.Resolve(ctx, "unk")
		if err != nil {
			t.Fatal(err)
		}
		want := fixed.Add(14 * 24 * time.Hour)
		if c.ResolutionStatus != schema.ResolutionStatusUnresolved ||
			c.NextResolveAt == nil || !c.NextResolveAt.Equal(want) {
			t.Fatalf("%+v", c)
		}
	})
}

func TestDuePollable_intervalAndCap(t *testing.T) {
	ctx := context.Background()
	store := newFakeCompanyStore()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	svc := New(store, nil, nil, nil, config.Companies{
		PollMaxPerRun:     2,
		PollIntervalHours: 6,
	})
	svc.now = func() time.Time { return now }

	got, err := svc.DuePollable(ctx, now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if store.lastDueLimit != 2 {
		t.Fatalf("limit=%d", store.lastDueLimit)
	}
	wantBefore := now.Add(-6 * time.Hour)
	if !store.lastDueBefore.Equal(wantBefore) {
		t.Fatalf("dueBefore=%v want %v", store.lastDueBefore, wantBefore)
	}
	_ = got
}

func TestPoll_allOutcomesAndStreaks(t *testing.T) {
	ctx := context.Background()
	fixed := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	cfg := config.Companies{PollIntervalHours: 6, PollMaxPerRun: 50}

	newResolved := func(store *fakeCompanyStore, key string) {
		store.byKey[key] = schema.Company{
			Key:              key,
			Name:             key,
			ATS:              schema.ATSGreenhouse,
			ATSSlug:          "SlugCase",
			BoardETag:        "etag-1",
			ResolutionStatus: schema.ResolutionStatusResolved,
			ResolutionMethod: schema.ResolutionMethodApplyURL,
			PollState:        schema.PollStateActive,
		}
	}

	t.Run("NOT_MODIFIED", func(t *testing.T) {
		store := newFakeCompanyStore()
		jobs := newFakeJobs()
		newResolved(store, "acme")
		fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, result: schema.BoardResult{
			Outcome: schema.PollOutcomeNotModified,
			ETag:    "etag-1",
		}}
		svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
		svc.now = func() time.Time { return fixed }
		out, err := svc.Poll(ctx, "acme")
		if err != nil || out.Outcome != schema.PollOutcomeNotModified {
			t.Fatalf("%v %v", out, err)
		}
		if len(jobs.upserted) != 0 || jobs.closedCalls != 0 {
			t.Fatal("not_modified must not upsert or close")
		}
		if store.byKey["acme"].LastPolledAt == nil {
			t.Fatal("last_polled_at")
		}
		if fetcher.lastSlug != "SlugCase" {
			t.Fatalf("slug sent=%q", fetcher.lastSlug)
		}
	})

	t.Run("COMPLETE upserts and closes", func(t *testing.T) {
		store := newFakeCompanyStore()
		jobs := newFakeJobs()
		newResolved(store, "acme")
		fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, result: schema.BoardResult{
			Outcome: schema.PollOutcomeComplete,
			ETag:    "etag-2",
			Jobs:    []jobdata.Job{{ID: "j1", Title: "Eng"}},
		}}
		svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
		svc.now = func() time.Time { return fixed }
		out, err := svc.Poll(ctx, "acme")
		if err != nil || out.Outcome != schema.PollOutcomeComplete {
			t.Fatalf("%v %v", out, err)
		}
		if len(jobs.upserted) != 1 || jobs.closedCalls != 1 || jobs.reopenCalls != 1 {
			t.Fatalf("upsert=%d close=%d reopen=%d", len(jobs.upserted), jobs.closedCalls, jobs.reopenCalls)
		}
		if store.byKey["acme"].BoardETag != "etag-2" || store.byKey["acme"].EmptyPollStreak != 0 {
			t.Fatalf("%+v", store.byKey["acme"])
		}
	})

	t.Run("PARTIAL upserts closes nothing", func(t *testing.T) {
		store := newFakeCompanyStore()
		jobs := newFakeJobs()
		newResolved(store, "acme")
		fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, result: schema.BoardResult{
			Outcome: schema.PollOutcomePartial,
			Jobs:    []jobdata.Job{{ID: "j1"}},
		}}
		svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
		svc.now = func() time.Time { return fixed }
		out, err := svc.Poll(ctx, "acme")
		if err != nil || out.Outcome != schema.PollOutcomePartial {
			t.Fatalf("%v %v", out, err)
		}
		if len(jobs.upserted) != 1 || jobs.closedCalls != 0 {
			t.Fatalf("upsert=%d close=%d", len(jobs.upserted), jobs.closedCalls)
		}
	})

	t.Run("EMPTY streak to INACTIVE", func(t *testing.T) {
		store := newFakeCompanyStore()
		jobs := newFakeJobs()
		newResolved(store, "acme")
		c := store.byKey["acme"]
		c.EmptyPollStreak = 4
		store.byKey["acme"] = c
		fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, result: schema.BoardResult{
			Outcome: schema.PollOutcomeEmpty,
		}}
		svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
		svc.now = func() time.Time { return fixed }
		out, err := svc.Poll(ctx, "acme")
		if err != nil || out.Outcome != schema.PollOutcomeEmpty {
			t.Fatalf("%v %v", out, err)
		}
		got := store.byKey["acme"]
		if got.EmptyPollStreak != 5 || got.PollState != schema.PollStateInactive {
			t.Fatalf("%+v", got)
		}
		if jobs.closedCalls != 0 {
			t.Fatal("EMPTY must not close")
		}
	})

	t.Run("DEAD_SLUG streak to PENDING keeps slug", func(t *testing.T) {
		store := newFakeCompanyStore()
		jobs := newFakeJobs()
		newResolved(store, "acme")
		c := store.byKey["acme"]
		c.PollFailCount = 2
		store.byKey["acme"] = c
		fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, result: schema.BoardResult{
			Outcome: schema.PollOutcomeDeadSlug,
		}}
		svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
		svc.now = func() time.Time { return fixed }
		out, err := svc.Poll(ctx, "acme")
		if err != nil || out.Outcome != schema.PollOutcomeDeadSlug {
			t.Fatalf("%v %v", out, err)
		}
		got := store.byKey["acme"]
		if got.PollFailCount != 3 {
			t.Fatalf("fail=%d", got.PollFailCount)
		}
		if got.ResolutionStatus != schema.ResolutionStatusPending {
			t.Fatalf("status=%s", got.ResolutionStatus)
		}
		if got.ATSSlug != "SlugCase" || got.ResolutionMethod != schema.ResolutionMethodApplyURL {
			t.Fatalf("history cleared: slug=%q method=%s", got.ATSSlug, got.ResolutionMethod)
		}
	})

	t.Run("ErrBoardFormat terminal and logged", func(t *testing.T) {
		store := newFakeCompanyStore()
		newResolved(store, "acme")
		var logBuf strings.Builder
		fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, err: companies.ErrBoardFormat}
		svc := New(store, nil, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
		svc.log = slog.New(slog.NewTextHandler(&logBuf, nil))
		_, err := svc.Poll(ctx, "acme")
		if !errors.Is(err, companies.ErrBoardFormat) {
			t.Fatalf("err=%v", err)
		}
		if !strings.Contains(logBuf.String(), "board response format mismatch") {
			t.Fatalf("log=%s", logBuf.String())
		}
		if !IsTerminalCompanyError(err) {
			t.Fatal("expected terminal")
		}
	})
}

// R1.3 — EMPTY board + open jobs linked only via company_key_ref (aggregator
// path) must emit the I7 discrepancy log. Hardcoding CountOpenByCompanyKeyRef
// to 0 would silence this; that is the R1 failure mode.
func TestPoll_emptyBoardLogsAggregatorFallback(t *testing.T) {
	ctx := context.Background()
	fixed := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	cfg := config.Companies{PollIntervalHours: 6, PollMaxPerRun: 50}

	store := newFakeCompanyStore()
	jobs := newFakeJobs()
	// Aggregator-sourced open jobs only — no board upserts this poll.
	jobs.openCount = 3
	store.byKey["acme"] = schema.Company{
		Key:              "acme",
		Name:             "acme",
		ATS:              schema.ATSGreenhouse,
		ATSSlug:          "acme",
		ResolutionStatus: schema.ResolutionStatusResolved,
		ResolutionMethod: schema.ResolutionMethodApplyURL,
		PollState:        schema.PollStateActive,
	}

	var logBuf strings.Builder
	fetcher := &fakeFetcher{ats: schema.ATSGreenhouse, result: schema.BoardResult{
		Outcome: schema.PollOutcomeEmpty,
	}}
	svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, cfg)
	svc.now = func() time.Time { return fixed }
	svc.log = slog.New(slog.NewTextHandler(&logBuf, nil))

	out, err := svc.Poll(ctx, "acme")
	if err != nil || out.Outcome != schema.PollOutcomeEmpty {
		t.Fatalf("%v %v", out, err)
	}
	if jobs.closedCalls != 0 {
		t.Fatal("EMPTY must not close")
	}
	if len(jobs.upserted) != 0 {
		t.Fatal("EMPTY must not upsert board jobs")
	}
	if jobs.countKey != "acme" {
		t.Fatalf("CountOpenByCompanyKeyRef key=%q", jobs.countKey)
	}
	log := logBuf.String()
	if !strings.Contains(log, "empty board while aggregator still has jobs") ||
		!strings.Contains(log, "board_jobs=0") ||
		!strings.Contains(log, "aggregator_jobs=3") {
		t.Fatalf("fallback log missing: %s", log)
	}
}

func TestParseATSURL(t *testing.T) {
	hit, ok := parseATSURL("https://boards.greenhouse.io/foo/jobs/1")
	if !ok || hit.ATS != schema.ATSGreenhouse || hit.Slug != "foo" || !hit.Pollable {
		t.Fatalf("%+v ok=%v", hit, ok)
	}
	hit, ok = parseATSURL("https://jobs.ashbyhq.com/Coder")
	if !ok || hit.Slug != "Coder" {
		t.Fatalf("ashby casing: %+v", hit)
	}
	hit, ok = parseATSURL("https://acme.workable.com/j/abc")
	if !ok || hit.Pollable || hit.ATS != schema.ATSWorkable {
		t.Fatalf("workable: %+v ok=%v", hit, ok)
	}
}

// Q1 — name-form then domain-form (and the reverse) collapse to one row + alias.
func TestUpsertFromAggregator_nameThenDomainMerge(t *testing.T) {
	ctx := context.Background()
	bl := NewBlocklistMatcher(schema.Blocklist{})

	tests := []struct {
		name      string
		first     schema.UpsertCompanyParams
		second    schema.UpsertCompanyParams
		wantKey   string
		wantAlias string
		wantWeb   string
	}{
		{
			name:      "name then domain",
			first:     schema.UpsertCompanyParams{Name: "YLD"},
			second:    schema.UpsertCompanyParams{Name: "YLD", Website: "https://www.yld.com"},
			wantKey:   "yld",
			wantAlias: "yld.com",
			wantWeb:   "https://www.yld.com",
		},
		{
			name:      "domain then name",
			first:     schema.UpsertCompanyParams{Name: "Foo Labs", Website: "https://foo.example"},
			second:    schema.UpsertCompanyParams{Name: "Foo Labs"},
			wantKey:   "foo.example",
			wantAlias: "foo labs",
			wantWeb:   "https://foo.example",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeCompanyStore()
			svc := New(store, nil, bl, nil, config.Companies{})

			r1, err := svc.UpsertFromAggregator(ctx, tc.first)
			if err != nil || !r1.Created || r1.Key != tc.wantKey {
				t.Fatalf("run1: %+v err=%v", r1, err)
			}
			r2, err := svc.UpsertFromAggregator(ctx, tc.second)
			if err != nil || !r2.Merged || r2.Key != tc.wantKey {
				t.Fatalf("run2: %+v err=%v", r2, err)
			}
			if len(store.byKey) != 1 {
				t.Fatalf("rows=%d want 1", len(store.byKey))
			}
			c := store.byKey[tc.wantKey]
			if c.Website != tc.wantWeb {
				t.Fatalf("website=%q want %q", c.Website, tc.wantWeb)
			}
			if tc.wantAlias != "" && !contains(c.Aliases, tc.wantAlias) {
				t.Fatalf("aliases=%v want %q", c.Aliases, tc.wantAlias)
			}
		})
	}
}

// Q2 — near-duplicate names stay separate; lemon.io blocked, lemon.markets not.
func TestUpsertFromAggregator_nearDuplicatesStaySeparate(t *testing.T) {
	ctx := context.Background()
	bl := NewBlocklistMatcher(schema.Blocklist{
		Exact: []string{"lemon.io"},
	})

	tests := []struct {
		name     string
		a, b     schema.UpsertCompanyParams
		wantKeys []string
		aBlocked bool
		bBlocked bool
	}{
		{
			name:     "cloudbees vs cloudbeds",
			a:        schema.UpsertCompanyParams{Name: "CloudBees"},
			b:        schema.UpsertCompanyParams{Name: "Cloudbeds"},
			wantKeys: []string{"cloudbees", "cloudbeds"},
		},
		{
			name:     "lemon.io blocked lemon.markets not",
			a:        schema.UpsertCompanyParams{Name: "lemon.io"},
			b:        schema.UpsertCompanyParams{Name: "lemon.markets"},
			aBlocked: true,
			wantKeys: []string{"lemon.markets"},
		},
		{
			name:     "insider vs insider one",
			a:        schema.UpsertCompanyParams{Name: "Insider"},
			b:        schema.UpsertCompanyParams{Name: "Insider One"},
			wantKeys: []string{"insider", "insider one"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeCompanyStore()
			svc := New(store, nil, bl, nil, config.Companies{})

			ra, err := svc.UpsertFromAggregator(ctx, tc.a)
			if err != nil {
				t.Fatal(err)
			}
			if tc.aBlocked {
				if !ra.Blocked || ra.Created || ra.Merged {
					t.Fatalf("a blocked result: %+v", ra)
				}
			} else if !ra.Created {
				t.Fatalf("a create: %+v", ra)
			}

			rb, err := svc.UpsertFromAggregator(ctx, tc.b)
			if err != nil {
				t.Fatal(err)
			}
			if tc.bBlocked {
				if !rb.Blocked || rb.Created || rb.Merged {
					t.Fatalf("b blocked result: %+v", rb)
				}
			} else if !rb.Created {
				t.Fatalf("b create: %+v", rb)
			}

			if len(store.byKey) != len(tc.wantKeys) {
				t.Fatalf("rows=%d want %d (%v)", len(store.byKey), len(tc.wantKeys), keysOf(store.byKey))
			}
			for _, k := range tc.wantKeys {
				if _, ok := store.byKey[k]; !ok {
					t.Fatalf("missing key %q; have %v", k, keysOf(store.byKey))
				}
			}
		})
	}
}

// Q4 — PARTIAL from an HTTP 200 board response upserts and closes nothing.
func TestPoll_partialHTTP200ClosesNothing(t *testing.T) {
	ctx := context.Background()
	fixed := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	body, err := os.ReadFile("../../collectors/ats/greenhouse/testdata/jobs_partial.json")
	if err != nil {
		t.Fatal(err)
	}
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("ETag", `"partial-etag"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	store := newFakeCompanyStore()
	store.byKey["storyblok"] = schema.Company{
		Key:              "storyblok",
		Name:             "Storyblok",
		ATS:              schema.ATSGreenhouse,
		ATSSlug:          "storyblok",
		ResolutionStatus: schema.ResolutionStatusResolved,
		PollState:        schema.PollStateActive,
	}
	jobs := newFakeJobs()
	fetcher := &greenhouse.Client{
		HTTPClient: srv.Client(),
		BaseURL:    srv.URL,
	}
	svc := New(store, jobs, nil, []companies.CompanyBoardFetcher{fetcher}, config.Companies{})
	svc.now = func() time.Time { return fixed }

	out, err := svc.Poll(ctx, "storyblok")
	if err != nil {
		t.Fatal(err)
	}
	if out.Outcome != schema.PollOutcomePartial {
		t.Fatalf("outcome=%s want PARTIAL", out.Outcome)
	}
	if hits == 0 {
		t.Fatal("expected at least one HTTP call")
	}
	if len(jobs.upserted) == 0 {
		t.Fatal("PARTIAL must upsert the jobs that arrived")
	}
	if jobs.closedCalls != 0 {
		t.Fatalf("PARTIAL must not close; closedCalls=%d (HTTP hits=%d all 200)", jobs.closedCalls, hits)
	}
}

func keysOf(m map[string]schema.Company) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// --- fakes -----------------------------------------------------------------

type fakeCompanyStore struct {
	byKey         map[string]schema.Company
	byWebsite     map[string]string
	byAlias       map[string]string
	lastDueBefore time.Time
	lastDueLimit  int
}

func newFakeCompanyStore() *fakeCompanyStore {
	return &fakeCompanyStore{
		byKey:     make(map[string]schema.Company),
		byWebsite: make(map[string]string),
		byAlias:   make(map[string]string),
	}
}

func (f *fakeCompanyStore) Upsert(ctx context.Context, c schema.Company) error {
	_ = ctx
	prev, ok := f.byKey[c.Key]
	if ok {
		c.FirstSeenAt = prev.FirstSeenAt
	}
	f.byKey[c.Key] = c
	if c.Website != "" {
		f.byWebsite[c.Website] = c.Key
		if d := NormalizeDomain(c.Website); d != "" {
			f.byWebsite[d] = c.Key
		}
	}
	for _, a := range c.Aliases {
		f.byAlias[a] = c.Key
	}
	return nil
}

func (f *fakeCompanyStore) GetByKey(ctx context.Context, key string) (schema.Company, error) {
	_ = ctx
	c, ok := f.byKey[key]
	if !ok {
		return schema.Company{}, companies.ErrCompanyNotFound
	}
	return c, nil
}

func (f *fakeCompanyStore) FindByWebsite(ctx context.Context, website string) (schema.Company, error) {
	_ = ctx
	key, ok := f.byWebsite[website]
	if !ok {
		return schema.Company{}, companies.ErrCompanyNotFound
	}
	return f.GetByKey(ctx, key)
}

func (f *fakeCompanyStore) FindByAlias(ctx context.Context, alias string) (schema.Company, error) {
	_ = ctx
	key, ok := f.byAlias[alias]
	if !ok {
		return schema.Company{}, companies.ErrCompanyNotFound
	}
	return f.GetByKey(ctx, key)
}

func (f *fakeCompanyStore) List(ctx context.Context, filter schema.ListFilter) ([]schema.Company, error) {
	_ = ctx
	_ = filter
	out := make([]schema.Company, 0, len(f.byKey))
	for _, c := range f.byKey {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeCompanyStore) Count(ctx context.Context, filter schema.ListFilter) (int, error) {
	items, err := f.List(ctx, filter)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func (f *fakeCompanyStore) ListPendingResolve(ctx context.Context, now time.Time, limit int) ([]schema.Company, error) {
	_ = ctx
	_ = now
	_ = limit
	return nil, nil
}

func (f *fakeCompanyStore) ListDuePollable(ctx context.Context, dueBefore time.Time, limit int) ([]schema.Company, error) {
	_ = ctx
	f.lastDueBefore = dueBefore
	f.lastDueLimit = limit
	return nil, nil
}

func (f *fakeCompanyStore) MarkPolled(ctx context.Context, p schema.MarkPolledParams) error {
	_ = ctx
	c, ok := f.byKey[p.Key]
	if !ok {
		return companies.ErrCompanyNotFound
	}
	at := p.LastPolledAt
	c.LastPolledAt = &at
	c.BoardETag = p.BoardETag
	c.PollFailCount = p.PollFailCount
	c.EmptyPollStreak = p.EmptyPollStreak
	if p.PollState != "" {
		c.PollState = p.PollState
	}
	f.byKey[p.Key] = c
	return nil
}

func (f *fakeCompanyStore) MarkResolution(ctx context.Context, p schema.MarkResolutionParams) error {
	_ = ctx
	c, ok := f.byKey[p.Key]
	if !ok {
		return companies.ErrCompanyNotFound
	}
	c.ResolutionStatus = p.ResolutionStatus
	c.ResolutionMethod = p.ResolutionMethod
	c.ATS = p.ATS
	c.ATSSlug = p.ATSSlug
	c.BoardURL = p.BoardURL
	c.NextResolveAt = p.NextResolveAt
	f.byKey[p.Key] = c
	return nil
}

type fakeJobs struct {
	upserted    []jobdata.Job
	closedCalls int
	reopenCalls int
	openCount   int64  // CountOpenByCompanyKeyRef result (aggregator-linked open jobs)
	countKey    string // last key passed to CountOpenByCompanyKeyRef
}

func newFakeJobs() *fakeJobs { return &fakeJobs{} }

func (f *fakeJobs) UpsertBoardJobs(ctx context.Context, companyKey string, jobs []jobdata.Job) error {
	_ = ctx
	_ = companyKey
	f.upserted = append(f.upserted, jobs...)
	return nil
}

func (f *fakeJobs) CloseAbsent(ctx context.Context, companyKey string, seenIDs []string) (int64, error) {
	_ = ctx
	_ = companyKey
	_ = seenIDs
	f.closedCalls++
	return 0, nil
}

func (f *fakeJobs) ReopenPresent(ctx context.Context, companyKey string, seenIDs []string) (int64, error) {
	_ = ctx
	_ = companyKey
	_ = seenIDs
	f.reopenCalls++
	return 0, nil
}

func (f *fakeJobs) BackfillCompanyKeyRef(ctx context.Context, companyKey string, contentKeys []string) (int64, error) {
	_ = ctx
	_ = companyKey
	_ = contentKeys
	return 0, nil
}

func (f *fakeJobs) CountOpenByCompanyKeyRef(ctx context.Context, companyKey string) (int64, error) {
	_ = ctx
	f.countKey = companyKey
	return f.openCount, nil
}

type fakeFetcher struct {
	ats      schema.ATS
	result   schema.BoardResult
	err      error
	lastSlug string
	lastETag string
}

func (f *fakeFetcher) ATS() schema.ATS { return f.ats }

func (f *fakeFetcher) FetchBoard(ctx context.Context, slug, companyName, etag string) (schema.BoardResult, error) {
	_ = ctx
	_ = companyName
	f.lastSlug = slug
	f.lastETag = etag
	if f.err != nil {
		return schema.BoardResult{}, f.err
	}
	return f.result, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func pendingWithBoard(key, boardURL string) schema.Company {
	return schema.Company{
		Key:              key,
		Name:             key,
		BoardURL:         boardURL,
		ResolutionStatus: schema.ResolutionStatusPending,
		PollState:        schema.PollStateActive,
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
