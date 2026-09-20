// Package remotifyeurope implements the Remotify Europe job board collector.
package remotifyeurope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// SourceName is the normative Job.Source value.
const SourceName = "remotify_europe"

// DefaultPageLimit is the server-side page size for listing POSTs.
const DefaultPageLimit = 24

// DefaultMaxPages caps POST pagination rounds when MaxPages is 0.
const DefaultMaxPages = 5

// RemotifyEurope fetches jobs via Next.js server actions and listing detail pages.
type RemotifyEurope struct {
	HTTPClient *http.Client
	// ListingURL defaults to DefaultListingURL.
	ListingURL string
	// MaxPages: 0 → DefaultMaxPages; -1 → no cap; >0 → explicit cap.
	MaxPages int
	// MaxJobs stops after this many jobs (0 = unlimited).
	MaxJobs int
	Now     func() time.Time
}

// Name implements collectors.Collector.
func (*RemotifyEurope) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *RemotifyEurope) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.fetchListing(ctx, "")
}

// FetchWithSlotSearch implements collectors.SlotSearchFetcher.
func (c *RemotifyEurope) FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error) {
	q := strings.TrimSpace(slotQuery)
	if q == "" {
		return c.Fetch(ctx)
	}
	return c.fetchListing(ctx, q)
}

func (c *RemotifyEurope) fetchListing(ctx context.Context, searchQuery string) ([]schema.Job, error) {
	client := c.httpClient()
	listingURL, err := listingPOSTURL(c.ListingURL)
	if err != nil {
		return nil, err
	}

	action, err := DiscoverNextAction(ctx, client, listingURL)
	if err != nil {
		return c.fetchFallbackHTML(ctx, client, listingURL)
	}

	maxPages := c.maxPagesEffective()
	nowFn := c.nowFn()
	cursor := nowFn().UTC().Format(time.RFC3339Nano)
	seen := make(map[string]struct{})
	var out []schema.Job

	for page := 0; maxPages == 0 || page < maxPages; page++ {
		body, err := c.postFlight(ctx, client, listingURL, action, searchQuery, cursor)
		if err != nil {
			return nil, err
		}
		stubs, err := ParseFlightListing(body)
		if err != nil {
			return nil, err
		}
		if len(stubs) == 0 {
			break
		}
		for _, stub := range stubs {
			if _, ok := seen[stub.ID]; ok {
				continue
			}
			seen[stub.ID] = struct{}{}
			j, err := c.fetchJobDetail(ctx, client, stub)
			if err != nil {
				continue
			}
			out = append(out, j)
			if c.MaxJobs > 0 && len(out) >= c.MaxJobs {
				return out, nil
			}
		}
		if len(stubs) < DefaultPageLimit {
			break
		}
		last := stubs[len(stubs)-1]
		if strings.TrimSpace(last.CreatedAt) == "" {
			break
		}
		cursor = last.CreatedAt
	}
	return out, nil
}

func (c *RemotifyEurope) fetchFallbackHTML(ctx context.Context, client *http.Client, listingURL string) ([]schema.Job, error) {
	ids, err := listingIDsFromHTML(ctx, client, listingURL)
	if err != nil {
		return nil, err
	}
	var out []schema.Job
	for _, id := range ids {
		j, err := c.fetchJobDetail(ctx, client, flightJobStub{ID: id})
		if err != nil {
			continue
		}
		out = append(out, j)
		if c.MaxJobs > 0 && len(out) >= c.MaxJobs {
			break
		}
	}
	return out, nil
}

func (c *RemotifyEurope) fetchJobDetail(ctx context.Context, client *http.Client, stub flightJobStub) (schema.Job, error) {
	detailURL := ListingJobURL(stub.ID)
	html, err := c.getHTML(ctx, client, detailURL)
	if err != nil {
		return schema.Job{}, err
	}
	detail, err := ParseListingDetailHTML(string(html))
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(detail.Title)
	if title == "" {
		title = strings.TrimSpace(stub.JobTitle)
	}
	company := strings.TrimSpace(detail.Company)
	if company == "" {
		company = strings.TrimSpace(stub.CompanyName)
	}
	if title == "" || company == "" {
		return schema.Job{}, fmt.Errorf("remotify europe: missing title or company for %s", stub.ID)
	}
	j := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         detailURL,
		ApplyURL:    strings.TrimSpace(detail.JobLink),
		Description: detail.Description,
	}
	if !detail.PostedAt.IsZero() {
		j.PostedAt = detail.PostedAt
	}
	if detail.RemoteKnown {
		j.Remote = detail.Remote
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, err
	}
	return j, nil
}

func (c *RemotifyEurope) postFlight(ctx context.Context, client *http.Client, listingURL, action, searchQuery, cursor string) ([]byte, error) {
	payload := []any{
		DefaultPageLimit,
		cursor,
		[]string{},
		searchQuery,
		[]string{},
		[]string{},
		[]string{},
		[]string{},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listingURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/x-component")
	req.Header.Set("Next-Action", action)
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remotify europe: listing post: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remotify europe: listing post: HTTP %s", resp.Status)
	}
	return b, nil
}

func (c *RemotifyEurope) getHTML(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remotify europe: GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remotify europe: GET %s: HTTP %s", u, resp.Status)
	}
	return b, nil
}

func (c *RemotifyEurope) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func (c *RemotifyEurope) nowFn() func() time.Time {
	if c.Now != nil {
		return c.Now
	}
	return func() time.Time { return time.Now().UTC() }
}

func (c *RemotifyEurope) maxPagesEffective() int {
	switch {
	case c.MaxPages == 0:
		return DefaultMaxPages
	case c.MaxPages < 0:
		return 0
	default:
		return c.MaxPages
	}
}
