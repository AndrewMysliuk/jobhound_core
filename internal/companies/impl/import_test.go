package impl

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/xuri/excelize/v2"
)

func TestClassifySeedATS(t *testing.T) {
	cases := []struct {
		raw   string
		ats   schema.ATS
		class seedATSClass
	}{
		{"Greenhouse", schema.ATSGreenhouse, seedATSPollable},
		{"Greenhouse (встроен в сайт)", schema.ATSGreenhouse, seedATSPollable},
		{"Ashby", schema.ATSAshby, seedATSPollable},
		{"Lever", schema.ATSLever, seedATSPollable},
		{"Teamtailor", schema.ATSTeamtailor, seedATSUnsupported},
		{"Kula", schema.ATSOther, seedATSUnsupported},
		{"Custom", "", seedATSNone},
		{"Custom (на сайте)", "", seedATSNone},
		{"UNKNOWN", "", seedATSNone},
		{"NONE", "", seedATSNone},
		{"Notion (без ATS)", "", seedATSNone},
		{"", "", seedATSNone},
	}
	for _, tc := range cases {
		ats, class := classifySeedATS(tc.raw)
		if ats != tc.ats || class != tc.class {
			t.Fatalf("%q: got (%q,%v) want (%q,%v)", tc.raw, ats, class, tc.ats, tc.class)
		}
	}
}

func TestImportFromSeed_fixtureSheet(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "seed_fixture.xlsx")
	if err := writeSeedFixture(path); err != nil {
		t.Fatal(err)
	}

	store := newFakeCompanyStore()
	bl := NewBlocklistMatcher(schema.Blocklist{Exact: []string{"lemon.io"}})
	fetchers := []companies.CompanyBoardFetcher{
		&mapFetcher{ats: schema.ATSGreenhouse, bySlug: map[string]schema.BoardResult{
			"good-gh": {Outcome: schema.PollOutcomeComplete},
		}},
		&mapFetcher{ats: schema.ATSAshby, bySlug: map[string]schema.BoardResult{
			"empty-ashby": {Outcome: schema.PollOutcomeEmpty},
		}},
		&mapFetcher{ats: schema.ATSLever, bySlug: map[string]schema.BoardResult{
			"dead-lever": {Outcome: schema.PollOutcomeDeadSlug},
		}},
	}
	svc := New(store, nil, bl, fetchers, config.Companies{})

	stats, err := svc.ImportFromSeed(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Skipped != 2 { // example + Fit=SKIP
		t.Fatalf("skipped=%d want 2", stats.Skipped)
	}
	if stats.Blocked != 1 {
		t.Fatalf("blocked=%d want 1", stats.Blocked)
	}
	if stats.Resolved != 2 { // greenhouse 200 + ashby empty 200
		t.Fatalf("resolved=%d want 2", stats.Resolved)
	}
	if stats.Pending != 3 { // lever 404 + custom + unknown
		t.Fatalf("pending=%d want 3", stats.Pending)
	}
	if stats.Unsupported != 1 { // teamtailor
		t.Fatalf("unsupported=%d want 1", stats.Unsupported)
	}
	if stats.Created != 6 { // resolved2 + pending3 + unsupported1 (blocked/skipped not created)
		t.Fatalf("created=%d want 6", stats.Created)
	}

	gh := store.byKey["goodgh.com"]
	if gh.ResolutionStatus != schema.ResolutionStatusResolved || gh.ATSSlug != "good-gh" || gh.ATS != schema.ATSGreenhouse {
		t.Fatalf("greenhouse row: %+v", gh)
	}
	if gh.ResolutionMethod != schema.ResolutionMethodSeed {
		t.Fatalf("method=%q", gh.ResolutionMethod)
	}

	empty := store.byKey["emptyashby.com"]
	if empty.ResolutionStatus != schema.ResolutionStatusResolved || empty.ATSSlug != "empty-ashby" {
		t.Fatalf("empty 200 must stay RESOLVED: %+v", empty)
	}

	dead := store.byKey["deadlever.com"]
	if dead.ResolutionStatus != schema.ResolutionStatusPending || dead.ATSSlug != "" || dead.ATS != "" {
		t.Fatalf("404 must discard slug: %+v", dead)
	}

	tt := store.byKey["teamtailor.co"]
	if tt.ResolutionStatus != schema.ResolutionStatusUnsupportedATS || tt.ATS != schema.ATSTeamtailor || tt.ATSSlug != "" {
		t.Fatalf("unsupported: %+v", tt)
	}

	custom := store.byKey["custom.co"]
	if custom.ResolutionStatus != schema.ResolutionStatusPending || custom.ATS != "" {
		t.Fatalf("custom: %+v", custom)
	}

	if _, ok := store.byKey["lemon.io"]; ok {
		t.Fatal("blocklisted lemon.io must not be stored")
	}
	if _, ok := store.byKey[NormalizeName("Acme Fintech (ПРИМЕР)")]; ok {
		t.Fatal("example row must not be stored")
	}

	// P3 — re-run merges, no duplicates.
	stats2, err := svc.ImportFromSeed(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if stats2.Created != 0 || stats2.Merged != 6 {
		t.Fatalf("rerun created=%d merged=%d want 0/6", stats2.Created, stats2.Merged)
	}
	if len(store.byKey) != 6 {
		t.Fatalf("rows after rerun=%d want 6", len(store.byKey))
	}
}

func TestReadSeedXLSX_realTemplateHeaders(t *testing.T) {
	rows, err := ReadSeedXLSX("../../../config/companies/companies_template.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 60 {
		t.Fatalf("rows=%d, expected the 64+ seed companies", len(rows))
	}
	foundExample := false
	for _, r := range rows {
		if isSeedExampleRow(r.Name) {
			foundExample = true
			break
		}
	}
	if !foundExample {
		t.Fatal("expected example row in real template")
	}
}

type mapFetcher struct {
	ats    schema.ATS
	bySlug map[string]schema.BoardResult
}

func (f *mapFetcher) ATS() schema.ATS { return f.ats }

func (f *mapFetcher) FetchBoard(ctx context.Context, slug, companyName, etag string) (schema.BoardResult, error) {
	_ = ctx
	_ = companyName
	_ = etag
	if r, ok := f.bySlug[slug]; ok {
		return r, nil
	}
	return schema.BoardResult{Outcome: schema.PollOutcomeDeadSlug}, nil
}

func writeSeedFixture(path string) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := seedSheetName
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	headers := []string{colCompany, colWebsite, colATS, colSlug, colFit, colVacanciesURL}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
	}
	data := [][]string{
		{"Acme Fintech (ПРИМЕР)", "https://acmefintech.com", "Greenhouse", "acmefintech", "FIT", ""},
		{"Skip Corp", "https://skip.example", "Ashby", "skipcorp", "SKIP", ""},
		{"Good GH", "https://goodgh.com", "Greenhouse", "good-gh", "FIT", "https://boards.greenhouse.io/good-gh"},
		{"Empty Ashby", "https://emptyashby.com", "Ashby", "empty-ashby", "FIT", ""},
		{"Dead Lever", "https://deadlever.com", "Lever", "dead-lever", "FIT", ""},
		{"TT Co", "https://teamtailor.co", "Teamtailor", "tt-slug", "FIT", ""},
		{"Custom Co", "https://custom.co", "Custom", "", "FIT", "https://custom.co/careers"},
		{"Unknown Co", "https://unknown.co", "UNKNOWN", "", "FIT", ""},
		{"lemon.io", "https://lemon.io", "Ashby", "lemonio", "FIT", ""},
	}
	for r, row := range data {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, val); err != nil {
				return err
			}
		}
	}
	return f.SaveAs(path)
}
