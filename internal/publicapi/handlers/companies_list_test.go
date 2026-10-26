package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	companyschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/rs/zerolog"
)

func TestGetCompanies(t *testing.T) {
	polled := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)

	t.Run("bad query", func(t *testing.T) {
		for _, rawURL := range []string{
			"/api/v1/companies?resolution_status=DONE",
			"/api/v1/companies?poll_state=PAUSED",
			"/api/v1/companies?ats=UNKNOWN",
			"/api/v1/companies?needs_review=yes",
			"/api/v1/companies?page=0",
			"/api/v1/companies?limit=101",
			"/api/v1/companies?page=no",
			"/api/v1/companies?ats=GREENHOUSE&ats=LEVER",
		} {
			t.Run(rawURL, func(t *testing.T) {
				api := &companiesListMock{}
				h := NewHTTPHandler(nil, Deps{
					Logger:    zerolog.Nop(),
					Companies: api,
				})
				req := httptest.NewRequest(http.MethodGet, rawURL, nil)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status %d %s", rec.Code, rec.Body.String())
				}
				var body schema.APIErrorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				assertAPIError(t, body.Error, schema.APIErrorCodeInvalidQuery)
				if api.listCalled || api.countCalled {
					t.Fatal("List/Count called")
				}
			})
		}
	})

	t.Run("filters and defaults", func(t *testing.T) {
		needsReview := true
		api := &companiesListMock{
			items: []companyschema.Company{{
				Key:              "storyblok",
				Name:             "Storyblok",
				Website:          "https://www.storyblok.com",
				ATS:              companyschema.ATSGreenhouse,
				ATSSlug:          "storyblok",
				BoardURL:         "https://boards.greenhouse.io/storyblok",
				ResolutionStatus: companyschema.ResolutionStatusResolved,
				ResolutionMethod: companyschema.ResolutionMethodSeed,
				PollState:        companyschema.PollStateActive,
				LastPolledAt:     &polled,
				PollFailCount:    0,
				NeedsReview:      true,
			}},
			total: 3,
		}
		h := NewHTTPHandler(nil, Deps{
			Logger:    zerolog.Nop(),
			Companies: api,
		})
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/companies?resolution_status=RESOLVED&poll_state=ACTIVE&ats=GREENHOUSE&needs_review=true&page=2&limit=25",
			nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listFilter.ResolutionStatus != companyschema.ResolutionStatusResolved ||
			api.listFilter.PollState != companyschema.PollStateActive ||
			api.listFilter.ATS != companyschema.ATSGreenhouse ||
			api.listFilter.NeedsReview == nil || *api.listFilter.NeedsReview != needsReview ||
			api.listFilter.Page != 2 || api.listFilter.Limit != 25 {
			t.Fatalf("filter %#v", api.listFilter)
		}
		var got schema.CompanyListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Page != 2 || got.Limit != 25 || got.Total != 3 || len(got.Items) != 1 {
			t.Fatalf("page %#v", got)
		}
		item := got.Items[0]
		if item.Key != "storyblok" || item.ATS != "GREENHOUSE" || item.ResolutionStatus != "RESOLVED" ||
			item.PollState != "ACTIVE" || item.NeedsReview != true || item.BoardURL == "" {
			t.Fatalf("item %#v", item)
		}
		if item.LastPolledAt == nil || !item.LastPolledAt.Equal(polled) {
			t.Fatalf("last_polled_at %v", item.LastPolledAt)
		}
		if strings.Contains(rec.Body.String(), "board_etag") {
			t.Fatalf("board_etag on wire: %s", rec.Body.String())
		}
	})

	t.Run("default page limit", func(t *testing.T) {
		api := &companiesListMock{}
		h := NewHTTPHandler(nil, Deps{
			Logger:    zerolog.Nop(),
			Companies: api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listFilter.Page != 1 || api.listFilter.Limit != schema.DefaultCompanyListLimit {
			t.Fatalf("filter %#v", api.listFilter)
		}
		if api.listFilter.ResolutionStatus != "" || api.listFilter.PollState != "" ||
			api.listFilter.ATS != "" || api.listFilter.NeedsReview != nil {
			t.Fatalf("unexpected filters %#v", api.listFilter)
		}
	})

	t.Run("unexpected does not leak cause", func(t *testing.T) {
		api := &companiesListMock{listErr: errBoom("secret db detail")}
		h := NewHTTPHandler(nil, Deps{
			Logger:    zerolog.Nop(),
			Companies: api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var body schema.APIErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, body.Error, schema.APIErrorCodeUnexpected)
		if strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("cause leaked: %s", rec.Body.String())
		}
	})
}

type errBoom string

func (e errBoom) Error() string { return string(e) }

type companiesListMock struct {
	items       []companyschema.Company
	total       int
	listCalled  bool
	countCalled bool
	listFilter  companyschema.ListFilter
	listErr     error
	countErr    error
}

func (m *companiesListMock) UpsertFromAggregator(context.Context, companyschema.UpsertCompanyParams) (companyschema.UpsertCompanyResult, error) {
	return companyschema.UpsertCompanyResult{}, nil
}

func (m *companiesListMock) Resolve(context.Context, string) (companyschema.Company, error) {
	return companyschema.Company{}, nil
}

func (m *companiesListMock) DuePollable(context.Context, time.Time, int) ([]companyschema.Company, error) {
	return nil, nil
}

func (m *companiesListMock) Poll(context.Context, string) (companyschema.PollResult, error) {
	return companyschema.PollResult{}, nil
}

func (m *companiesListMock) ListPendingResolve(context.Context, time.Time, int) ([]companyschema.Company, error) {
	return nil, nil
}

func (m *companiesListMock) RecordPollTransportFailure(context.Context, string) error {
	return nil
}

func (m *companiesListMock) List(_ context.Context, f companyschema.ListFilter) ([]companyschema.Company, error) {
	m.listCalled = true
	m.listFilter = f
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.items, nil
}

func (m *companiesListMock) Count(_ context.Context, f companyschema.ListFilter) (int, error) {
	m.countCalled = true
	m.listFilter = f
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.total, nil
}
