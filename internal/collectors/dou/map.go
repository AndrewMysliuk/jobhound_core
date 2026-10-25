package dou

import (
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func jobFromFeedAndDetail(item feedItem, detail *detailFields) (schema.Job, error) {
	rawURL := strings.TrimSpace(item.Link)
	if rawURL == "" {
		rawURL = strings.TrimSpace(item.GUID)
	}
	listingURL, err := domainutils.NormalizeListingURL(rawURL)
	if err != nil {
		return schema.Job{}, fmt.Errorf("listing URL: %w", err)
	}

	title := strings.TrimSpace(item.Title)
	company := ""
	description := utils.StripHTMLToPlainText(item.Description)
	placeRaw := ""
	applyURL := ""
	postedAt := item.PubDate

	if detail != nil {
		if t := strings.TrimSpace(detail.Title); t != "" {
			title = t
		}
		company = strings.TrimSpace(detail.Company)
		if d := strings.TrimSpace(detail.Description); d != "" {
			description = d
		}
		placeRaw = strings.TrimSpace(detail.PlaceRaw)
		applyURL = strings.TrimSpace(detail.ApplyURL)
		if postedAt.IsZero() && !detail.PostedAt.IsZero() {
			postedAt = detail.PostedAt
		}
	}

	title = utils.NormalizePlainText(title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("empty title for %s", listingURL)
	}
	description = utils.NormalizePlainText(description)

	remote := utils.RemoteMVPRule(title, description, nil, placeRaw)
	j := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		ApplyURL:    applyURL,
		Description: description,
		PostedAt:    postedAt.UTC(),
		Location:    utils.LocationFromParsed(remote, "", nil, nil, placeRaw, nil),
		Position:    utils.InferPosition(title, description, nil),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, err
	}
	return j, nil
}
