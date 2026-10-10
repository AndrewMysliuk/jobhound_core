// Package europeremotely implements the Europe Remotely job board collector (specs/005-job-collectors).
package europeremotely

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
const SourceName = "europe_remotely"

// feedEnvelope matches the AJAX JSON shape (resources/europe-remotely.md).
type feedEnvelope struct {
	HasMore bool   `json:"has_more"`
	HTML    string `json:"html"`
}

// DefaultMaxFeedPages is the default cap on AJAX listing pages (≈60 jobs/page on the live site).
// Further pages need explicit MaxFeedPages on the collector (still bounded by maxFeedPagesHardCap).
const DefaultMaxFeedPages = 2

// maxFeedPagesHardCap bounds MaxFeedPages when set above this value (safety rail).
const maxFeedPagesHardCap = 500

// EuropeRemotely fetches listings via POST (admin-ajax-style) and job pages via GET.
type EuropeRemotely struct {
	HTTPClient *http.Client
	// FeedURL is the full URL for the feed POST; empty is invalid (use DefaultFeedURL from bootstrap or tests).
	FeedURL string
	// FeedForm is base application/x-www-form-urlencoded fields; "page" is overwritten each batch.
	FeedForm url.Values
	// MaxFeedPages caps AJAX listing pages (page=1..N). 0 means DefaultMaxFeedPages.
	// Stops after this many pages even if has_more is still true.
	MaxFeedPages int
	// MaxJobs stops after this many jobs are collected (after each detail fetch). 0 = unlimited.
	MaxJobs int
	// SiteBase resolves relative links in listing fragments; nil uses DefaultSiteBase().
	SiteBase  *url.URL
	Countries *utils.CountryResolver
	// Now anchors relative "posted" parsing; defaults to time.Now().UTC if nil.
	Now func() time.Time
	// OnDateWarn is called when posted_display cannot be parsed (soft failure per domain-mapping-mvp.md).
	OnDateWarn func(raw string)
	// StartErr is returned by Fetch when homepage nonce discovery failed at process start.
	// Other collectors still run; this source fails on its own ingest.
	StartErr error
}

// Name implements collectors.Collector.
func (*EuropeRemotely) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *EuropeRemotely) Fetch(ctx context.Context) ([]schema.Job, error) {
	if c.StartErr != nil {
		return nil, c.StartErr
	}
	if strings.TrimSpace(c.FeedURL) == "" {
		return nil, fmt.Errorf("europe remotely: empty FeedURL")
	}
	if c.FeedForm == nil {
		return nil, fmt.Errorf("europe remotely: nil FeedForm")
	}
	base := c.SiteBase
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
		client = utils.NewHTTPClient()
	}

	limit := c.maxFeedPagesEffective()

	seen := make(map[string]struct{})
	var jobs []schema.Job
	for page := 1; page <= limit; page++ {
		form := cloneValues(c.FeedForm)
		form.Set("page", strconv.Itoa(page))
		body, err := postForm(ctx, client, c.FeedURL, form)
		if err != nil {
			return nil, fmt.Errorf("feed page %d: %w", page, err)
		}
		hasMore, htmlFrag, err := decodeFeedEnvelope(body)
		if err != nil {
			return nil, fmt.Errorf("feed page %d: %w", page, err)
		}
		cards, err := parseListingCards(htmlFrag, base)
		if err != nil {
			return nil, fmt.Errorf("feed page %d: %w", page, err)
		}
		for _, card := range cards {
			listingURL, err := utils.CanonicalListingURL(card.jobPageURL)
			if err != nil {
				return nil, fmt.Errorf("listing URL: %w", err)
			}
			if _, ok := seen[listingURL]; ok {
				continue
			}
			seen[listingURL] = struct{}{}

			detailHTML, err := httpGet(ctx, client, listingURL)
			if err != nil {
				if ctx.Err() != nil {
					return nil, err
				}
				skipDetail(ctx, "detail fetch", listingURL, err)
				continue
			}
			detail, err := parseJobDetailHTML(string(detailHTML), base)
			if err != nil {
				skipDetail(ctx, "detail parse", listingURL, err)
				continue
			}

			title := strings.TrimSpace(detail.title)
			if title == "" {
				title = strings.TrimSpace(card.title)
			}
			if title == "" {
				skipDetail(ctx, "detail empty title", listingURL, fmt.Errorf("empty title"))
				continue
			}
			company := strings.TrimSpace(detail.company)
			if company == "" {
				company = strings.TrimSpace(card.company)
			}
			if company == "" {
				skipDetail(ctx, "detail empty company", listingURL, fmt.Errorf("empty company"))
				continue
			}

			warn := func(raw string) {
				if c.OnDateWarn != nil {
					c.OnDateWarn(raw)
				}
			}
			postedAt := resolvePostedAt(nowFn(), card.postedDisplay, detail.postedDisplay, warn)

			applyURL := detail.applyURL
			if nu, err := utils.NormalizeApplyURL(applyURL); err == nil {
				applyURL = nu
			}
			if applyURL != "" {
				if missing, err := utils.RecruiteeJobPageMissing(ctx, client, applyURL); err == nil && missing {
					applyURL = ""
				}
			}

			hiringCountries, hiringRegions, hiringRaw := utils.ParseHiringScope(c.Countries, card.locationRaw, detail.locationRaw)
			if len(hiringCountries) == 0 && len(hiringRegions) == 0 {
				hiringRegions = []string{schema.RegionCodeEurope.String()}
			}
			j := schema.Job{
				Source:         SourceName,
				Title:          title,
				Company:        company,
				CompanyWebsite: strings.TrimSpace(detail.companyWebsite),
				URL:            listingURL,
				ApplyURL:       applyURL,
				Description:    detail.description,
				PostedAt:       postedAt,
				Location:       utils.LocationFromParsed(utils.RemoteMVPRule(title, detail.description, detail.tags), "", hiringCountries, hiringRegions, hiringRaw, nil),
				SalaryRaw:      salaryRaw(card.compensation, detail.compensationRaw),
				Tags:           detail.tags,
				Position:       utils.InferPosition(title, detail.description, detail.tags),
			}
			if err := domainutils.AssignStableID(&j); err != nil {
				return nil, fmt.Errorf("stable id: %w", err)
			}
			jobs = append(jobs, j)
			if c.MaxJobs > 0 && len(jobs) >= c.MaxJobs {
				return jobs, nil
			}
		}
		if !hasMore {
			break
		}
	}
	return jobs, nil
}

// FetchWithQuery implements collectors.QueryFetcher (admin-ajax search_keywords).
func (c *EuropeRemotely) FetchWithQuery(ctx context.Context, slotQuery string) ([]schema.Job, error) {
	q := strings.TrimSpace(slotQuery)
	if q == "" {
		return c.Fetch(ctx)
	}
	if strings.TrimSpace(c.FeedURL) == "" || c.FeedForm == nil {
		return c.Fetch(ctx)
	}
	c2 := *c
	form := cloneValues(c.FeedForm)
	form.Set("search_keywords", q)
	c2.FeedForm = form
	return c2.Fetch(ctx)
}

func skipDetail(ctx context.Context, op, listingURL string, err error) {
	slog.WarnContext(ctx, "europe_remotely: fetch skip", "source", SourceName, "op", op, "url", listingURL, "err", err)
}

func (c *EuropeRemotely) maxFeedPagesEffective() int {
	n := c.MaxFeedPages
	if n <= 0 {
		n = DefaultMaxFeedPages
	}
	if n > maxFeedPagesHardCap {
		n = maxFeedPagesHardCap
	}
	return n
}

func cloneValues(v url.Values) url.Values {
	if v == nil {
		return url.Values{}
	}
	out := make(url.Values, len(v))
	for k, vv := range v {
		out[k] = append([]string(nil), vv...)
	}
	return out
}

// CloneFeedForm returns a deep copy of form values (e.g. before mutating for a one-off debug request).
func CloneFeedForm(v url.Values) url.Values {
	return cloneValues(v)
}

func decodeFeedEnvelope(b []byte) (hasMore bool, html string, err error) {
	var probe struct {
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return false, "", fmt.Errorf("decode feed json: %w", err)
	}
	if probe.Success != nil {
		var wrapped struct {
			Success bool `json:"success"`
			Data    struct {
				HasMore bool   `json:"has_more"`
				HTML    string `json:"html"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &wrapped); err != nil {
			return false, "", fmt.Errorf("decode feed json: %w", err)
		}
		if !wrapped.Success {
			return false, "", fmt.Errorf("feed ajax: success=false")
		}
		return wrapped.Data.HasMore, wrapped.Data.HTML, nil
	}
	var env feedEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return false, "", fmt.Errorf("decode feed json: %w", err)
	}
	return env.HasMore, env.HTML, nil
}

func postForm(ctx context.Context, client *http.Client, rawURL string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	utils.SetCollectFormPostHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
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
