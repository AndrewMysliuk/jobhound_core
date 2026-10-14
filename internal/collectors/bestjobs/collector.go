// Package bestjobs implements the bestjobs.eu board collector.
package bestjobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "bestjobs"

const (
	siteOrigin   = "https://www.bestjobs.eu"
	listPrefix   = siteOrigin + "/locuri-de-munca/"
	detailPrefix = siteOrigin + "/loc-de-munca/"
	jobsAPI      = siteOrigin + "/api/proxy/v2/jobs"
	pageLimit    = "24"
)

// BestJobs fetches the Romania and remote keyword lists, then cursor pages.
type BestJobs struct {
	HTTPClient *http.Client
	Countries  *utils.CountryResolver
}

// Name implements collectors.Collector.
func (*BestJobs) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *BestJobs) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// One query loads only the -in-romania and -in-remote listing URLs.
func (c *BestJobs) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	lists, ok := listingURLs(query)
	if !ok {
		return nil, nil
	}
	client := c.httpClient()
	seen := make(map[string]struct{})
	var out []schema.Job
	for _, listURL := range lists {
		jobs, err := c.fetchListing(ctx, client, listURL, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func listingURLs(query string) ([2]string, bool) {
	fields := strings.Fields(strings.TrimSpace(query))
	if len(fields) == 0 {
		return [2]string{}, false
	}
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = url.PathEscape(field)
	}
	slug := strings.Join(parts, "+")
	return [2]string{
		listPrefix + slug + "-in-romania",
		listPrefix + slug + "-in-remote",
	}, true
}

func (c *BestJobs) fetchListing(ctx context.Context, client *http.Client, listURL string, seen map[string]struct{}) ([]schema.Job, error) {
	body, err := get(ctx, client, listURL)
	if err != nil {
		return nil, err
	}
	page, err := parseListHTML(body)
	if err != nil {
		return nil, err
	}
	out, err := c.jobsFromCards(ctx, client, page.Items, seen)
	if err != nil {
		return nil, err
	}
	filters := filterValues(page.Filters)
	if filters == nil || page.NextCursor == nil {
		return out, nil
	}
	cursor := strings.TrimSpace(*page.NextCursor)
	seenCursor := make(map[string]struct{})
	for cursor != "" {
		if _, dup := seenCursor[cursor]; dup {
			break
		}
		seenCursor[cursor] = struct{}{}
		apiBody, err := get(ctx, client, jobsAPIURL(filters, cursor))
		if err != nil {
			return nil, err
		}
		apiPage, err := parseJobsAPI(apiBody)
		if err != nil {
			return nil, err
		}
		more, err := c.jobsFromCards(ctx, client, apiPage.Items, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
		if apiPage.NextCursor == nil {
			break
		}
		cursor = strings.TrimSpace(*apiPage.NextCursor)
	}
	return out, nil
}

func (c *BestJobs) jobsFromCards(ctx context.Context, client *http.Client, cards []listCard, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, card := range cards {
		slug := strings.TrimSpace(card.Slug)
		if slug == "" {
			continue
		}
		listing, err := detailURL(slug)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[listing]; dup {
			continue
		}
		seen[listing] = struct{}{}
		body, err := get(ctx, client, listing)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, c.Countries, listing)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func detailURL(slug string) (string, error) {
	if strings.ContainsAny(slug, "/?#") {
		return "", fmt.Errorf("bestjobs: bad slug %q", slug)
	}
	return utils.CanonicalListingURL(detailPrefix + slug)
}

func jobsAPIURL(filters url.Values, cursor string) string {
	q := url.Values{}
	for key, values := range filters {
		if key == "_lat" || key == "_lon" {
			continue
		}
		for _, value := range values {
			q.Add(key, value)
		}
	}
	q.Set("limit", pageLimit)
	q.Set("cursor", cursor)
	return jobsAPI + "?" + q.Encode()
}

func (c *BestJobs) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bestjobs: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bestjobs: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bestjobs: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}
