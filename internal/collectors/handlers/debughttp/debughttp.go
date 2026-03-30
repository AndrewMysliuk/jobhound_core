// Package debughttp serves a local-only HTTP surface to exercise pipeline.Collector without the public API (specs/005-job-collectors/spec.md).
package debughttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

const (
	defaultDebugLimit = 200
	maxDebugLimit     = 10000
	maxDebugBodyBytes = 512 << 10
)

// PathEuropeRemotely is the registered method+path for the Europe Remotely debug fetch.
const PathEuropeRemotely = "POST /debug/collectors/europe_remotely"

// PathWorkingNomads is the registered method+path for the Working Nomads debug fetch.
const PathWorkingNomads = "POST /debug/collectors/working_nomads"

// NewMux registers GET /health and per-source POST routes (see Path* constants).
// europeRemotely and workingNomadsIface must not be nil.
// workingNomadsConcrete may be nil (tests); when non-nil, POST /debug/collectors/working_nomads
// can override ES query/sort/page_size/_source per request without mutating the shared bootstrap instance.
// europeRemotelyConcrete may be nil (tests); when non-nil, POST /debug/collectors/europe_remotely
// uses MaxJobs=limit (early stop) and optional feed_form / search_keywords without mutating bootstrap.
func NewMux(europeRemotely, workingNomadsIface pipeline.Collector, workingNomadsConcrete *workingnomads.WorkingNomads, europeRemotelyConcrete *europeremotely.EuropeRemotely) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc(PathEuropeRemotely, func(w http.ResponseWriter, r *http.Request) {
		runCollectorDebug(w, r, europeRemotely, nil, europeRemotelyConcrete)
	})
	mux.HandleFunc(PathWorkingNomads, func(w http.ResponseWriter, r *http.Request) {
		runCollectorDebug(w, r, workingNomadsIface, workingNomadsConcrete, nil)
	})
	return mux
}

// debugCollectorsRequest is the single JSON contract for POST /debug/collectors/* (optional body).
// Omitted keys use defaults; Working Nomads–specific fields are ignored on the Europe Remotely route;
// Europe-specific feed fields are ignored on working_nomads.
type debugCollectorsRequest struct {
	Limit *int `json:"limit"`
	// Working Nomads → jobsapi/_search (ignored on europe_remotely).
	Query            *json.RawMessage `json:"query"`
	Sort             *json.RawMessage `json:"sort"`
	PageSize         *int             `json:"page_size"`
	SourceFieldNames []string         `json:"_source"`
	// Europe Remotely → merged into POST admin-ajax form after bootstrap fields (ignored on working_nomads).
	// Use for filters captured from DevTools (e.g. date range) when the site sends them as form keys.
	FeedForm map[string]string `json:"feed_form,omitempty"`
	// Europe Remotely: sets form field search_keywords (applied after feed_form; overrides the same key).
	SearchKeywords *string `json:"search_keywords,omitempty"`
}

func runCollectorDebug(w http.ResponseWriter, r *http.Request, coll pipeline.Collector, wnConcrete *workingnomads.WorkingNomads, erConcrete *europeremotely.EuropeRemotely) {
	if coll == nil {
		http.Error(w, "collector not configured", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	raw, err := readDebugRequestBody(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := parseDebugCollectorsRequest(raw)
	if err != nil {
		http.Error(w, fmt.Sprintf("JSON body: %v", err), http.StatusBadRequest)
		return
	}
	limit, err := resolveLimit(req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var jobs []domain.Job
	var fetchErr error
	var upstreamFetched int

	isWN := coll.Name() == workingnomads.SourceName && wnConcrete != nil
	isER := coll.Name() == europeremotely.SourceName && erConcrete != nil
	switch {
	case isWN:
		c := *wnConcrete
		applyWorkingNomadsOverrides(&req, &c)
		if limit > 0 {
			c.MaxFetchJobs = limit
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	case isER:
		c := *erConcrete
		applyEuropeRemotelyOverrides(&req, &c)
		if limit > 0 {
			c.MaxJobs = limit
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	default:
		jobs, fetchErr = coll.Fetch(ctx)
		upstreamFetched = len(jobs)
		if limit > 0 && len(jobs) > limit {
			jobs = jobs[:limit]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if fetchErr != nil {
		_ = json.NewEncoder(w).Encode(runCollectorResponse{
			OK:        false,
			Collector: coll.Name(),
			Error:     fetchErr.Error(),
		})
		return
	}

	resp := runCollectorResponse{
		OK:        true,
		Collector: coll.Name(),
		Count:     len(jobs),
	}
	if !isWN && !isER && upstreamFetched > len(jobs) {
		resp.UpstreamFetched = upstreamFetched
	}

	resp.Jobs = make([]jobDebugJSON, 0, len(jobs))
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, jobToDebugJSON(j))
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func readDebugRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxDebugBodyBytes)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return b, nil
}

func parseDebugCollectorsRequest(b []byte) (debugCollectorsRequest, error) {
	var req debugCollectorsRequest
	if len(bytesTrimSpace(b)) == 0 {
		return req, nil
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return debugCollectorsRequest{}, err
	}
	return req, nil
}

func resolveLimit(p *int) (int, error) {
	if p == nil {
		return defaultDebugLimit, nil
	}
	n := *p
	if n < 0 {
		return 0, fmt.Errorf("invalid limit (non-negative integer or 0 for unlimited)")
	}
	if n == 0 {
		return 0, nil
	}
	if n > maxDebugLimit {
		return 0, fmt.Errorf("limit exceeds max (%d)", maxDebugLimit)
	}
	return n, nil
}

func applyEuropeRemotelyOverrides(req *debugCollectorsRequest, c *europeremotely.EuropeRemotely) {
	c.FeedForm = europeremotely.CloneFeedForm(c.FeedForm)
	for k, v := range req.FeedForm {
		c.FeedForm.Set(k, v)
	}
	if req.SearchKeywords != nil {
		c.FeedForm.Set("search_keywords", *req.SearchKeywords)
	}
}

func applyWorkingNomadsOverrides(req *debugCollectorsRequest, c *workingnomads.WorkingNomads) {
	if req.Query != nil {
		c.Query = cloneRawMessage(*req.Query)
	}
	if req.Sort != nil {
		c.Sort = cloneRawMessage(*req.Sort)
	}
	if req.PageSize != nil && *req.PageSize > 0 {
		c.PageSize = *req.PageSize
	}
	if len(req.SourceFieldNames) > 0 {
		c.SourceFieldNames = append([]string(nil), req.SourceFieldNames...)
	}
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\t' || b[i] == '\r') {
		i++
	}
	return b[i:j]
}

func cloneRawMessage(m json.RawMessage) json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	out := make([]byte, len(m))
	copy(out, m)
	return out
}

func jobToDebugJSON(j domain.Job) jobDebugJSON {
	out := jobDebugJSON{
		ID:          j.ID,
		Source:      j.Source,
		Title:       j.Title,
		Company:     j.Company,
		URL:         j.URL,
		ApplyURL:    j.ApplyURL,
		Description: j.Description,
		Remote:      j.Remote,
		CountryCode: j.CountryCode,
		SalaryRaw:   j.SalaryRaw,
		Tags:        j.Tags,
		Position:    j.Position,
		UserID:      j.UserID,
	}
	if !j.PostedAt.IsZero() {
		out.PostedAt = j.PostedAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}

type runCollectorResponse struct {
	OK              bool           `json:"ok"`
	Collector       string         `json:"collector"`
	Count           int            `json:"count"`
	UpstreamFetched int            `json:"upstream_fetched,omitempty"`
	Error           string         `json:"error,omitempty"`
	Jobs            []jobDebugJSON `json:"jobs,omitempty"`
}

type jobDebugJSON struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	URL         string   `json:"url"`
	ApplyURL    string   `json:"apply_url,omitempty"`
	Description string   `json:"description,omitempty"`
	PostedAt    string   `json:"posted_at,omitempty"`
	Remote      *bool    `json:"remote"`
	CountryCode string   `json:"country_code,omitempty"`
	SalaryRaw   string   `json:"salary_raw,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Position    *string  `json:"position,omitempty"`
	UserID      *string  `json:"user_id,omitempty"`
}
