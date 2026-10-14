// Package competitionline implements the competitionline.com jobs catalog collector.
package competitionline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "competitionline"

const (
	siteOrigin   = "https://www.competitionline.com"
	maxListPages = 40
)

// Competitionline fetches the unfiltered /api/jobs/ catalog with net/http.
type Competitionline struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*Competitionline) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Competitionline) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// The profile child is catalog, which ingest sends as an empty query.
// The list is unfiltered: the query is not sent, and fs is never sent.
func (c *Competitionline) FetchWithQuery(ctx context.Context, _ string) ([]schema.Job, error) {
	client := c.httpClient()
	seen := make(map[int]struct{})
	var out []schema.Job
	for page := 0; page < maxListPages; page++ {
		body, err := get(ctx, client, listURL(page))
		if err != nil {
			return nil, err
		}
		hits, next, err := parseList(body)
		if err != nil {
			return nil, err
		}
		jobs, err := c.jobsFromHits(ctx, client, hits, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
		if strings.TrimSpace(next) == "" {
			break
		}
	}
	return out, nil
}

func listURL(page int) string {
	return siteOrigin + "/api/jobs/?page=" + strconv.Itoa(page)
}

func detailURL(id int) string {
	return siteOrigin + "/api/jobs/" + strconv.Itoa(id)
}

func (c *Competitionline) jobsFromHits(ctx context.Context, client *http.Client, hits []listHit, seen map[int]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, hit := range hits {
		if hit.ID <= 0 {
			continue
		}
		if _, dup := seen[hit.ID]; dup {
			continue
		}
		seen[hit.ID] = struct{}{}
		body, err := get(ctx, client, detailURL(hit.ID))
		if err != nil {
			return nil, err
		}
		job, err := parseDetail(body, hit.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *Competitionline) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(u.Path), "/api/modgnikehtotsyek") {
		return nil, fmt.Errorf("competitionline: refused %s", rawURL)
	}
	if _, ok := u.Query()["fs"]; ok {
		return nil, fmt.Errorf("competitionline: refused fs on %s", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("competitionline: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("competitionline: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("competitionline: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}
