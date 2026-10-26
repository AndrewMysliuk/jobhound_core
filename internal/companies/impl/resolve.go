package impl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
)

// atsHit is a recognised board URL parse result.
type atsHit struct {
	ATS      schema.ATS
	Slug     string
	BoardURL string
	Pollable bool
}

// Resolve finds ats + slug for one PENDING company and commits the outcome.
// Step 1 only (this slice): apply URL on the raw host → APPLY_URL; recognised
// non-pollable host → UNSUPPORTED_ATS with next_resolve_at NULL; unknown host →
// follow redirects; nothing → UNRESOLVED with next_resolve_at = now + retryDays.
func (s *Service) Resolve(ctx context.Context, companyKey string) (schema.Company, error) {
	if s == nil || s.companies == nil {
		return schema.Company{}, errors.New("companies: service not configured")
	}
	c, err := s.companies.GetByKey(ctx, companyKey)
	if err != nil {
		return schema.Company{}, err
	}
	if c.ResolutionStatus != schema.ResolutionStatusPending {
		return c, nil
	}

	candidates := uniqueNonEmpty(c.BoardURL, c.Website)
	now := s.now()

	for _, raw := range candidates {
		hit, ok := parseATSURL(raw)
		if ok {
			return s.commitResolve(ctx, c, hit, schema.ResolutionMethodApplyURL)
		}
	}

	// Unknown raw host → follow redirects, then re-parse the final URL.
	for _, raw := range candidates {
		finalURL, err := s.followRedirects(ctx, raw)
		if err != nil || finalURL == "" || finalURL == raw {
			continue
		}
		hit, ok := parseATSURL(finalURL)
		if ok {
			return s.commitResolve(ctx, c, hit, schema.ResolutionMethodApplyURL)
		}
	}

	retry := now.Add(time.Duration(s.cfg.ResolveRetryDays) * 24 * time.Hour)
	if err := s.companies.MarkResolution(ctx, schema.MarkResolutionParams{
		Key:              c.Key,
		ResolutionStatus: schema.ResolutionStatusUnresolved,
		NextResolveAt:    &retry,
	}); err != nil {
		return schema.Company{}, err
	}
	c.ResolutionStatus = schema.ResolutionStatusUnresolved
	c.NextResolveAt = &retry
	return c, nil
}

func (s *Service) commitResolve(ctx context.Context, c schema.Company, hit atsHit, method schema.ResolutionMethod) (schema.Company, error) {
	if !hit.Pollable {
		if err := s.companies.MarkResolution(ctx, schema.MarkResolutionParams{
			Key:              c.Key,
			ResolutionStatus: schema.ResolutionStatusUnsupportedATS,
			ResolutionMethod: method,
			ATS:              hit.ATS,
			ATSSlug:          "",
			BoardURL:         hit.BoardURL,
			NextResolveAt:    nil,
		}); err != nil {
			return schema.Company{}, err
		}
		c.ResolutionStatus = schema.ResolutionStatusUnsupportedATS
		c.ResolutionMethod = method
		c.ATS = hit.ATS
		c.ATSSlug = ""
		c.BoardURL = hit.BoardURL
		c.NextResolveAt = nil
		return c, nil
	}

	slug := hit.Slug
	if err := s.companies.MarkResolution(ctx, schema.MarkResolutionParams{
		Key:              c.Key,
		ResolutionStatus: schema.ResolutionStatusResolved,
		ResolutionMethod: method,
		ATS:              hit.ATS,
		ATSSlug:          slug,
		BoardURL:         hit.BoardURL,
		NextResolveAt:    nil,
	}); err != nil {
		return schema.Company{}, err
	}
	c.ResolutionStatus = schema.ResolutionStatusResolved
	c.ResolutionMethod = method
	c.ATS = hit.ATS
	c.ATSSlug = slug
	c.BoardURL = hit.BoardURL
	c.NextResolveAt = nil
	return c, nil
}

func (s *Service) followRedirects(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	client := s.http
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "JobHoundCompanies/1.0")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()
	if res.Request != nil && res.Request.URL != nil {
		return res.Request.URL.String(), nil
	}
	return raw, nil
}

// parseATSURL matches Greenhouse / Lever / Ashby and recognised non-pollable hosts.
func parseATSURL(raw string) (atsHit, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return atsHit{}, false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return atsHit{}, false
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	segments := splitPath(path)

	if hit, ok := parsePollableHost(host, segments, u); ok {
		return hit, true
	}
	if ats, ok := unsupportedHost(host); ok {
		return atsHit{
			ATS:      ats,
			BoardURL: u.String(),
			Pollable: false,
		}, true
	}
	return atsHit{}, false
}

func parsePollableHost(host string, segments []string, u *url.URL) (atsHit, bool) {
	switch {
	case host == "boards.greenhouse.io" || host == "job-boards.greenhouse.io" ||
		strings.HasSuffix(host, ".greenhouse.io"):
		slug := firstSegment(segments)
		if slug == "" || slug == "embed" {
			return atsHit{}, false
		}
		return atsHit{
			ATS:      schema.ATSGreenhouse,
			Slug:     slug,
			BoardURL: fmt.Sprintf("https://boards.greenhouse.io/%s", slug),
			Pollable: true,
		}, true
	case host == "jobs.lever.co" || host == "lever.co" || strings.HasSuffix(host, ".lever.co"):
		slug := firstSegment(segments)
		if slug == "" {
			return atsHit{}, false
		}
		return atsHit{
			ATS:      schema.ATSLever,
			Slug:     slug,
			BoardURL: fmt.Sprintf("https://jobs.lever.co/%s", slug),
			Pollable: true,
		}, true
	case host == "jobs.ashbyhq.com" || host == "ashbyhq.com" || strings.HasSuffix(host, ".ashbyhq.com"):
		slug := firstSegment(segments)
		if slug == "" {
			return atsHit{}, false
		}
		return atsHit{
			ATS:      schema.ATSAshby,
			Slug:     slug,
			BoardURL: fmt.Sprintf("https://jobs.ashbyhq.com/%s", slug),
			Pollable: true,
		}, true
	default:
		_ = u
		return atsHit{}, false
	}
}

func unsupportedHost(host string) (schema.ATS, bool) {
	switch {
	case strings.Contains(host, "myworkdayjobs.com"), strings.Contains(host, "workday.com"):
		return schema.ATSWorkday, true
	case strings.Contains(host, "teamtailor.com"):
		return schema.ATSTeamtailor, true
	case strings.Contains(host, "workable.com"):
		return schema.ATSWorkable, true
	case strings.Contains(host, "recruitee.com"):
		return schema.ATSRecruitee, true
	case strings.Contains(host, "personio.de"), strings.Contains(host, "personio.com"):
		return schema.ATSPersonio, true
	case strings.Contains(host, "rippling.com"):
		return schema.ATSRippling, true
	case strings.Contains(host, "smartrecruiters.com"):
		return schema.ATSSmartRecruiters, true
	default:
		return "", false
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstSegment(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	return segments[0]
}
