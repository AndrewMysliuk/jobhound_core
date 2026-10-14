package bestjobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var nextDataRE = regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

var foldDiacritics = strings.NewReplacer(
	"ă", "a", "â", "a", "î", "i", "ș", "s", "ț", "t",
	"Ă", "A", "Â", "A", "Î", "I", "Ș", "S", "Ț", "T",
	"ş", "s", "ţ", "t", "Ş", "S", "Ţ", "T",
)

type listPage struct {
	Items      []listCard
	NextCursor *string
	Filters    infiniteParams
}

type listCard struct {
	Slug string `json:"slug"`
}

type infiniteParams struct {
	ObjFilterParams map[string]string `json:"objFilterParams"`
	FetchURLParams  string            `json:"fetchUrlParams"`
}

type nextPayload struct {
	Props struct {
		PageProps struct {
			InfiniteQueryParams    infiniteParams `json:"infiniteQueryParams"`
			JobListCardsFromServer struct {
				Items      []listCard `json:"items"`
				NextCursor *string    `json:"nextCursor"`
			} `json:"jobListCardsFromServer"`
			Job *detailJob `json:"job"`
		} `json:"pageProps"`
	} `json:"props"`
}

type detailJob struct {
	Title         string           `json:"title"`
	Description   string           `json:"description"`
	Salary        flexString       `json:"salary"`
	Remote        bool             `json:"remote"`
	PartialRemote bool             `json:"partialRemote"`
	OwnApplyURL   flexString       `json:"ownApplyUrl"`
	Employer      detailEmployer   `json:"employer"`
	Locations     []detailLocation `json:"locations"`
}

type detailEmployer struct {
	EmployerName string `json:"employerName"`
}

type detailLocation struct {
	Name    string `json:"name"`
	Tooltip string `json:"tooltip"`
}

type flexString string

func (s *flexString) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	var text string
	if err := json.Unmarshal(b, &text); err != nil {
		return nil
	}
	*s = flexString(strings.TrimSpace(text))
	return nil
}

func parseListHTML(html []byte) (listPage, error) {
	payload, err := decodeNextData(html)
	if err != nil {
		return listPage{}, err
	}
	cards := payload.Props.PageProps.JobListCardsFromServer
	return listPage{
		Items:      cards.Items,
		NextCursor: cards.NextCursor,
		Filters:    payload.Props.PageProps.InfiniteQueryParams,
	}, nil
}

func parseJobsAPI(body []byte) (listPage, error) {
	var page struct {
		Items      []listCard `json:"items"`
		NextCursor *string    `json:"nextCursor"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return listPage{}, fmt.Errorf("bestjobs: decode jobs api: %w", err)
	}
	return listPage{Items: page.Items, NextCursor: page.NextCursor}, nil
}

func parseDetailHTML(html []byte, countries *utils.CountryResolver, listing string) (schema.Job, error) {
	payload, err := decodeNextData(html)
	if err != nil {
		return schema.Job{}, err
	}
	job := payload.Props.PageProps.Job
	if job == nil {
		return schema.Job{}, fmt.Errorf("bestjobs: detail %s: missing job", listing)
	}
	title := strings.TrimSpace(job.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("bestjobs: detail %s: empty title", listing)
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(job.Employer.EmployerName),
		URL:         listing,
		ApplyURL:    httpApplyURL(string(job.OwnApplyURL)),
		Description: utils.StripHTMLToPlainText(job.Description),
		Location:    locationOf(countries, job.Remote, job.PartialRemote, job.Locations),
		SalaryRaw:   string(job.Salary),
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func decodeNextData(html []byte) (nextPayload, error) {
	match := nextDataRE.FindSubmatch(html)
	if len(match) < 2 {
		return nextPayload{}, fmt.Errorf("bestjobs: __NEXT_DATA__ script not found")
	}
	var payload nextPayload
	if err := json.Unmarshal(match[1], &payload); err != nil {
		return nextPayload{}, fmt.Errorf("bestjobs: decode __NEXT_DATA__: %w", err)
	}
	return payload, nil
}

// filterValues is the first-page filter query with _lat and _lon removed.
// An empty result means the page did not show filters, so the caller must not
// invent a keyword-only API request.
func filterValues(p infiniteParams) url.Values {
	v := url.Values{}
	if raw := strings.TrimPrefix(strings.TrimSpace(p.FetchURLParams), "?"); raw != "" {
		if parsed, err := url.ParseQuery(raw); err == nil {
			v = parsed
		}
	}
	for key, value := range p.ObjFilterParams {
		value = strings.TrimSpace(value)
		if value == "" || v.Has(key) {
			continue
		}
		v.Set(key, value)
	}
	v.Del("_lat")
	v.Del("_lon")
	v.Del("limit")
	v.Del("cursor")
	if len(v) == 0 {
		return nil
	}
	return v
}

func locationOf(countries *utils.CountryResolver, remote, partial bool, locs []detailLocation) schema.Location {
	rawParts := make([]string, 0, len(locs))
	inputs := make([]string, 0, len(locs))
	for _, loc := range locs {
		name := strings.TrimSpace(loc.Name)
		tip := strings.TrimSpace(loc.Tooltip)
		switch {
		case tip != "":
			rawParts = append(rawParts, tip)
			inputs = append(inputs, foldDiacritics.Replace(tip))
		case name != "":
			rawParts = append(rawParts, name)
			inputs = append(inputs, foldDiacritics.Replace(name))
		}
	}
	hiringCountries, regions, _ := utils.ParseHiringScope(countries, inputs...)
	return schema.Location{
		Type:      workplace(remote, partial),
		Countries: hiringCountries,
		Regions:   regions,
		Raw:       strings.Join(rawParts, "; "),
	}
}

func workplace(remote, partial bool) string {
	if remote {
		return schema.LocationRemote
	}
	if partial {
		return schema.LocationHybrid
	}
	return schema.LocationOffice
}

func httpApplyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return raw
}
