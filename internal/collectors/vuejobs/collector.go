// Package vuejobs implements the VueJobs.com board collector (Nuxt SSR catalog).
package vuejobs

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "vue_jobs"

// DefaultMaxPages caps listing pagination when MaxPages is 0.
const DefaultMaxPages = 5

const listingPageSizeHint = 25

// VueJobs fetches remote catalog pages and parses __NUXT_DATA__.
type VueJobs struct {
	HTTPClient *http.Client
	// MaxPages: 0 → DefaultMaxPages; -1 → no cap; >0 → explicit cap.
	MaxPages int
	// MaxJobs stops after this many jobs (0 = unlimited).
	MaxJobs int
}

// Name implements collectors.Collector.
func (*VueJobs) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *VueJobs) Fetch(ctx context.Context) ([]schema.Job, error) {
	client := c.httpClient()
	maxPages := c.maxPagesEffective()
	var out []schema.Job
	seenSlug := make(map[string]struct{})

	for page := 1; maxPages == 0 || page <= maxPages; page++ {
		html, err := c.getListingHTML(ctx, client, page)
		if err != nil {
			return nil, err
		}
		rows, pager, err := ParseListingHTML(html)
		if err != nil {
			return nil, fmt.Errorf("vuejobs: page %d: %w", page, err)
		}
		pageJobs, err := jobsFromListing(rows)
		if err != nil {
			return nil, fmt.Errorf("vuejobs: page %d: %w", page, err)
		}
		for _, j := range pageJobs {
			if c.MaxJobs > 0 && len(out) >= c.MaxJobs {
				return out, nil
			}
			key := j.URL
			if key == "" {
				continue
			}
			if _, dup := seenSlug[key]; dup {
				continue
			}
			seenSlug[key] = struct{}{}
			out = append(out, j)
		}
		if pager.CurrentPage >= pager.LastPage || len(rows) < listingPageSizeHint {
			break
		}
		if page >= pager.LastPage {
			break
		}
	}
	return out, nil
}

func (c *VueJobs) getListingHTML(ctx context.Context, client *http.Client, page int) ([]byte, error) {
	u := ListingURL(page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vuejobs: GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vuejobs: GET %s: HTTP %s", u, resp.Status)
	}
	return b, nil
}

func (c *VueJobs) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func (c *VueJobs) maxPagesEffective() int {
	if c == nil {
		return DefaultMaxPages
	}
	switch {
	case c.MaxPages == 0:
		return DefaultMaxPages
	case c.MaxPages < 0:
		return 0
	default:
		return c.MaxPages
	}
}
