// Package archgee implements the archgee.com HTML jobs collector.
package archgee

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "archgee"

const (
	siteOrigin   = "https://archgee.com"
	maxListPages = 40
)

// Archgee fetches one of the three contract queries with net/http.
type Archgee struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*Archgee) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Archgee) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// query is architect, landscape-architect, or remote. Anything else, including
// interior-designer, fetches nothing.
func (c *Archgee) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	kind, value, ok := queryPair(query)
	if !ok {
		return nil, nil
	}
	client := c.httpClient()
	seenURL := make(map[string]struct{})
	seenID := make(map[string]struct{})
	var out []schema.Job
	for page := 1; page <= maxListPages; page++ {
		rawURL := listURL(kind, value, page)
		body, err := get(ctx, client, rawURL)
		if err != nil {
			return nil, err
		}
		rows, hasNext, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		jobs, err := c.jobsFromRows(ctx, client, rows, seenURL, seenID)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
		if !hasNext {
			break
		}
	}
	return out, nil
}

func queryPair(query string) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(query)) {
	case "architect":
		return "category", "architect", true
	case "landscape-architect":
		return "category", "landscape-architect", true
	case "remote":
		return "remote", "remote", true
	default:
		return "", "", false
	}
}

func listURL(kind, value string, page int) string {
	raw := siteOrigin + "/jobs?" + kind + "=" + value
	if page > 1 {
		raw += "&page=" + strconv.Itoa(page)
	}
	return raw
}

func (c *Archgee) jobsFromRows(ctx context.Context, client *http.Client, rows []listRow, seenURL, seenID map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, row := range rows {
		if _, dup := seenURL[row.URL]; dup {
			continue
		}
		seenURL[row.URL] = struct{}{}
		body, err := get(ctx, client, row.URL)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, row)
		if err != nil {
			return nil, err
		}
		if _, dup := seenID[job.ID]; dup {
			continue
		}
		seenID[job.ID] = struct{}{}
		out = append(out, job)
	}
	return out, nil
}

func (c *Archgee) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if strings.Contains(strings.ToLower(rawURL), "/api/") {
		return nil, fmt.Errorf("archgee: refused %s", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("archgee: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("archgee: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("archgee: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}
