package competitionline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

type listHit struct {
	ID int `json:"id"`
}

func parseList(body []byte) ([]listHit, string, error) {
	var page struct {
		Next json.RawMessage `json:"next"`
		Hits []listHit       `json:"hits"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", fmt.Errorf("competitionline: list json: %w", err)
	}
	next, err := nextPage(page.Next)
	if err != nil {
		return nil, "", err
	}
	return page.Hits, next, nil
}

func nextPage(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("competitionline: list next: %w", err)
	}
	return strings.TrimSpace(s), nil
}

func parseDetail(body []byte, id int) (schema.Job, error) {
	var row struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Country         string  `json:"country"`
		City            string  `json:"city"`
		CompanyName     string  `json:"company_name"`
		PublicationDate string  `json:"publication_date"`
		DirectApply     *string `json:"directApply"`
	}
	if err := json.Unmarshal(body, &row); err != nil {
		return schema.Job{}, fmt.Errorf("competitionline: detail %d: %w", id, err)
	}
	title := strings.TrimSpace(row.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("competitionline: detail %d: empty title", id)
	}
	listing, err := listingURL(id)
	if err != nil {
		return schema.Job{}, err
	}
	var countries []string
	if code := countryCode(row.Country); code != "" {
		countries = []string{code}
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(row.CompanyName),
		URL:         listing,
		ApplyURL:    httpApplyURL(row.DirectApply),
		Description: utils.StripHTMLToPlainText(row.Description),
		PostedAt:    parsePosted(row.PublicationDate),
		Location: schema.Location{
			Countries: countries,
			Raw:       strings.TrimSpace(row.City),
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func listingURL(id int) (string, error) {
	return utils.CanonicalListingURL(siteOrigin + "/de/jobs/" + strconv.Itoa(id))
}

func countryCode(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "deutschland", "germany":
		return "DE"
	case "österreich", "oesterreich", "austria":
		return "AT"
	case "schweiz", "switzerland", "suisse":
		return "CH"
	}
	s := strings.TrimSpace(name)
	if len(s) == 2 {
		return strings.ToUpper(s)
	}
	return ""
}

func httpApplyURL(raw *string) string {
	if raw == nil {
		return ""
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return s
}

func parsePosted(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
