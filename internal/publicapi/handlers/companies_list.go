package handlers

import (
	"net/http"
	"strconv"

	companyschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) getCompanies(w http.ResponseWriter, r *http.Request) {
	f, ok := h.parseCompaniesQuery(r)
	if !ok {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidQuery})
		return
	}
	if h.deps.Companies == nil {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeUnexpected})
		return
	}
	items, err := h.deps.Companies.List(r.Context(), f)
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	total, err := h.deps.Companies.Count(r.Context(), f)
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	out := schema.CompanyListResponse{
		Items: make([]schema.CompanyListItem, 0, len(items)),
		Page:  f.Page,
		Limit: f.Limit,
		Total: total,
	}
	for _, c := range items {
		out.Items = append(out.Items, schema.CompanyListItem{
			Key:              c.Key,
			Name:             c.Name,
			Website:          c.Website,
			ATS:              c.ATS.String(),
			ATSSlug:          c.ATSSlug,
			BoardURL:         c.BoardURL,
			ResolutionStatus: c.ResolutionStatus.String(),
			ResolutionMethod: c.ResolutionMethod.String(),
			PollState:        c.PollState.String(),
			LastPolledAt:     c.LastPolledAt,
			PollFailCount:    c.PollFailCount,
			NeedsReview:      c.NeedsReview,
		})
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}

func (h *HTTPHandler) parseCompaniesQuery(r *http.Request) (companyschema.ListFilter, bool) {
	values := r.URL.Query()
	out := companyschema.ListFilter{
		Page:  1,
		Limit: schema.DefaultCompanyListLimit,
	}

	rawStatus, ok := h.optionalQuery(values["resolution_status"])
	if !ok {
		return companyschema.ListFilter{}, false
	}
	if rawStatus != "" {
		parsed, err := companyschema.FromStringResolutionStatus(rawStatus)
		if err != nil {
			return companyschema.ListFilter{}, false
		}
		out.ResolutionStatus = parsed
	}

	rawPoll, ok := h.optionalQuery(values["poll_state"])
	if !ok {
		return companyschema.ListFilter{}, false
	}
	if rawPoll != "" {
		parsed, err := companyschema.FromStringPollState(rawPoll)
		if err != nil {
			return companyschema.ListFilter{}, false
		}
		out.PollState = parsed
	}

	rawATS, ok := h.optionalQuery(values["ats"])
	if !ok {
		return companyschema.ListFilter{}, false
	}
	if rawATS != "" {
		parsed, err := companyschema.FromStringATS(rawATS)
		if err != nil {
			return companyschema.ListFilter{}, false
		}
		out.ATS = parsed
	}

	rawReview, ok := h.optionalQuery(values["needs_review"])
	if !ok {
		return companyschema.ListFilter{}, false
	}
	if rawReview != "" {
		v, err := strconv.ParseBool(rawReview)
		if err != nil {
			return companyschema.ListFilter{}, false
		}
		out.NeedsReview = &v
	}

	page, ok := h.boundedQuery(values["page"], 1, 0)
	if !ok {
		return companyschema.ListFilter{}, false
	}
	out.Page = page
	limit, ok := h.boundedQuery(values["limit"], schema.DefaultCompanyListLimit, schema.MaxCompanyListLimit)
	if !ok {
		return companyschema.ListFilter{}, false
	}
	out.Limit = limit
	return out, true
}
