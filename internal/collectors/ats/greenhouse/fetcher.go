// Package greenhouse fetches Greenhouse Job Board API listings for company polls.
package greenhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	companyschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	domainschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

const (
	// SourceName is Job.Source for vacancies from this board.
	SourceName     = "greenhouse"
	defaultBaseURL = "https://boards-api.greenhouse.io"
)

// Client implements companies.CompanyBoardFetcher for Greenhouse.
type Client struct {
	HTTPClient *http.Client
	// BaseURL overrides the API host (tests). Trailing slash is stripped.
	BaseURL string
	// Countries resolves office / location fragments; nil keeps Raw only.
	Countries *utils.CountryResolver
	// MaxJobs caps mapped jobs (0 = unlimited). Hitting the cap → PARTIAL.
	MaxJobs int
}

var _ companies.CompanyBoardFetcher = (*Client)(nil)

// ATS implements companies.CompanyBoardFetcher.
func (*Client) ATS() companyschema.ATS { return companyschema.ATSGreenhouse }

// FetchBoard implements companies.CompanyBoardFetcher.
// slug is sent verbatim (never lowercased / stripped). content=true is always set.
// companyName is the fallback when a job omits company_name.
func (c *Client) FetchBoard(ctx context.Context, slug, companyName, etag string) (companyschema.BoardResult, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return companyschema.BoardResult{}, fmt.Errorf("greenhouse: empty slug")
	}

	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = defaultBaseURL
	}
	// Slug is path-joined verbatim — PathEscape would alter intentional forms.
	reqURL := base + "/v1/boards/" + slug + "/jobs?content=true"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return companyschema.BoardResult{}, err
	}
	utils.SetCollectorUserAgent(req)
	if etag = strings.TrimSpace(etag); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return companyschema.BoardResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	outETag := strings.TrimSpace(resp.Header.Get("ETag"))
	switch resp.StatusCode {
	case http.StatusNotModified:
		_, _ = io.Copy(io.Discard, resp.Body)
		if outETag == "" {
			outETag = etag
		}
		return companyschema.BoardResult{Outcome: companyschema.PollOutcomeNotModified, ETag: outETag}, nil
	case http.StatusNotFound:
		_, _ = io.Copy(io.Discard, resp.Body)
		return companyschema.BoardResult{Outcome: companyschema.PollOutcomeDeadSlug, ETag: outETag}, nil
	case http.StatusOK:
		// decode below
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return companyschema.BoardResult{}, fmt.Errorf("greenhouse: unexpected status %d", resp.StatusCode)
	}

	var env boardEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return companyschema.BoardResult{}, fmt.Errorf("%w: greenhouse decode: %w", companies.ErrBoardFormat, err)
	}

	if len(env.Jobs) == 0 {
		return companyschema.BoardResult{Outcome: companyschema.PollOutcomeEmpty, ETag: outETag}, nil
	}

	partial := env.Meta.Total > 0 && len(env.Jobs) < env.Meta.Total
	jobs := make([]domainschema.Job, 0, len(env.Jobs))
	for i, w := range env.Jobs {
		if c.MaxJobs > 0 && len(jobs) >= c.MaxJobs {
			partial = true
			break
		}
		j, err := jobFromWire(c.Countries, companyName, w)
		if err != nil {
			return companyschema.BoardResult{}, fmt.Errorf("greenhouse: job[%d]: %w", i, err)
		}
		jobs = append(jobs, j)
	}

	outcome := companyschema.PollOutcomeComplete
	if partial {
		outcome = companyschema.PollOutcomePartial
	}
	return companyschema.BoardResult{Jobs: jobs, Outcome: outcome, ETag: outETag}, nil
}
