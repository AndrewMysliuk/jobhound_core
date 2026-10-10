package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

func (h *HTTPHandler) getProfileJobs(w http.ResponseWriter, r *http.Request) {
	q, ok := h.parseProfileJobsQuery(r)
	if !ok {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidQuery})
		return
	}
	profileID := apputils.StringsTrimPathValue(r, "profile_id")
	if profileID == "" {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileNotFound})
		return
	}
	if _, err := h.deps.Profiles.Get(r.Context(), profileID); err != nil {
		if errors.Is(err, profiles.ErrProfileNotFound) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileNotFound, Cause: err})
			return
		}
		if errors.Is(err, profiles.ErrProfileInvalid) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileInvalidDefinition, Cause: err})
			return
		}
		apputils.WriteError(w, err)
		return
	}
	page, err := h.deps.Scoring.ListJobs(r.Context(), scoringschema.ListJobsParams{
		ProfileID:  profileID,
		Bucket:     q.bucket,
		UserStatus: q.userStatus,
		Page:       q.page,
		Limit:      q.limit,
	})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, h.profileJobPage(page))
}

type profileJobsQuery struct {
	bucket     scoringschema.Bucket
	userStatus scoringschema.UserStatus
	page       int
	limit      int
}

func (h *HTTPHandler) parseProfileJobsQuery(r *http.Request) (profileJobsQuery, bool) {
	values := r.URL.Query()
	out := profileJobsQuery{
		bucket:     scoringschema.BucketPassed,
		userStatus: scoringschema.UserStatusNew,
		page:       1,
		limit:      schema.DefaultJobListLimit,
	}
	bucket, ok := h.optionalQuery(values["bucket"])
	if !ok {
		return profileJobsQuery{}, false
	}
	if bucket != "" {
		parsed, err := scoringschema.FromStringBucket(bucket)
		if err != nil {
			return profileJobsQuery{}, false
		}
		out.bucket = parsed
	}
	status, ok := h.optionalQuery(values["user_status"])
	if !ok {
		return profileJobsQuery{}, false
	}
	if status != "" {
		parsed, err := scoringschema.FromStringUserStatus(status)
		if err != nil {
			return profileJobsQuery{}, false
		}
		out.userStatus = parsed
	}
	page, ok := h.boundedQuery(values["page"], 1, 0)
	if !ok {
		return profileJobsQuery{}, false
	}
	out.page = page
	limit, ok := h.boundedQuery(values["limit"], schema.DefaultJobListLimit, schema.MaxJobListLimit)
	if !ok {
		return profileJobsQuery{}, false
	}
	out.limit = limit
	return out, true
}

func (h *HTTPHandler) optionalQuery(vs []string) (string, bool) {
	if len(vs) == 0 {
		return "", true
	}
	if len(vs) != 1 {
		return "", false
	}
	return strings.TrimSpace(vs[0]), true
}

func (h *HTTPHandler) boundedQuery(vs []string, fallback, max int) (int, bool) {
	raw, ok := h.optionalQuery(vs)
	if !ok {
		return 0, false
	}
	if raw == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || (max > 0 && n > max) {
		return 0, false
	}
	return n, true
}

func (h *HTTPHandler) profileJobPage(page scoringschema.JobPage) schema.ProfileJobPage {
	jobs := make([]schema.ProfileJob, 0, len(page.Jobs))
	for _, job := range page.Jobs {
		sources := job.Sources
		if sources == nil {
			sources = []string{}
		}
		regions := job.Location.Regions
		if regions == nil {
			regions = []string{}
		}
		countries := job.Location.Countries
		if countries == nil {
			countries = []string{}
		}
		signals := make([]schema.ProfileJobSignal, 0, len(job.Signals))
		for _, signal := range job.Signals {
			signals = append(signals, schema.ProfileJobSignal{
				Code:   signal.Code.String(),
				Points: signal.Points,
				Terms:  signal.Terms,
			})
		}
		var postedAt *time.Time
		if !job.PostedAt.IsZero() {
			posted := job.PostedAt
			postedAt = &posted
		}
		jobs = append(jobs, schema.ProfileJob{
			JobID:    job.JobID,
			Title:    job.Title,
			Company:  job.Company,
			URL:      job.URL,
			ApplyURL: job.ApplyURL,
			Location: schema.ProfileJobLocation{
				Type:      job.Location.Type,
				Regions:   regions,
				Countries: countries,
				Timezone:  job.Location.Timezone,
				Raw:       job.Location.Raw,
			},
			Sources:     sources,
			PostedAt:    postedAt,
			FirstSeenAt: job.FirstSeenAt,
			LastSeenAt:  job.LastSeenAt,
			Bucket:      job.Bucket.String(),
			Score:       job.Score,
			Signals:     signals,
			UserStatus:  job.UserStatus.String(),
		})
	}
	return schema.ProfileJobPage{
		Page:  page.Page,
		Limit: page.Limit,
		Total: page.Total,
		Jobs:  jobs,
	}
}
