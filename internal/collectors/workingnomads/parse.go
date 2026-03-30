package workingnomads

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

const listingURLPrefix = "https://www.workingnomads.com/jobs/"

// jobSource mirrors _source from jobsapi/_search (resources/working-nomads.md).
type jobSource struct {
	ID               int64    `json:"id"`
	Title            string   `json:"title"`
	Slug             string   `json:"slug"`
	Company          string   `json:"company"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	AllTags          []string `json:"all_tags"`
	Locations        []string `json:"locations"`
	LocationBase     string   `json:"location_base"`
	PubDate          string   `json:"pub_date"`
	ApplyOption      string   `json:"apply_option"`
	ApplyEmail       string   `json:"apply_email"`
	ApplyURL         string   `json:"apply_url"`
	Expired          bool     `json:"expired"`
	SalaryRange      string   `json:"salary_range"`
	SalaryRangeShort string   `json:"salary_range_short"`
}

type searchHit struct {
	Source jobSource `json:"_source"`
}

type searchResponse struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []searchHit `json:"hits"`
	} `json:"hits"`
}

func jobFromSource(countries *utils.CountryResolver, src jobSource) (domain.Job, error) {
	if src.Expired {
		return domain.Job{}, errSkipHit
	}
	slug := strings.TrimSpace(src.Slug)
	if slug == "" {
		return domain.Job{}, fmt.Errorf("working nomads: empty slug")
	}
	rawListing := listingURLPrefix + slug
	listingURL, err := utils.CanonicalListingURL(rawListing)
	if err != nil {
		return domain.Job{}, fmt.Errorf("listing URL: %w", err)
	}
	applyURL, err := applyURLForWN(&src)
	if err != nil {
		return domain.Job{}, err
	}
	postedAt, err := parsePubDate(strings.TrimSpace(src.PubDate))
	if err != nil {
		return domain.Job{}, fmt.Errorf("pub_date: %w", err)
	}
	descPlain := utils.StripHTMLToPlainText(src.Description)
	tags := boardTags(src)
	title := strings.TrimSpace(src.Title)
	company := strings.TrimSpace(src.Company)
	j := domain.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		ApplyURL:    applyURL,
		Description: descPlain,
		PostedAt:    postedAt,
		Remote:      utils.RemoteMVPRule(title, descPlain, tags),
		CountryCode: countryFromWN(countries, src),
		SalaryRaw:   salaryRawWN(src),
		Tags:        tags,
		Position:    utils.InferPosition(title, descPlain, tags),
	}
	if err := domain.AssignStableID(&j); err != nil {
		return domain.Job{}, fmt.Errorf("stable id: %w", err)
	}
	return j, nil
}

var errSkipHit = errors.New("working nomads: skip hit")

func boardTags(src jobSource) []string {
	if len(src.Tags) > 0 {
		out := make([]string, 0, len(src.Tags))
		for _, t := range src.Tags {
			t = strings.TrimSpace(t)
			if t != "" {
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	var out []string
	for _, t := range src.AllTags {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyURLForWN(src *jobSource) (string, error) {
	opt := strings.TrimSpace(src.ApplyOption)
	switch opt {
	case "with_your_ats":
		return strings.TrimSpace(src.ApplyURL), nil
	case "with_email":
		email := strings.TrimSpace(src.ApplyEmail)
		if email == "" {
			return "", nil
		}
		return "mailto:" + email, nil
	case "with_our_ats":
		return strings.TrimSpace(src.ApplyURL), nil
	default:
		return "", fmt.Errorf("working nomads: unknown apply_option %q", opt)
	}
}

func parsePubDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty pub_date")
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse %q", s)
}

func salaryRawWN(src jobSource) string {
	a := strings.TrimSpace(src.SalaryRange)
	b := strings.TrimSpace(src.SalaryRangeShort)
	switch {
	case a != "" && b != "" && a != b:
		return a + " | " + b
	case a != "":
		return a
	default:
		return b
	}
}

func countryFromWN(r *utils.CountryResolver, src jobSource) string {
	if r == nil {
		return ""
	}
	for _, loc := range src.Locations {
		for _, part := range strings.Split(loc, ",") {
			part = strings.TrimSpace(part)
			if code := r.Alpha2ForName(part); code != "" {
				return code
			}
		}
	}
	for _, part := range strings.Split(src.LocationBase, ",") {
		part = strings.TrimSpace(part)
		if code := r.Alpha2ForName(part); code != "" {
			return code
		}
	}
	return ""
}
