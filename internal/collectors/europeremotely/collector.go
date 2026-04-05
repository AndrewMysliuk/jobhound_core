// Package europeremotely implements the Europe Remotely job board collector (specs/005-job-collectors).
package europeremotely

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

// maxFeedPages caps AJAX pagination to avoid an infinite loop if has_more stays true.
const maxFeedPages = 500

// EuropeRemotely fetches listings via POST (admin-ajax-style) and job pages via GET.
type EuropeRemotely struct {
	HTTPClient *http.Client
	// FeedURL is the full URL for the feed POST; empty is invalid (use DefaultFeedURL from bootstrap or tests).
	FeedURL string
	// FeedForm is base application/x-www-form-urlencoded fields; "page" is overwritten each batch.
	FeedForm url.Values
	// MaxJobs stops after this many jobs are collected (after each detail fetch). 0 = unlimited.
	MaxJobs int
	// SiteBase resolves relative links in listing fragments; nil uses DefaultSiteBase().
	SiteBase  *url.URL
	Countries *utils.CountryResolver
	// Now anchors relative "posted" parsing; defaults to time.Now().UTC if nil.
	Now func() time.Time
	// OnDateWarn is called when posted_display cannot be parsed (soft failure per domain-mapping-mvp.md).
	OnDateWarn func(raw string)
}

// Name implements collectors.Collector.
func (*EuropeRemotely) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *EuropeRemotely) Fetch(ctx context.Context) ([]schema.Job, error) {
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

	seen := make(map[string]struct{})
	var jobs []schema.Job
	page := 1
	for {
		if page > maxFeedPages {
			return nil, fmt.Errorf("feed page limit exceeded (%d)", maxFeedPages)
		}
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
			company := strings.TrimSpace(detail.company)
			if company == "" {
				company = strings.TrimSpace(card.company)
			}
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
				ApplyURL:    detail.applyURL,
				Description: detail.description,
				PostedAt:    postedAt,
				Remote:      utils.RemoteMVPRule(title, detail.description, detail.tags),
				CountryCode: c.countryCode(card.locationRaw, detail.locationRaw),
				SalaryRaw:   salaryRaw(card.compensation, detail.compensationRaw),
				Tags:        detail.tags,
				Position:    utils.InferPosition(title, detail.description, detail.tags),
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
		page++
	}
	return jobs, nil
}

func (c *EuropeRemotely) countryCode(listingLoc, detailLoc string) string {
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
