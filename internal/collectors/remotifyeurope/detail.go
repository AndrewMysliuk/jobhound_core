package remotifyeurope

import (
	"context"
	"encoding/json"
	"fmt"
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
	listingIDRE = regexp.MustCompile(`/listing/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)
	jobLinkRE   = regexp.MustCompile(`"job_link"\s*:\s*"((?:\\.|[^"\\])*)"`)
	ldJSONRE    = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
)

// ListingDetail is parsed from a GET /listing/{id} HTML page.
type ListingDetail struct {
	Title       string
	Company     string
	Description string
	JobLink     string
	PostedAt    time.Time
	Remote      *bool
	RemoteKnown bool
}

// ParseListingDetailHTML extracts JSON-LD JobPosting fields and job_link from listing HTML.
func ParseListingDetailHTML(html string) (ListingDetail, error) {
	var out ListingDetail
	jp, err := firstJobPostingFromHTML(html)
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
	if link, ok := jobLinkFromHTML(html); ok {
		out.JobLink = link
	}
	return out, nil
}

type jobPostingWire struct {
	Type            json.RawMessage `json:"@type"`
	Title           string          `json:"title"`
	Company         string          `json:"-"`
	Description     string          `json:"description"`
	DatePosted      string          `json:"datePosted"`
	JobLocationType string          `json:"jobLocationType"`
	HiringOrg       json.RawMessage `json:"hiringOrganization"`
}

func firstJobPostingFromHTML(html string) (jobPostingWire, error) {
	var zero jobPostingWire
	for _, m := range ldJSONRE.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			continue
		}
		jp, ok, err := decodeJobPostingJSON([]byte(raw))
		if err != nil {
			return zero, err
		}
		if ok {
			return jp, nil
		}
	}
	return zero, fmt.Errorf("remotify europe: JobPosting ld+json not found")
}

func decodeJobPostingJSON(raw []byte) (jobPostingWire, bool, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 {
		return jobPostingWire{}, false, nil
	}
	switch raw[0] {
	case '{':
		var w jobPostingWire
		if err := json.Unmarshal(raw, &w); err != nil {
			return jobPostingWire{}, false, err
		}
		if wireIsJobPosting(w.Type) {
			w.Company = hiringOrgName(w.HiringOrg)
			return w, true, nil
		}
		var wrap struct {
			Graph []json.RawMessage `json:"@graph"`
		}
		if err := json.Unmarshal(raw, &wrap); err == nil {
			for _, block := range wrap.Graph {
				jp, ok, err := decodeJobPostingJSON(block)
				if err != nil {
					return jobPostingWire{}, false, err
				}
				if ok {
					return jp, true, nil
				}
			}
		}
	case '[':
		var blocks []json.RawMessage
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return jobPostingWire{}, false, err
		}
		for _, block := range blocks {
			jp, ok, err := decodeJobPostingJSON(block)
			if err != nil {
				return jobPostingWire{}, false, err
			}
			if ok {
				return jp, true, nil
			}
		}
	}
	return jobPostingWire{}, false, nil
}

func hiringOrgName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var o struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &o); err == nil {
		return strings.TrimSpace(o.Name)
	}
	return ""
}

func wireIsJobPosting(atType json.RawMessage) bool {
	if len(atType) == 0 {
		return false
	}
	var s string
	if err := json.Unmarshal(atType, &s); err == nil {
		return strings.EqualFold(strings.TrimSpace(s), "JobPosting")
	}
	var arr []string
	if err := json.Unmarshal(atType, &arr); err == nil {
		for _, v := range arr {
			if strings.EqualFold(strings.TrimSpace(v), "JobPosting") {
				return true
			}
		}
	}
	return false
}

func jobLinkFromHTML(html string) (string, bool) {
	m := jobLinkRE.FindStringSubmatch(html)
	if len(m) < 2 {
		return "", false
	}
	return unescapeJSONString(m[1]), true
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
	defer resp.Body.Close()
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

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
