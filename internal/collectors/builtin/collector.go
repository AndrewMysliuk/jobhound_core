package builtin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// SourceName is the normative Job.Source value (contracts/collector.md).
const SourceName = "builtin"

const listingPageSizeHint = 20

// BuiltIn fetches remote Built In listings (JSON-LD ItemList) then job details (JobPosting).
type BuiltIn struct {
	HTTPClient *http.Client
	// ListingBase is the listing URL without query (default https://builtin.com/jobs/remote); tests may override.
	ListingBase       *url.URL
	InterRequestDelay time.Duration
	// MaxListingPagesPerCountry is 1 or 2; zero means 2 (resources/builtin.md).
	MaxListingPagesPerCountry int
	// MaxJobs stops after N successful jobs when > 0 (debug / response caps).
	MaxJobs int
	// TestAlpha3 limits countries when non-empty (unit tests only); production uses all normative territories.
	TestAlpha3 []string
	// OnDatePostedWarn is called when datePosted is missing or unparseable (soft failure).
	OnDatePostedWarn func(raw string)
}

// Name implements collectors.Collector.
func (*BuiltIn) Name() string { return SourceName }

// Fetch implements collectors.Collector (search-only board: no HTTP; contracts/collector.md).
func (*BuiltIn) Fetch(context.Context) ([]schema.Job, error) {
	return []schema.Job{}, nil
}

// FetchWithSlotSearch implements collectors.SlotSearchFetcher.
func (c *BuiltIn) FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error) {
	q := strings.TrimSpace(slotQuery)
	if q == "" {
		return []schema.Job{}, nil
	}
	return c.fetchRemote(ctx, q)
}

func (c *BuiltIn) listingBaseResolved() (*url.URL, error) {
	if c != nil && c.ListingBase != nil {
		return c.ListingBase, nil
	}
	return url.Parse("https://builtin.com/jobs/remote")
}

func (c *BuiltIn) maxPagesPerCountry() int {
	if c == nil || c.MaxListingPagesPerCountry <= 0 {
		return 2
	}
	if c.MaxListingPagesPerCountry > 2 {
		return 2
	}
	return c.MaxListingPagesPerCountry
}

func (c *BuiltIn) territoriesOrdered() []territory {
	if c == nil || len(c.TestAlpha3) == 0 {
		return normativeTerritories
	}
	want := make(map[string]struct{}, len(c.TestAlpha3))
	for _, a3 := range c.TestAlpha3 {
		a3 = strings.ToUpper(strings.TrimSpace(a3))
		if a3 != "" {
			want[a3] = struct{}{}
		}
	}
	var out []territory
	for _, t := range normativeTerritories {
		if _, ok := want[t.Alpha3]; ok {
			out = append(out, t)
		}
	}
	return out
}

func (c *BuiltIn) fetchRemote(ctx context.Context, search string) ([]schema.Job, error) {
	base, err := c.listingBaseResolved()
	if err != nil {
		return nil, err
	}
	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	maxPages := c.maxPagesPerCountry()

	seen := make(map[string]struct{})
	var ordered []struct {
		url    string
		alpha2 string
	}

	for _, terr := range c.territoriesOrdered() {
		for page := 1; page <= maxPages; page++ {
			c.sleep(ctx)
			listURL, err := c.buildListingURL(base, search, terr.Alpha3, page)
			if err != nil {
				return nil, err
			}
			listHTML, err := httpGet(ctx, client, listURL)
			if err != nil {
				return nil, fmt.Errorf("listing %s page %d: %w", terr.Alpha3, page, err)
			}
			urls, err := parseListingJobURLs(string(listHTML))
			if err != nil {
				return nil, fmt.Errorf("listing %s page %d: %w", terr.Alpha3, page, err)
			}
			if len(urls) == 0 {
				break
			}
			for _, rawU := range urls {
				canon, err := utils.CanonicalListingURL(rawU)
				if err != nil {
					return nil, fmt.Errorf("listing URL: %w", err)
				}
				if _, ok := seen[canon]; ok {
					continue
				}
				seen[canon] = struct{}{}
				ordered = append(ordered, struct {
					url    string
					alpha2 string
				}{url: canon, alpha2: terr.Alpha2})
			}
			if len(urls) < listingPageSizeHint {
				break
			}
		}
	}

	var jobs []schema.Job
	for _, uc := range ordered {
		if c != nil && c.MaxJobs > 0 && len(jobs) >= c.MaxJobs {
			break
		}
		c.sleep(ctx)
		detailHTML, err := httpGet(ctx, client, uc.url)
		if err != nil {
			return nil, fmt.Errorf("detail %s: %w", uc.url, err)
		}
		jp, err := parseJobPostingFromDetailHTML(string(detailHTML))
		if err != nil {
			return nil, fmt.Errorf("detail %s: %w", uc.url, err)
		}
		title := strings.TrimSpace(jp.title)
		if title == "" {
			return nil, fmt.Errorf("detail %s: empty title", uc.url)
		}
		tags := builtinTags(jp)
		descPlain := plainFromJSONLDDescription(jp.description)
		urlForJob := jp.rawURL
		if strings.TrimSpace(urlForJob) == "" {
			urlForJob = uc.url
		}
		canonURL, err := utils.CanonicalListingURL(urlForJob)
		if err != nil {
			return nil, fmt.Errorf("detail %s: job url: %w", uc.url, err)
		}
		applyURL := applyURLFromHiringOrg(jp.hiringOrg)
		if strings.TrimSpace(applyURL) != "" {
			if au, err := utils.CanonicalListingURL(applyURL); err == nil {
				applyURL = au
			}
		}
		j := schema.Job{
			Source:      SourceName,
			Title:       title,
			Company:     strings.TrimSpace(jp.company),
			URL:         canonURL,
			ApplyURL:    applyURL,
			Description: descPlain,
			PostedAt:    parsePostedAt(jp.datePosted, c.onDatePostedWarn()),
			Remote:      resolveRemote(jp, tags),
			CountryCode: uc.alpha2,
			SalaryRaw:   formatSalaryRaw(jp.baseSalaryRaw),
			Tags:        tags,
			Position:    utils.InferPosition(title, descPlain, tags),
		}
		if err := domainutils.AssignStableID(&j); err != nil {
			return nil, fmt.Errorf("stable id: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (c *BuiltIn) onDatePostedWarn() func(string) {
	if c == nil {
		return nil
	}
	return c.OnDatePostedWarn
}

func (c *BuiltIn) buildListingURL(base *url.URL, search, alpha3 string, page int) (string, error) {
	if base == nil {
		return "", fmt.Errorf("listing base URL is nil")
	}
	u := *base
	q := url.Values{}
	q.Set("country", alpha3)
	q.Set("allLocations", "true")
	q.Set("search", search)
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *BuiltIn) sleep(ctx context.Context) {
	if c == nil || c.InterRequestDelay <= 0 {
		return
	}
	t := time.NewTimer(c.InterRequestDelay)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func parsePostedAt(raw string, warn func(string)) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	if warn != nil {
		warn(raw)
	}
	return time.Time{}
}

func httpGet(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpStatusError(resp.Status, b)
	}
	return b, nil
}

func httpStatusError(status string, body []byte) error {
	snippet := string(body)
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	return fmt.Errorf("HTTP %s: %s", status, strings.TrimSpace(snippet))
}
