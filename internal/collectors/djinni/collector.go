// Package djinni implements the Djinni jobs collector (specs/005-job-collectors/resources/djinni.md).
package djinni

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// SourceName is the normative Job.Source value (contracts/collector.md).
const SourceName = "djinni"

// DefaultMaxJobs caps jobs returned per Fetch when MaxJobs <= 0 (contracts/environment.md).
const DefaultMaxJobs = 100

// DefaultAllKeywords is used when AllKeywords is empty on Fetch (slot-free pipeline runs).
const DefaultAllKeywords = "go"

const maxJobsHardCap = 500

const jobsPerPageFull = 15

// Djinni fetches listing pages then job detail HTML with JobPosting JSON-LD.
type Djinni struct {
	HTTPClient *http.Client
	// SiteBase overrides https://djinni.co/ for tests; nil uses DefaultSiteBase().
	SiteBase *url.URL
	// AllKeywords is the listing all_keywords query param for Fetch; empty uses DefaultAllKeywords.
	AllKeywords string
	// MaxJobs caps collected jobs after detail fetches; <= 0 uses DefaultMaxJobs.
	MaxJobs int
	// InterRequestDelay is applied between consecutive HTTP calls; zero skips.
	InterRequestDelay time.Duration
	// StartPage is the first listing page (1-based); values < 1 are treated as 1.
	StartPage int
	Countries *utils.CountryResolver
	// OnDatePostedWarn is called when datePosted is missing or unparseable (soft failure).
	OnDatePostedWarn func(raw string)
}

// Name implements collectors.Collector.
func (*Djinni) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Djinni) Fetch(ctx context.Context) ([]schema.Job, error) {
	keywords := strings.TrimSpace(c.AllKeywords)
	if keywords == "" {
		keywords = DefaultAllKeywords
	}
	base := c.SiteBase
	if base == nil {
		u, err := DefaultSiteBase()
		if err != nil {
			return nil, err
		}
		base = u
	}
	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	max := c.maxJobsEffective()
	page := c.StartPage
	if page < 1 {
		page = 1
	}

	seen := make(map[string]struct{})
	var cards []listingCard

	for {
		if len(cards) >= max {
			break
		}
		c.sleep(ctx)
		listURL := buildListingURL(base, keywords, page)
		listHTML, err := httpGet(ctx, client, listURL)
		if err != nil {
			return nil, fmt.Errorf("listing page %d: %w", page, err)
		}
		batch, err := parseListingCards(string(listHTML), base)
		if err != nil {
			return nil, fmt.Errorf("listing page %d: %w", page, err)
		}
		if len(batch) == 0 {
			break
		}
		for _, card := range batch {
			canon, err := utils.CanonicalListingURL(card.jobURL)
			if err != nil {
				return nil, fmt.Errorf("listing URL: %w", err)
			}
			if _, ok := seen[canon]; ok {
				continue
			}
			seen[canon] = struct{}{}
			cards = append(cards, card)
			if len(cards) >= max {
				break
			}
		}
		if len(batch) < jobsPerPageFull {
			break
		}
		page++
	}

	var jobs []schema.Job
	for _, card := range cards {
		if len(jobs) >= max {
			break
		}
		listingURL, err := utils.CanonicalListingURL(card.jobURL)
		if err != nil {
			return nil, fmt.Errorf("listing URL: %w", err)
		}

		c.sleep(ctx)
		detailHTML, err := httpGet(ctx, client, listingURL)
		if err != nil {
			return nil, fmt.Errorf("detail %s: %w", listingURL, err)
		}
		jp, err := parseJobPostingFromDetailHTML(string(detailHTML))
		if err != nil {
			return nil, fmt.Errorf("detail %s: %w", listingURL, err)
		}

		title := strings.TrimSpace(jp.title)
		if title == "" {
			title = card.title
		}
		if strings.TrimSpace(title) == "" {
			return nil, fmt.Errorf("detail %s: empty title", listingURL)
		}
		company := strings.TrimSpace(jp.company)
		if company == "" {
			company = strings.TrimSpace(card.company)
		}
		if company == "" {
			return nil, fmt.Errorf("detail %s: empty company", listingURL)
		}

		descPlain := plainFromJSONLDDescription(jp.description)
		salary := formatSalaryRaw(jp.baseSalaryRaw)
		if salary == "" {
			salary = strings.TrimSpace(card.salaryPreview)
		}

		urlForJob := jp.rawURL
		if strings.TrimSpace(urlForJob) == "" {
			urlForJob = listingURL
		}
		canonURL, err := utils.CanonicalListingURL(urlForJob)
		if err != nil {
			return nil, fmt.Errorf("detail %s: job url: %w", listingURL, err)
		}

		tags := mergeTags(jp.category, jp.ldTags, card.tags)
		remote := resolveRemote(jp.jobLocationType, title, descPlain, tags, card.metaHints)
		cc := countryFromJobPosting(jp, c.Countries)

		postedAt := parsePostedAt(jp.datePosted, c.OnDatePostedWarn)

		j := schema.Job{
			Source:      SourceName,
			Title:       title,
			Company:     company,
			URL:         canonURL,
			ApplyURL:    "",
			Description: descPlain,
			PostedAt:    postedAt,
			Remote:      remote,
			CountryCode: cc,
			SalaryRaw:   salary,
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

// FetchWithSlotSearch implements collectors.SlotSearchFetcher (maps slot query → all_keywords).
func (c *Djinni) FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error) {
	q := strings.TrimSpace(slotQuery)
	if q == "" {
		return c.Fetch(ctx)
	}
	c2 := *c
	c2.AllKeywords = q
	return c2.Fetch(ctx)
}

func (c *Djinni) maxJobsEffective() int {
	n := c.MaxJobs
	if n <= 0 {
		n = DefaultMaxJobs
	}
	if n > maxJobsHardCap {
		n = maxJobsHardCap
	}
	return n
}

func (c *Djinni) sleep(ctx context.Context) {
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

// DefaultSiteBase returns https://djinni.co/ for resolving relative links.
func DefaultSiteBase() (*url.URL, error) {
	return url.Parse("https://djinni.co/")
}

func parsePostedAt(raw string, warn func(string)) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	// Prefer explicit offsets; then Djinni-style local datetime with fractional seconds (no zone → UTC).
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	localLayouts := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range localLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
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

func plainFromJSONLDDescription(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "<") {
		return utils.NormalizePlainText(raw)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + raw + "</body>"))
	if err != nil {
		return utils.NormalizePlainText(raw)
	}
	return utils.NormalizePlainText(doc.Find("body").Text())
}

func httpStatusError(status string, body []byte) error {
	snippet := string(body)
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	return fmt.Errorf("HTTP %s: %s", status, strings.TrimSpace(snippet))
}
