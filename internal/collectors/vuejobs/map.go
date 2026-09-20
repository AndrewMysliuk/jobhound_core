package vuejobs

import (
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func jobsFromListing(rows []ListingJob) ([]schema.Job, error) {
	var out []schema.Job
	for _, row := range rows {
		if row.PROLocked {
			continue
		}
		j, ok, err := jobFromListing(row)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, j)
		}
	}
	return out, nil
}

func jobFromListing(row ListingJob) (schema.Job, bool, error) {
	title := strings.TrimSpace(row.Title)
	company := strings.TrimSpace(row.Company)
	if title == "" || company == "" {
		return schema.Job{}, false, nil
	}
	slug := strings.TrimSpace(row.Slug)
	if slug == "" {
		return schema.Job{}, false, nil
	}

	listingURL, err := utils.CanonicalListingURL(JobCardURL(slug))
	if err != nil {
		return schema.Job{}, false, err
	}

	applyRaw := strings.TrimSpace(row.ApplyURL)
	if applyRaw == "" {
		applyRaw = strings.TrimSpace(row.SourceURL)
	}
	var applyURL string
	if applyRaw != "" {
		applyURL, err = utils.CanonicalListingURL(applyRaw)
		if err != nil {
			return schema.Job{}, false, err
		}
	}

	descPlain := utils.StripHTMLToPlainText(row.Description)
	postedAt, _ := parsePublishedAt(row.PublishedAt)

	j := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		ApplyURL:    applyURL,
		Description: descPlain,
		PostedAt:    postedAt,
		Remote:      remoteFromWorkPlace(row.WorkPlace, title, descPlain),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, false, err
	}
	return j, true, nil
}

func parsePublishedAt(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000000Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, nil
}

func remoteFromWorkPlace(workPlace []string, title, desc string) *bool {
	for _, w := range workPlace {
		if strings.EqualFold(strings.TrimSpace(w), "remote") {
			v := true
			return &v
		}
	}
	return utils.RemoteMVPRule(title, desc, nil)
}
