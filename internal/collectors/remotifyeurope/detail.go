package remotifyeurope

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

const fallbackListingIDCap = 24

var (
	listingIDRE    = regexp.MustCompile(`/listing/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)
	jobLinkPlainRE = regexp.MustCompile(`"job_link"\s*:\s*"((?:\\.|[^"\\])*)"`)
	jobLinkEscRE   = regexp.MustCompile(`\\"job_link\\"\s*:\s*\\"((?:\\.|[^\\])*?)\\"`)
	sitePlainRE    = regexp.MustCompile(`"company_website"\s*:\s*"((?:\\.|[^"\\])*)"`)
	siteEscRE      = regexp.MustCompile(`\\"company_website\\"\s*:\s*\\"((?:\\.|[^\\])*?)\\"`)
)

// ListingDetail is parsed from a GET /listing/{id} HTML page.
type ListingDetail struct {
	Title          string
	Company        string
	Description    string
	JobLink        string
	CompanyWebsite string
	PostedAt       time.Time
	Remote         *bool
	RemoteKnown    bool
	LocationTexts  []string
	HeaderPin      string
}

// ParseListingDetailHTML extracts JSON-LD JobPosting fields and job_link from listing HTML.
func ParseListingDetailHTML(html string) (ListingDetail, error) {
	var out ListingDetail
	jp, err := utils.FirstJobPostingFromHTML(html, "remotify europe")
	if err != nil {
		return out, err
	}
	out.Title = jp.Title
	out.Company = jp.Company
	out.Description = jp.Description
	if t, err := time.Parse(time.RFC3339Nano, jp.DatePosted); err == nil {
		out.PostedAt = t.UTC()
	} else if t, err := time.Parse(time.RFC3339, jp.DatePosted); err == nil {
		out.PostedAt = t.UTC()
	}
	if strings.EqualFold(strings.TrimSpace(jp.JobLocationType), "TELECOMMUTE") {
		v := true
		out.Remote = &v
		out.RemoteKnown = true
	}
	if link, ok := embeddedJSONString(html, jobLinkPlainRE, jobLinkEscRE); ok {
		out.JobLink = strings.TrimSpace(link)
	}
	if site, ok := embeddedJSONString(html, sitePlainRE, siteEscRE); ok {
		out.CompanyWebsite = httpWebsite(site)
	}
	locStrings := append([]string(nil), utils.LocationStringsFromJSONLD(jp.ApplicantLocationRequirements)...)
	locStrings = append(locStrings, utils.LocationStringsFromJSONLD(jp.JobLocation)...)
	out.LocationTexts = locStrings
	out.HeaderPin = headerLocationPin(html)
	return out, nil
}

const headerLocationPinPath = "M12 2C8.13 2 5 5.13 5 9"

var headerPinTextRE = regexp.MustCompile(`<p class="text-sm">([^<]*)</p>`)

func headerLocationPin(page string) string {
	idx := strings.Index(page, headerLocationPinPath)
	if idx < 0 {
		return ""
	}
	rest := page[idx:]
	svgEnd := strings.Index(rest, "</svg>")
	if svgEnd < 0 {
		return ""
	}
	window := rest[svgEnd+len("</svg>"):]
	if next := strings.Index(window, "<svg"); next >= 0 {
		window = window[:next]
	}
	var parts []string
	for _, m := range headerPinTextRE.FindAllStringSubmatch(window, -1) {
		t := strings.TrimSpace(html.UnescapeString(m[1]))
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ", ")
}

func embeddedJSONString(html string, plain, escaped *regexp.Regexp) (string, bool) {
	if m := plain.FindStringSubmatch(html); len(m) >= 2 {
		return unescapeJSONString(m[1]), true
	}
	if m := escaped.FindStringSubmatch(html); len(m) >= 2 {
		return unescapeJSONString(m[1]), true
	}
	return "", false
}

func httpWebsite(raw string) string {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return s
	}
	return ""
}

func unescapeJSONString(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case '"', '\\', '/':
			b.WriteByte(s[i])
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			if i+4 >= len(s) {
				b.WriteByte('u')
				continue
			}
			hex := s[i+1 : i+5]
			i += 4
			cp, err := strconv.ParseUint(hex, 16, 16)
			if err != nil {
				continue
			}
			b.WriteRune(rune(cp))
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func listingIDsFromHTML(ctx context.Context, client *http.Client, listingURL string) ([]string, error) {
	if client == nil {
		client = utils.NewHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listingURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remotify europe: fallback listing page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remotify europe: fallback listing page: HTTP %s", resp.Status)
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, m := range listingIDRE.FindAllStringSubmatch(string(b), -1) {
		if len(m) < 2 {
			continue
		}
		id := strings.ToLower(m[1])
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		if len(ids) >= fallbackListingIDCap {
			break
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("remotify europe: no listing ids in fallback HTML")
	}
	return ids, nil
}
