// Package dou implements the DOU.ua vacancies collector (specs/005-job-collectors/resources/dou.md).
package dou

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// SourceName is the normative Job.Source value (contracts/collector.md).
const SourceName = "dou_ua"

// DefaultMaxJobs caps jobs returned per Fetch when MaxJobs <= 0 (resources/dou.md).
const DefaultMaxJobs = 100

const maxJobsHardCap = 500

type xhrEnvelope struct {
	HTML string `json:"html"`
	Last bool   `json:"last"`
	Num  int    `json:"num"`
}

// DOU fetches listings via GET + POST xhr-load and full descriptions via GET detail pages.
type DOU struct {
	HTTPClient *http.Client
	// Search is the vacancies search string (query param search); required non-empty.
	Search string
	// ListingBaseURL overrides https://jobs.dou.ua/ for tests; nil uses DefaultSiteBase().
	ListingBase *url.URL
	// MaxJobs caps collected jobs after detail fetches; <= 0 uses DefaultMaxJobs.
	MaxJobs int
	// InterRequestDelay is applied between consecutive HTTP calls (listing, xhr-load, detail); zero skips.
	InterRequestDelay time.Duration
	Countries         *utils.CountryResolver
	Now               func() time.Time
	OnDateWarn        func(raw string)
}

// Name implements collectors.Collector.
func (*DOU) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *DOU) Fetch(ctx context.Context) ([]schema.Job, error) {
	search := strings.TrimSpace(c.Search)
	if search == "" {
		return nil, fmt.Errorf("dou: empty search")
	}
	base := c.ListingBase
	if base == nil {
		u, err := DefaultSiteBase()
		if err != nil {
			return nil, err
		}
		base = u
	}
	nowFn := c.Now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClientWithJar()
	}

	listingPageURL := buildListingURL(base, search)
	xhrLoadURL := buildXHRLoadURL(base, search)

	c.sleep(ctx)
	listingHTML, err := httpGet(ctx, client, listingPageURL)
	if err != nil {
		return nil, fmt.Errorf("listing: %w", err)
	}
	listingStr := string(listingHTML)
	csrf := extractCSRFToken(listingStr)
	if csrf == "" {
		return nil, fmt.Errorf("dou: missing csrfmiddlewaretoken on listing")
	}

	listDoc, err := goquery.NewDocumentFromReader(strings.NewReader(listingStr))
	if err != nil {
		return nil, fmt.Errorf("listing document: %w", err)
	}
	cards, err := parseListingCardsFromDocument(listDoc, base)
	if err != nil {
		return nil, fmt.Errorf("listing cards: %w", err)
	}

	ordered, seen, err := dedupeOrderedCards(cards, base)
	if err != nil {
		return nil, err
	}

	max := c.maxJobsEffective()
	last := false
	numShown := len(ordered)

	for !last && len(ordered) < max {
		c.sleep(ctx)
		form := url.Values{}
		form.Set("csrfmiddlewaretoken", csrf)
		form.Set("count", strconv.Itoa(numShown))
		body, err := postXHRLoad(ctx, client, xhrLoadURL, listingPageURL, form)
		if err != nil {
			return nil, fmt.Errorf("xhr-load: %w", err)
		}
		var env xhrEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("xhr-load json: %w", err)
		}
		fragCards, err := parseListingCards(env.HTML, base)
		if err != nil {
			return nil, fmt.Errorf("xhr-load cards: %w", err)
		}
		added := 0
		for _, card := range fragCards {
			canon, err := utils.CanonicalListingURL(card.jobPageURL)
			if err != nil {
				return nil, fmt.Errorf("listing URL: %w", err)
			}
			if _, ok := seen[canon]; ok {
				continue
			}
			seen[canon] = struct{}{}
			ordered = append(ordered, card)
			added++
			if len(ordered) >= max {
				break
			}
		}
		last = env.Last
		if env.Num > 0 {
			numShown = env.Num
		} else {
			numShown = len(ordered)
		}
		if added == 0 && !last {
			last = true
		}
	}

	var jobs []schema.Job
	for _, card := range ordered {
		if len(jobs) >= max {
			break
		}
		listingURL, err := utils.CanonicalListingURL(card.jobPageURL)
		if err != nil {
			return nil, fmt.Errorf("listing URL: %w", err)
		}

		c.sleep(ctx)
		detailHTML, err := httpGet(ctx, client, listingURL)
		if err != nil {
			return nil, fmt.Errorf("detail %s: %w", listingURL, err)
		}
		detail, err := parseJobDetailHTML(string(detailHTML), base)
		if err != nil {
			return nil, fmt.Errorf("detail %s: %w", listingURL, err)
		}

		title := strings.TrimSpace(detail.title)
		if title == "" {
			title = strings.TrimSpace(card.title)
		}
		if title == "" {
			return nil, fmt.Errorf("detail %s: empty title", listingURL)
		}
		company := strings.TrimSpace(card.company)
		if company == "" {
			return nil, fmt.Errorf("detail %s: empty company", listingURL)
		}

		warn := func(raw string) {
			if c.OnDateWarn != nil {
				c.OnDateWarn(raw)
			}
		}
		postedAt := resolvePostedAt(nowFn(), card.postedDisplay, detail.postedDisplay, warn)

		j := schema.Job{
			Source:      SourceName,
			Title:       title,
			Company:     company,
			URL:         listingURL,
			ApplyURL:    "",
			Description: detail.description,
			PostedAt:    postedAt,
			Remote:      utils.RemoteMVPRule(title, detail.description, detail.tags, card.locationRaw, detail.locationRaw),
			CountryCode: c.countryCode(card.locationRaw, detail.locationRaw),
			SalaryRaw:   detail.salaryRaw,
			Tags:        detail.tags,
			Position:    utils.InferPosition(title, detail.description, detail.tags),
		}
		if err := domainutils.AssignStableID(&j); err != nil {
			return nil, fmt.Errorf("stable id: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (c *DOU) maxJobsEffective() int {
	n := c.MaxJobs
	if n <= 0 {
		n = DefaultMaxJobs
	}
	if n > maxJobsHardCap {
		n = maxJobsHardCap
	}
	return n
}

func (c *DOU) countryCode(listingLoc, detailLoc string) string {
	if c.Countries == nil {
		return ""
	}
	for _, part := range splitLocationParts(listingLoc, detailLoc) {
		if code := c.Countries.Alpha2ForName(part); code != "" {
			return code
		}
	}
	return ""
}

func splitLocationParts(a, b string) []string {
	var out []string
	for _, s := range []string{a, b} {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func dedupeOrderedCards(cards []listingCard, base *url.URL) ([]listingCard, map[string]struct{}, error) {
	seen := make(map[string]struct{})
	var ordered []listingCard
	for _, card := range cards {
		canon, err := utils.CanonicalListingURL(card.jobPageURL)
		if err != nil {
			return nil, nil, fmt.Errorf("listing URL: %w", err)
		}
		if _, ok := seen[canon]; ok {
			continue
		}
		seen[canon] = struct{}{}
		ordered = append(ordered, card)
	}
	return ordered, seen, nil
}

// DefaultSiteBase returns https://jobs.dou.ua/ for resolving relative links.
func DefaultSiteBase() (*url.URL, error) {
	return url.Parse("https://jobs.dou.ua/")
}

func buildListingURL(base *url.URL, search string) string {
	u := *base
	u.Path = "/vacancies/"
	q := url.Values{}
	q.Set("search", search)
	q.Set("descr", "1")
	u.RawQuery = q.Encode()
	return u.String()
}

func buildXHRLoadURL(base *url.URL, search string) string {
	u := *base
	u.Path = "/vacancies/xhr-load/"
	q := url.Values{}
	q.Set("search", search)
	q.Set("descr", "1")
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *DOU) sleep(ctx context.Context) {
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

func postXHRLoad(ctx context.Context, client *http.Client, xhrURL, referer string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xhrURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", "https://jobs.dou.ua")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
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
