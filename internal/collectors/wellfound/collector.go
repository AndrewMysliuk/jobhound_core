// Package wellfound implements the Wellfound (AngelList Talent) job board collector.
package wellfound

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// siteMu serializes Wellfound HTTP so the 4 role children do not burst (429 / Turnstile).
var siteMu sync.Mutex

// SourceName is the normative Job.Source value.
const SourceName = "wellfound"

// DefaultMaxPages caps listing pagination when MaxPages is 0.
const DefaultMaxPages = 5

// DefaultInterRequestDelay is applied after each GET when InterRequestDelay is zero.
const DefaultInterRequestDelay = 1000 * time.Millisecond

const listingPageSizeHint = 25

// Wellfound fetches role listing HTML then job detail pages (JSON-LD JobPosting).
type Wellfound struct {
	HTTPClient *http.Client
	// MaxPages: 0 → DefaultMaxPages; -1 → no cap; >0 → explicit cap.
	MaxPages int
	// MaxJobs stops after this many jobs (0 = unlimited).
	MaxJobs int
	// InterRequestDelay overrides DefaultInterRequestDelay when > 0.
	InterRequestDelay time.Duration
}

// Name implements collectors.Collector.
func (*Wellfound) Name() string { return SourceName }

// Fetch implements collectors.Collector (slug matrix only; no catalog Fetch).
func (*Wellfound) Fetch(context.Context) ([]schema.Job, error) {
	return []schema.Job{}, nil
}

// FetchWithSlotSearch implements collectors.SlotSearchFetcher.
func (c *Wellfound) FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error) {
	slug := strings.TrimSpace(slotQuery)
	if slug == "" {
		return c.Fetch(ctx)
	}
	return c.fetchRole(ctx, slug)
}

func (c *Wellfound) fetchRole(ctx context.Context, slug string) ([]schema.Job, error) {
	client := c.httpClient()
	maxPages := c.maxPagesEffective()
	seen := make(map[string]struct{})
	var stubs []ListingJob

	for page := 1; maxPages == 0 || page <= maxPages; page++ {
		listURL := roleListingURL(slug, page)
		html, finalURL, err := c.getHTML(ctx, client, listURL)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(finalURL, "/role/r/"+slug) {
			return nil, fmt.Errorf("wellfound: role %q redirected to %s", slug, finalURL)
		}
		pageJobs, err := ParseListingHTML(string(html))
		if err != nil {
			return nil, fmt.Errorf("wellfound: listing page %d: %w", page, err)
		}
		if len(pageJobs) == 0 {
			break
		}
		for _, j := range pageJobs {
			if _, ok := seen[j.Path]; ok {
				continue
			}
			seen[j.Path] = struct{}{}
			stubs = append(stubs, j)
		}
		if len(pageJobs) < listingPageSizeHint {
			break
		}
	}

	var out []schema.Job
	for _, stub := range stubs {
		if c.MaxJobs > 0 && len(out) >= c.MaxJobs {
			break
		}
		jobURL := jobListingURL(stub.Path)
		detailHTML, _, err := c.getHTML(ctx, client, jobURL)
		if err != nil {
			continue
		}
		detail, err := ParseJobDetailHTML(string(detailHTML))
		if err != nil {
			continue
		}
		title := strings.TrimSpace(stub.Title)
		if title == "" {
			title = strings.TrimSpace(detail.Title)
		}
		company := strings.TrimSpace(stub.Company)
		if company == "" {
			company = strings.TrimSpace(detail.Company)
		}
		if title == "" || company == "" {
			continue
		}
		canonURL, err := utils.CanonicalListingURL(jobURL)
		if err != nil {
			continue
		}
		j := schema.Job{
			Source:      SourceName,
			Title:       title,
			Company:     company,
			URL:         canonURL,
			ApplyURL:    "",
			Description: detail.Description,
			PostedAt:    detail.PostedAt,
			Remote:      detail.Remote,
		}
		if err := domainutils.AssignStableID(&j); err != nil {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

func (c *Wellfound) getHTML(ctx context.Context, client *http.Client, u string) ([]byte, string, error) {
	siteMu.Lock()
	defer siteMu.Unlock()

	var last error
	for attempt := 0; attempt < 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if attempt > 0 {
			if err := sleepFor(ctx, time.Duration(attempt)*3*time.Second); err != nil {
				return nil, "", err
			}
		}
		b, final, status, err := doGET(ctx, client, u)
		if err != nil {
			return nil, "", fmt.Errorf("wellfound: GET %s: %w", u, err)
		}
		if status == http.StatusTooManyRequests {
			last = fmt.Errorf("wellfound: GET %s: HTTP %s", u, statusText(status))
			continue
		}
		if status < 200 || status >= 300 {
			return nil, "", fmt.Errorf("wellfound: GET %s: HTTP %s", u, statusText(status))
		}
		_ = sleepFor(ctx, c.interDelay())
		return b, final, nil
	}
	return nil, "", last
}

func doGET(ctx context.Context, client *http.Client, u string) ([]byte, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, "", 0, err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", resp.StatusCode, err
	}
	final := u
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return b, final, resp.StatusCode, nil
}

func statusText(code int) string {
	return fmt.Sprintf("%d %s", code, http.StatusText(code))
}

func (c *Wellfound) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func (c *Wellfound) maxPagesEffective() int {
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

func (c *Wellfound) interDelay() time.Duration {
	if c != nil && c.InterRequestDelay > 0 {
		return c.InterRequestDelay
	}
	return DefaultInterRequestDelay
}

func sleepFor(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
