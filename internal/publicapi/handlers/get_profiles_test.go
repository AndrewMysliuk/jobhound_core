package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/rs/zerolog"
)

func TestGetProfilesReturnsListInStoreOrder(t *testing.T) {
	h := NewHTTPHandler(nil, Deps{
		Logger: zerolog.Nop(),
		Profiles: fakeProfileStore{list: []profileschema.Profile{
			{
				ID:           "andrew",
				Name:         "Andrew",
				Domain:       profileschema.DomainSoftware,
				Sources:      []string{"himalayas", "wellfound"},
				Queries:      []string{"golang"},
				ExcludeTitle: []string{"intern"},
				ExcludeText:  []string{"clearance"},
				Penalties:    []string{"php"},
			},
			{
				ID:     "zeta",
				Name:   "Zeta",
				Domain: profileschema.DomainArchitecture,
			},
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, hidden := range []string{"exclude_title", "exclude_text", "penalties", "wellfound_roles"} {
		if strings.Contains(body, hidden) {
			t.Fatalf("response includes %s: %s", hidden, body)
		}
	}

	var got schema.ProfileListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Profiles) != 2 {
		t.Fatalf("profiles %#v", got.Profiles)
	}
	if got.Profiles[0].ID != "andrew" || got.Profiles[0].Name != "Andrew" || got.Profiles[0].Domain != "software" {
		t.Fatalf("first %#v", got.Profiles[0])
	}
	if len(got.Profiles[0].Sources) != 2 || got.Profiles[0].Sources[0] != "himalayas" || got.Profiles[0].Sources[1] != "wellfound" {
		t.Fatalf("sources %#v", got.Profiles[0].Sources)
	}
	if len(got.Profiles[0].Queries) != 1 || got.Profiles[0].Queries[0] != "golang" {
		t.Fatalf("queries %#v", got.Profiles[0].Queries)
	}
	if got.Profiles[1].ID != "zeta" || got.Profiles[1].Domain != "architecture" {
		t.Fatalf("second %#v", got.Profiles[1])
	}
	if got.Profiles[1].Sources == nil || len(got.Profiles[1].Sources) != 0 || got.Profiles[1].Queries == nil || len(got.Profiles[1].Queries) != 0 {
		t.Fatalf("empty lists %#v", got.Profiles[1])
	}
}

type fakeProfileStore struct {
	list []profileschema.Profile
	err  error
}

func (f fakeProfileStore) List(context.Context) ([]profileschema.Profile, error) {
	return f.list, f.err
}

func (f fakeProfileStore) Get(context.Context, string) (profileschema.Profile, error) {
	return profileschema.Profile{}, profiles.ErrProfileNotFound
}
