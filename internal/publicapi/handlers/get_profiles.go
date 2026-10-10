package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) getProfiles(w http.ResponseWriter, r *http.Request) {
	list, err := h.deps.Profiles.List(r.Context())
	if err != nil {
		if errors.Is(err, profiles.ErrProfileInvalid) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileInvalidDefinition, Cause: err})
			return
		}
		apputils.WriteError(w, err)
		return
	}
	out := schema.ProfileListResponse{Profiles: make([]schema.ProfileListItem, 0, len(list))}
	for _, p := range list {
		sources := p.Sources
		if sources == nil {
			sources = []string{}
		}
		queries := p.Queries
		if queries == nil {
			queries = []string{}
		}
		out.Profiles = append(out.Profiles, schema.ProfileListItem{
			ID:      p.ID,
			Name:    p.Name,
			Domain:  p.Domain.String(),
			Sources: sources,
			Queries: queries,
		})
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
