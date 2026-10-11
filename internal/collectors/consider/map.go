package consider

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

type jobWire struct {
	JobID               string         `json:"jobId"`
	Title               string         `json:"title"`
	CompanyName         string         `json:"companyName"`
	CompanyDomain       string         `json:"companyDomain"`
	URL                 string         `json:"url"`
	ApplyURL            string         `json:"applyUrl"`
	TimeStamp           string         `json:"timeStamp"`
	MinValue            salaryAmount   `json:"minValue"`
	MaxValue            salaryAmount   `json:"maxValue"`
	Currency            string         `json:"currency"`
	Locations           []string       `json:"locations"`
	NormalizedLocations []labeledValue `json:"normalizedLocations"`
	Regions             []labeledValue `json:"regions"`
	Remote              bool           `json:"remote"`
	Hybrid              bool           `json:"hybrid"`
	RequiredSkills      skillsField    `json:"requiredSkills"`
	PreferredSkills     skillsField    `json:"preferredSkills"`
}

type labeledValue struct {
	Label string `json:"label"`
}

type skillWire struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// skillsField is a JSON array of {id,label} or a string containing that JSON.
type skillsField []skillWire

func (s *skillsField) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*s = nil
		return nil
	}
	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			*s = nil
			return nil
		}
		return json.Unmarshal([]byte(raw), s)
	}
	var list []skillWire
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*s = list
	return nil
}

// salaryAmount accepts a JSON number or string. Junk stays unset.
type salaryAmount struct {
	n *float64
}

func (s *salaryAmount) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		s.n = nil
		return nil
	}
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		s.n = &num
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		s.n = nil
		return nil
	}
	str = strings.TrimSpace(str)
	if str == "" {
		s.n = nil
		return nil
	}
	num, err := strconv.ParseFloat(str, 64)
	if err != nil {
		s.n = nil
		return nil
	}
	s.n = &num
	return nil
}

func jobFromWire(source string, countries *utils.CountryResolver, w jobWire) (schema.Job, bool) {
	title := strings.TrimSpace(w.Title)
	company := strings.TrimSpace(w.CompanyName)
	rawURL := strings.TrimSpace(w.URL)
	apply := strings.TrimSpace(w.ApplyURL)
	if title == "" || company == "" || (rawURL == "" && apply == "") {
		return schema.Job{}, false
	}
	if apply == "" {
		apply = rawURL
	}

	desc, tags := skillLines(w.RequiredSkills, w.PreferredSkills)
	hiringCountries, hiringRegions, _ := utils.ParseHiringScope(countries, hiringInputs(w)...)
	j := schema.Job{
		Source:         source,
		Title:          title,
		Company:        company,
		CompanyWebsite: companyWebsite(w.CompanyDomain),
		URL:            rawURL,
		ApplyURL:       apply,
		Description:    desc,
		PostedAt:       postedAt(w.TimeStamp),
		SalaryRaw:      formatSalary(w.MinValue.n, w.MaxValue.n, w.Currency),
		Tags:           tags,
		Location: schema.Location{
			Type:      locationType(w),
			Regions:   hiringRegions,
			Countries: hiringCountries,
			Raw:       joinLocations(w.Locations),
		},
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, false
	}
	return j, true
}

func hiringInputs(w jobWire) []string {
	out := make([]string, 0, len(w.Locations)+len(w.NormalizedLocations)+len(w.Regions))
	out = append(out, w.Locations...)
	for _, n := range w.NormalizedLocations {
		out = append(out, n.Label)
	}
	for _, r := range w.Regions {
		out = append(out, r.Label)
	}
	return out
}

func locationType(w jobWire) string {
	switch {
	case w.Remote:
		return schema.LocationRemote
	case w.Hybrid:
		return schema.LocationHybrid
	case hasLocation(w.Locations):
		return schema.LocationOffice
	default:
		return ""
	}
}

func hasLocation(locations []string) bool {
	for _, s := range locations {
		if strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

func joinLocations(locations []string) string {
	parts := make([]string, 0, len(locations))
	for _, s := range locations {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func companyWebsite(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if u, err := url.Parse(domain); err == nil && u.Scheme != "" && u.Host != "" {
		return domain
	}
	return "https://" + domain
}

func postedAt(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatSalary(min, max *float64, currency string) string {
	if min == nil && max == nil {
		return ""
	}
	var b strings.Builder
	if min != nil {
		b.WriteString(strconv.FormatFloat(*min, 'f', -1, 64))
	}
	b.WriteByte('-')
	if max != nil {
		b.WriteString(strconv.FormatFloat(*max, 'f', -1, 64))
	}
	if cur := strings.TrimSpace(currency); cur != "" {
		b.WriteByte(' ')
		b.WriteString(cur)
	}
	return b.String()
}

// skillLines lists required labels, then preferred labels, one per line.
// Skill id resume:Go also contributes "golang", because the label "Go" does not contain that word.
func skillLines(required, preferred skillsField) (string, []string) {
	var lines []string
	var tags []string
	seenLabel := map[string]struct{}{}
	seenLine := map[string]struct{}{}
	golangAdded := false
	add := func(skills skillsField) {
		for _, sk := range skills {
			if label := strings.TrimSpace(sk.Label); label != "" {
				if _, ok := seenLabel[label]; !ok {
					seenLabel[label] = struct{}{}
					tags = append(tags, label)
					if _, ok := seenLine[label]; !ok {
						seenLine[label] = struct{}{}
						lines = append(lines, label)
					}
				}
			}
			if sk.ID == "resume:Go" && !golangAdded {
				golangAdded = true
				if _, ok := seenLine["golang"]; !ok {
					seenLine["golang"] = struct{}{}
					lines = append(lines, "golang")
				}
			}
		}
	}
	add(required)
	add(preferred)
	return strings.Join(lines, "\n"), tags
}
