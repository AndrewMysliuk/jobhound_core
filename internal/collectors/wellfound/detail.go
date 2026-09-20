package wellfound

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var ldJSONRE = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

// JobDetail is parsed from a job detail page (JSON-LD JobPosting).
type JobDetail struct {
	Title       string
	Company     string
	Description string
	PostedAt    time.Time
	Remote      *bool
}

// ParseJobDetailHTML extracts JobPosting fields from detail HTML.
func ParseJobDetailHTML(html string) (JobDetail, error) {
	jp, err := firstJobPostingFromHTML(html)
	if err != nil {
		return JobDetail{}, err
	}
	var out JobDetail
	out.Title = strings.TrimSpace(jp.Title)
	out.Company = strings.TrimSpace(jp.Company)
	out.Description = strings.TrimSpace(jp.Description)
	if t, err := time.Parse(time.RFC3339Nano, jp.DatePosted); err == nil {
		out.PostedAt = t.UTC()
	} else if t, err := time.Parse(time.RFC3339, jp.DatePosted); err == nil {
		out.PostedAt = t.UTC()
	}
	if strings.EqualFold(strings.TrimSpace(jp.JobLocationType), "TELECOMMUTE") {
		v := true
		out.Remote = &v
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
	return zero, fmt.Errorf("wellfound: JobPosting ld+json not found")
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

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
