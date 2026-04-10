package himalayas

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

type jobWire struct {
	Title                string    `json:"title"`
	Excerpt              string    `json:"excerpt"`
	CompanyName          string    `json:"companyName"`
	EmploymentType       string    `json:"employmentType"`
	MinSalary            *float64  `json:"minSalary"`
	MaxSalary            *float64  `json:"maxSalary"`
	Currency             string    `json:"currency"`
	Seniority            []string  `json:"seniority"`
	LocationRestrictions []string  `json:"locationRestrictions"`
	TimezoneRestrictions []float64 `json:"timezoneRestrictions"`
	Categories           []string  `json:"categories"`
	ParentCategories     []string  `json:"parentCategories"`
	Description          string    `json:"description"`
	PubDate              float64   `json:"pubDate"`
	ApplicationLink      string    `json:"applicationLink"`
	GUID                 string    `json:"guid"`
}

type apiEnvelope struct {
	Jobs       []jobWire `json:"jobs"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
	TotalCount int       `json:"totalCount"`
}

func jobFromWire(cr *utils.CountryResolver, w jobWire, onPubDateWarn func(raw float64)) (schema.Job, error) {
	rawListing := strings.TrimSpace(w.GUID)
	if rawListing == "" {
		rawListing = strings.TrimSpace(w.ApplicationLink)
	}
	if rawListing == "" {
		return schema.Job{}, fmt.Errorf("himalayas: missing guid and applicationLink")
	}
	listingURL, err := utils.CanonicalListingURL(rawListing)
	if err != nil {
		return schema.Job{}, fmt.Errorf("himalayas: listing URL: %w", err)
	}

	title := strings.TrimSpace(w.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("himalayas: empty title for %s", listingURL)
	}
	company := strings.TrimSpace(w.CompanyName)
	if company == "" {
		return schema.Job{}, fmt.Errorf("himalayas: empty companyName for %s", listingURL)
	}

	descPlain := utils.StripHTMLToPlainText(w.Description)
	tags := mergeTagsDedupe(w.Categories, w.Seniority, w.ParentCategories)
	locJoined := strings.Join(trimStringSlice(w.LocationRestrictions), ", ")
	remote := utils.RemoteMVPRule(title, descPlain, tags, strings.TrimSpace(w.Excerpt), locJoined)

	var postedAt time.Time
	switch {
	case w.PubDate <= 0:
		if onPubDateWarn != nil {
			onPubDateWarn(w.PubDate)
		}
	default:
		sec := int64(w.PubDate)
		if float64(sec) != w.PubDate {
			if onPubDateWarn != nil {
				onPubDateWarn(w.PubDate)
			}
			break
		}
		postedAt = time.Unix(sec, 0).UTC()
	}

	var tz []float64
	if len(w.TimezoneRestrictions) > 0 {
		tz = append([]float64(nil), w.TimezoneRestrictions...)
	}

	j := schema.Job{
		Source:          SourceName,
		Title:           title,
		Company:         company,
		URL:             listingURL,
		ApplyURL:        "",
		Description:     descPlain,
		PostedAt:        postedAt,
		Remote:          remote,
		CountryCode:     countryFromRestrictions(cr, w.LocationRestrictions),
		SalaryRaw:       formatSalaryRaw(w.MinSalary, w.MaxSalary, w.Currency),
		Tags:            tags,
		TimezoneOffsets: tz,
		Position:        utils.InferPosition(title, descPlain, tags),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, err
	}
	return j, nil
}

func mergeTagsDedupe(categories, seniority, parent []string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, group := range [][]string{categories, seniority, parent} {
		for _, s := range group {
			t := strings.TrimSpace(s)
			if t == "" {
				continue
			}
			k := strings.ToLower(t)
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

func trimStringSlice(in []string) []string {
	var out []string
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func countryFromRestrictions(cr *utils.CountryResolver, restrictions []string) string {
	for _, raw := range restrictions {
		for _, part := range strings.Split(raw, ",") {
			code := cr.Alpha2ForName(part)
			if code != "" {
				return code
			}
		}
	}
	return ""
}

func formatSalaryRaw(min, max *float64, currency string) string {
	if min == nil && max == nil {
		return ""
	}
	cur := strings.TrimSpace(currency)
	switch {
	case min != nil && max != nil:
		return fmt.Sprintf("%g-%g %s", *min, *max, cur)
	case min != nil:
		return fmt.Sprintf("%g+ %s", *min, cur)
	default:
		return fmt.Sprintf("up to %g %s", *max, cur)
	}
}
