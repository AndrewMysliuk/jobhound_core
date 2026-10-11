// Package consider fetches portfolio jobs from Consider-hosted fund boards.
package consider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

const (
	defaultMaxPages = 40
	searchPageSize  = 50
	maxBodyBytes    = 8 << 20
)

// Tenant is one Consider fund. Host is the origin; BoardID is board.id.
type Tenant struct {
	SourceID string
	Host     string
	BoardID  string
}

// Board is one Consider fund. FetchUnion builds a cookie jar on a client that
// shares HTTPClient's transport. The jar is not HTTPClient.
type Board struct {
	HTTPClient *http.Client
	Countries  *utils.CountryResolver
	Tenant     Tenant
	// MaxPages caps search pages. 0 means 40. Hitting the cap returns the jobs already mapped.
	MaxPages int
}

var (
	_ collectors.Collector         = (*Board)(nil)
	_ collectors.UnionQueryFetcher = (*Board)(nil)

	csrfTokenRE = regexp.MustCompile(`"csrfToken"\s*:\s*"([^"]*)"`)

	// Exact match on the trimmed profile word. "go" is not a key.
	profileSkillID = map[string]string{
		"vue":        "resume:Vue.js",
		"golang":     "resume:Go",
		"typescript": "resume:TypeScript",
		"react":      "resume:React.js",
		"node":       "resume:Node.js",
	}
)

// Tenants is every Consider fund in the contract, in table order.
func Tenants() []Tenant {
	return []Tenant{
		{SourceID: "consider_andreessen_horowitz", Host: "https://portfoliojobs.a16z.com", BoardID: "andreessen-horowitz"},
		{SourceID: "consider_lightspeed", Host: "https://jobs.lsvp.com", BoardID: "lightspeed"},
		{SourceID: "consider_sequoia_capital", Host: "https://jobs.sequoiacap.com", BoardID: "sequoia-capital"},
		{SourceID: "consider_gv", Host: "https://jobs.gv.com", BoardID: "gv"},
		{SourceID: "consider_bessemer_ventures", Host: "https://jobs.bvp.com", BoardID: "bessemer-ventures"},
		{SourceID: "consider_balderton_capital", Host: "https://careers.balderton.com", BoardID: "balderton-capital"},
		{SourceID: "consider_notion_capital", Host: "https://jobs.notion.vc", BoardID: "notion-capital"},
		{SourceID: "consider_atlantic_labs", Host: "https://jobs.atlantic.vc", BoardID: "atlantic-labs"},
		{SourceID: "consider_battery_ventures", Host: "https://jobs.battery.com", BoardID: "battery-ventures"},
		{SourceID: "consider_panoramic_ventures", Host: "https://jobs.panoramic.vc", BoardID: "panoramic-ventures"},
		{SourceID: "consider_ivp", Host: "https://careers.ivp.com", BoardID: "ivp"},
		{SourceID: "consider_kleiner_perkins", Host: "https://jobs.kleinerperkins.com", BoardID: "kleiner-perkins"},
		{SourceID: "consider_nea", Host: "https://careers.nea.com", BoardID: "nea"},
		{SourceID: "consider_sequoia_capital_india", Host: "https://careers.peakxv.com", BoardID: "sequoia-capital-india"},
		{SourceID: "consider_union_square_ventures", Host: "https://jobs.usv.com", BoardID: "union-square-ventures"},
	}
}

// Name implements collectors.Collector.
func (b *Board) Name() string {
	if b == nil {
		return ""
	}
	return b.Tenant.SourceID
}

// Fetch implements collectors.Collector. It does not call the network.
func (b *Board) Fetch(context.Context) ([]schema.Job, error) {
	return []schema.Job{}, nil
}

type searchBody struct {
	Meta    json.RawMessage `json:"meta"`
	Board   boardBody       `json:"board"`
	Query   queryBody       `json:"query"`
	Grouped bool            `json:"grouped"`
}

type boardBody struct {
	ID       string `json:"id"`
	IsParent bool   `json:"isParent"`
}

type queryBody struct {
	Skills    []string `json:"skills"`
	Locations []string `json:"locations"`
}

// FetchUnion implements collectors.UnionQueryFetcher.
func (b *Board) FetchUnion(ctx context.Context, queries []string) ([]schema.Job, error) {
	if b == nil {
		return nil, errors.New("consider: nil board")
	}
	skills := resolveSkills(queries)
	if len(skills) == 0 {
		b.logFetched(ctx, nil)
		return []schema.Job{}, nil
	}
	host, err := tenantHost(b)
	if err != nil {
		return nil, err
	}
	client, err := sessionClient(b.HTTPClient)
	if err != nil {
		return nil, err
	}
	sessionURL, err := url.JoinPath(host, "jobs")
	if err != nil {
		return nil, fmt.Errorf("consider: %s: session url: %w", b.Tenant.SourceID, err)
	}
	searchURL, err := url.JoinPath(host, "api-boards", "search-jobs")
	if err != nil {
		return nil, fmt.Errorf("consider: %s: search url: %w", b.Tenant.SourceID, err)
	}

	token, err := b.openSession(ctx, client, sessionURL)
	if err != nil {
		return nil, err
	}

	meta, err := json.Marshal(struct {
		Size int `json:"size"`
	}{Size: searchPageSize})
	if err != nil {
		return nil, fmt.Errorf("consider: %s: search meta: %w", b.Tenant.SourceID, err)
	}

	seen := map[string]struct{}{}
	var jobs []schema.Job
	for page := 0; page < b.maxPages(); page++ {
		pageJobs, nextMeta, err := b.searchPage(ctx, client, searchURL, token, meta, skills)
		if err != nil {
			return nil, err
		}
		if len(pageJobs) == 0 {
			break
		}
		if pageAllSeen(pageJobs, seen) {
			break
		}
		for _, w := range pageJobs {
			if id := strings.TrimSpace(w.JobID); id != "" {
				seen[id] = struct{}{}
			}
			job, ok := jobFromWire(b.Tenant.SourceID, b.Countries, w)
			if !ok {
				continue
			}
			jobs = append(jobs, job)
		}
		if !metaHasSequence(nextMeta) {
			break
		}
		meta = nextMeta
	}
	b.logFetched(ctx, jobs)
	if jobs == nil {
		return []schema.Job{}, nil
	}
	return jobs, nil
}

func (b *Board) maxPages() int {
	if b == nil || b.MaxPages <= 0 {
		return defaultMaxPages
	}
	return b.MaxPages
}

func (b *Board) logFetched(ctx context.Context, jobs []schema.Job) {
	withApply := 0
	for _, j := range jobs {
		if strings.TrimSpace(j.ApplyURL) != "" {
			withApply++
		}
	}
	slog.InfoContext(ctx, "consider: fetch",
		"source", b.Tenant.SourceID,
		"jobs", len(jobs),
		"with_apply_url", withApply,
	)
}

func resolveSkills(queries []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, q := range queries {
		id, ok := profileSkillID[strings.TrimSpace(q)]
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func tenantHost(b *Board) (string, error) {
	host := strings.TrimRight(strings.TrimSpace(b.Tenant.Host), "/")
	if host == "" {
		return "", fmt.Errorf("consider: %s: empty host", b.Tenant.SourceID)
	}
	u, err := url.Parse(host)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("consider: %s: host %q", b.Tenant.SourceID, b.Tenant.Host)
	}
	return host, nil
}

func sessionClient(base *http.Client) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("consider: cookie jar: %w", err)
	}
	if base == nil {
		base = utils.NewHTTPClient()
	}
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Transport: transport,
		Timeout:   base.Timeout,
		Jar:       jar,
	}, nil
}

func (b *Board) openSession(ctx context.Context, client *http.Client, sessionURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("consider: %s: session request: %w", b.Tenant.SourceID, err)
	}
	utils.SetCollectorUserAgent(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("consider: %s: GET /jobs: %w", b.Tenant.SourceID, err)
	}
	body, err := readBody(resp)
	if err != nil {
		return "", fmt.Errorf("consider: %s: GET /jobs: %w", b.Tenant.SourceID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", statusErr(b.Tenant.SourceID, "GET /jobs", resp.StatusCode)
	}
	token, ok := csrfFromHTML(body)
	if !ok {
		return "", fmt.Errorf("consider: %s: csrfToken missing", b.Tenant.SourceID)
	}
	return token, nil
}

func csrfFromHTML(body []byte) (string, bool) {
	m := csrfTokenRE.FindSubmatch(body)
	if m == nil {
		return "", false
	}
	token := strings.TrimSpace(string(m[1]))
	if token == "" {
		return "", false
	}
	return token, true
}

func (b *Board) searchPage(ctx context.Context, client *http.Client, searchURL, token string, meta json.RawMessage, skills []string) ([]jobWire, json.RawMessage, error) {
	payload, err := json.Marshal(searchBody{
		Meta: meta,
		Board: boardBody{
			ID:       b.Tenant.BoardID,
			IsParent: true,
		},
		Query: queryBody{
			Skills:    skills,
			Locations: []string{"Europe", "United States"},
		},
		Grouped: false,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("consider: %s: search body: %w", b.Tenant.SourceID, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("consider: %s: search request: %w", b.Tenant.SourceID, err)
	}
	utils.SetCollectJSONPostHeaders(req)
	req.Header.Set("x-csrf-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("consider: %s: search: %w", b.Tenant.SourceID, err)
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, nil, fmt.Errorf("consider: %s: search: %w", b.Tenant.SourceID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, statusErr(b.Tenant.SourceID, "search", resp.StatusCode)
	}
	return decodeSearch(b.Tenant.SourceID, body)
}

func decodeSearch(sourceID string, body []byte) ([]jobWire, json.RawMessage, error) {
	var env struct {
		Meta json.RawMessage `json:"meta"`
		Jobs json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, fmt.Errorf("consider: %s: search json: %w", sourceID, err)
	}
	raw := bytes.TrimSpace(env.Jobs)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || raw[0] != '[' {
		return nil, nil, fmt.Errorf("consider: %s: search response has no jobs array", sourceID)
	}
	var jobs []jobWire
	if err := json.Unmarshal(raw, &jobs); err != nil {
		return nil, nil, fmt.Errorf("consider: %s: search jobs: %w", sourceID, err)
	}
	return jobs, env.Meta, nil
}

func metaHasSequence(meta json.RawMessage) bool {
	meta = bytes.TrimSpace(meta)
	if len(meta) == 0 || bytes.Equal(meta, []byte("null")) {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(meta, &fields); err != nil {
		return false
	}
	raw, ok := fields["sequence"]
	if !ok {
		return false
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}

func pageAllSeen(jobs []jobWire, seen map[string]struct{}) bool {
	if len(jobs) == 0 {
		return false
	}
	for _, j := range jobs {
		id := strings.TrimSpace(j.JobID)
		if id == "" {
			return false
		}
		if _, ok := seen[id]; !ok {
			return false
		}
	}
	return true
}

func statusErr(sourceID, op string, code int) error {
	if code == http.StatusNotFound || code == http.StatusGone {
		return fmt.Errorf("consider: %s: %s: HTTP %d: %w", sourceID, op, code, collectors.ErrTenantUnavailable)
	}
	return fmt.Errorf("consider: %s: %s: HTTP %d", sourceID, op, code)
}

func readBody(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}
