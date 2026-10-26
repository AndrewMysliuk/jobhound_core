// Package lever fetches Lever Postings API listings for company polls.
package lever

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	companyschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	domainschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

const (
	// SourceName is Job.Source for vacancies from this board.
	SourceName      = "lever"
	defaultBaseURL  = "https://api.lever.co"
	defaultPageSize = 100
)

// Client implements companies.CompanyBoardFetcher for Lever.
type Client struct {
	HTTPClient *http.Client
	// BaseURL overrides the API host (tests). Trailing slash is stripped.
	BaseURL   string
	Countries *utils.CountryResolver
	// PageSize is the Lever limit per request (0 → defaultPageSize).
	PageSize int
	// MaxJobs caps mapped jobs (0 = unlimited). Hitting the cap → PARTIAL.
	MaxJobs int
	// MaxPages caps pagination rounds (0 = unlimited). Hitting the cap mid-board → PARTIAL.
	MaxPages int
}

var _ companies.CompanyBoardFetcher = (*Client)(nil)

// ATS implements companies.CompanyBoardFetcher.
func (*Client) ATS() companyschema.ATS { return companyschema.ATSLever }

// FetchBoard implements companies.CompanyBoardFetcher.
// slug is sent verbatim. Paginated via skip/limit until a short page or MaxPages/MaxJobs.
// companyName is required — postings have no company field.
func (c *Client) FetchBoard(ctx context.Context, slug, companyName, etag string) (companyschema.BoardResult, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return companyschema.BoardResult{}, fmt.Errorf("lever: empty slug")
	}
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return companyschema.BoardResult{}, fmt.Errorf("%w: lever: empty companyName", companies.ErrBoardFormat)
	}

	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = defaultBaseURL
	}
	pageSize := c.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	var (
		all       []domainschema.Job
		outETag   string
		partial   bool
		pages     int
		skip      int
		firstPage = true
	)

	for {
		if c.MaxPages > 0 && pages >= c.MaxPages {
			partial = true
			break
		}
		if c.MaxJobs > 0 && len(all) >= c.MaxJobs {
			partial = true
			break
		}

		pageETag := ""
		if firstPage {
			pageETag = etag
		}
		wire, respETag, outcome, done, err := c.fetchPage(ctx, client, base, slug, pageETag, skip, pageSize)
		if err != nil {
			return companyschema.BoardResult{}, err
		}
		if firstPage {
			outETag = respETag
			firstPage = false
			switch outcome {
			case companyschema.PollOutcomeNotModified, companyschema.PollOutcomeDeadSlug:
				return companyschema.BoardResult{Outcome: outcome, ETag: outETag}, nil
			}
		}
		if done {
			break
		}
		pages++

		for i, w := range wire {
			if c.MaxJobs > 0 && len(all) >= c.MaxJobs {
				partial = true
				break
			}
			j, err := jobFromWire(c.Countries, companyName, w)
			if err != nil {
				return companyschema.BoardResult{}, fmt.Errorf("lever: job[%d]: %w", skip+i, err)
			}
			all = append(all, j)
		}
		if partial {
			break
		}
		if len(wire) < pageSize {
			break
		}
		skip += pageSize
	}

	if len(all) == 0 && !partial {
		return companyschema.BoardResult{Outcome: companyschema.PollOutcomeEmpty, ETag: outETag}, nil
	}
	outcome := companyschema.PollOutcomeComplete
	if partial {
		outcome = companyschema.PollOutcomePartial
	}
	return companyschema.BoardResult{Jobs: all, Outcome: outcome, ETag: outETag}, nil
}

// fetchPage returns decoded jobs, response etag, early outcome (304/404), and whether
// the caller should stop without treating the body as a job page.
func (c *Client) fetchPage(
	ctx context.Context,
	client *http.Client,
	base, slug, etag string,
	skip, limit int,
) (jobs []jobWire, outETag string, outcome companyschema.PollOutcome, early bool, err error) {
	u, err := url.Parse(base + "/v0/postings/" + slug)
	if err != nil {
		return nil, "", "", false, err
	}
	q := u.Query()
	q.Set("mode", "json")
	q.Set("skip", strconv.Itoa(skip))
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", "", false, err
	}
	utils.SetCollectorUserAgent(req)
	if etag = strings.TrimSpace(etag); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	outETag = strings.TrimSpace(resp.Header.Get("ETag"))
	switch resp.StatusCode {
	case http.StatusNotModified:
		_, _ = io.Copy(io.Discard, resp.Body)
		if outETag == "" {
			outETag = etag
		}
		return nil, outETag, companyschema.PollOutcomeNotModified, true, nil
	case http.StatusNotFound:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, outETag, companyschema.PollOutcomeDeadSlug, true, nil
	case http.StatusOK:
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, "", "", false, fmt.Errorf("lever: unexpected status %d", resp.StatusCode)
	}

	var wire []jobWire
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, "", "", false, fmt.Errorf("%w: lever decode: %w", companies.ErrBoardFormat, err)
	}
	return wire, outETag, "", false, nil
}
