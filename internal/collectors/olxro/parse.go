package olxro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var ofertaPath = regexp.MustCompile(`^/oferta/loc-de-munca/.+-ID[^/]+\.html$`)

type listPage struct {
	Ads        []listAd
	TotalPages int
}

type listAd struct {
	ID           string
	Title        string
	Description  string
	URL          string
	SearchReason string
	Posted       time.Time
	Workplace    string
	Salary       string
}

func parseListHTML(body []byte) (listPage, error) {
	state, err := prerenderedState(body)
	if err != nil {
		return listPage{}, err
	}
	return parseListState(state)
}

func parseListState(state json.RawMessage) (listPage, error) {
	var root struct {
		Listing struct {
			Listing json.RawMessage `json:"listing"`
		} `json:"listing"`
	}
	if err := json.Unmarshal(state, &root); err != nil {
		return listPage{}, fmt.Errorf("olx_ro: state: %w", err)
	}
	if len(bytes.TrimSpace(root.Listing.Listing)) == 0 {
		return listPage{}, fmt.Errorf("olx_ro: listing.listing missing")
	}
	var listing struct {
		Ads        json.RawMessage `json:"ads"`
		TotalPages int             `json:"totalPages"`
	}
	if err := json.Unmarshal(root.Listing.Listing, &listing); err != nil {
		return listPage{}, fmt.Errorf("olx_ro: listing: %w", err)
	}
	adsRaw := bytes.TrimSpace(listing.Ads)
	if len(adsRaw) == 0 {
		return listPage{}, fmt.Errorf("olx_ro: ads missing")
	}
	if bytes.Equal(adsRaw, []byte("null")) {
		return listPage{TotalPages: listing.TotalPages}, nil
	}
	var rawAds []json.RawMessage
	if err := json.Unmarshal(adsRaw, &rawAds); err != nil {
		return listPage{}, fmt.Errorf("olx_ro: ads: %w", err)
	}
	ads := make([]listAd, 0, len(rawAds))
	for _, raw := range rawAds {
		ad, ok, err := decodeAd(raw)
		if err != nil {
			return listPage{}, err
		}
		if ok {
			ads = append(ads, ad)
		}
	}
	return listPage{Ads: ads, TotalPages: listing.TotalPages}, nil
}

type adWire struct {
	ID           json.RawMessage `json:"id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	URL          string          `json:"url"`
	URLPath      string          `json:"urlPath"`
	SearchReason string          `json:"searchReason"`
	CreatedTime  string          `json:"createdTime"`
	DatePosted   string          `json:"datePosted"`
	JobCharacter string          `json:"job_character"`
	Salary       json.RawMessage `json:"salary"`
	Params       []paramWire     `json:"params"`
}

type paramWire struct {
	Key             string          `json:"key"`
	Value           json.RawMessage `json:"value"`
	NormalizedValue string          `json:"normalizedValue"`
}

func decodeAd(raw json.RawMessage) (listAd, bool, error) {
	var wire adWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return listAd{}, false, fmt.Errorf("olx_ro: ad: %w", err)
	}
	id, ok := numericID(wire.ID)
	if !ok {
		return listAd{}, false, nil
	}
	return listAd{
		ID:           id,
		Title:        strings.TrimSpace(wire.Title),
		Description:  utils.StripHTMLToPlainText(wire.Description),
		URL:          ofertaURL(wire),
		SearchReason: strings.TrimSpace(wire.SearchReason),
		Posted:       postedFromAd(wire),
		Workplace:    workplace(jobCharacter(wire)),
		Salary:       salaryFrom(wire),
	}, true, nil
}

func numericID(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || raw[0] == '"' {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err != nil {
		return "", false
	}
	if _, err := n.Int64(); err != nil {
		f, ferr := n.Float64()
		if ferr != nil || f != math.Trunc(f) {
			return "", false
		}
		return strconv.FormatInt(int64(f), 10), true
	}
	return n.String(), true
}

func ofertaURL(wire adWire) string {
	if listing, ok := canonicalOferta(wire.URL); ok {
		return listing
	}
	if listing, ok := canonicalOferta(wire.URLPath); ok {
		return listing
	}
	return ""
}

func canonicalOferta(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(strings.ToLower(raw), "/api/") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "www.olx.ro"
	}
	host := strings.ToLower(u.Host)
	if host != "www.olx.ro" && host != "olx.ro" {
		return "", false
	}
	if strings.Contains(u.Path, "/d/oferta/") || !ofertaPath.MatchString(u.Path) {
		return "", false
	}
	u.Scheme = "https"
	u.Host = "www.olx.ro"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return "", false
	}
	return listing, true
}

func jobCharacter(wire adWire) string {
	if s := strings.TrimSpace(wire.JobCharacter); s != "" {
		return s
	}
	for _, p := range wire.Params {
		if p.Key != "job_character" {
			continue
		}
		if s := strings.TrimSpace(p.NormalizedValue); s != "" {
			return s
		}
		return jsonString(p.Value)
	}
	return ""
}

func salaryFrom(wire adWire) string {
	if s := formatSalaryRaw(wire.Salary); s != "" {
		return s
	}
	for _, p := range wire.Params {
		if p.Key != "salary" {
			continue
		}
		if s := formatSalaryRaw(p.Value); s != "" {
			return s
		}
	}
	return ""
}

func formatSalaryRaw(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return ""
	}
	var box struct {
		From         *float64 `json:"from"`
		To           *float64 `json:"to"`
		CurrencyCode string   `json:"currencyCode"`
	}
	if err := json.Unmarshal(raw, &box); err != nil {
		return ""
	}
	return formatSalary(box.From, box.To, box.CurrencyCode)
}

func formatSalary(from, to *float64, currency string) string {
	if from == nil && to == nil {
		return ""
	}
	var b strings.Builder
	switch {
	case from != nil && to != nil:
		b.WriteString(formatAmount(*from))
		b.WriteByte('-')
		b.WriteString(formatAmount(*to))
	case from != nil:
		b.WriteString(formatAmount(*from))
	default:
		b.WriteString(formatAmount(*to))
	}
	if cur := strings.TrimSpace(currency); cur != "" {
		b.WriteByte(' ')
		b.WriteString(cur)
	}
	return b.String()
}

func formatAmount(n float64) string {
	if n == math.Trunc(n) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func postedFromAd(wire adWire) time.Time {
	if t := parsePosted(wire.CreatedTime); !t.IsZero() {
		return t
	}
	return parsePosted(wire.DatePosted)
}

func workplace(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "can-work-remote":
		return schema.LocationRemote
	case "hybrid":
		return schema.LocationHybrid
	case "stable-location-wanted", "need-to-travel":
		return schema.LocationOffice
	default:
		return ""
	}
}

func parsePosted(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func jsonString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func prerenderedState(html []byte) (json.RawMessage, error) {
	idx := bytes.Index(html, []byte("__PRERENDERED_STATE__"))
	if idx < 0 {
		return nil, fmt.Errorf("olx_ro: __PRERENDERED_STATE__ not found")
	}
	rest := html[idx+len("__PRERENDERED_STATE__"):]
	eq := bytes.IndexByte(rest, '=')
	if eq < 0 {
		return nil, fmt.Errorf("olx_ro: __PRERENDERED_STATE__ has no value")
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(rest[eq+1:])))
	var encoded string
	if err := dec.Decode(&encoded); err != nil {
		return nil, fmt.Errorf("olx_ro: prerendered state: %w", err)
	}
	var state json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &state); err != nil {
		return nil, fmt.Errorf("olx_ro: prerendered state: %w", err)
	}
	state = bytes.TrimSpace(state)
	if len(state) > 0 && state[0] == '"' {
		var inner string
		if err := json.Unmarshal(state, &inner); err != nil {
			return nil, fmt.Errorf("olx_ro: prerendered state: %w", err)
		}
		state = bytes.TrimSpace([]byte(inner))
	}
	if len(state) == 0 || state[0] != '{' {
		return nil, fmt.Errorf("olx_ro: prerendered state is not an object")
	}
	return state, nil
}

func parseDetailHTML(body []byte, ad listAd) (schema.Job, error) {
	state, err := prerenderedState(body)
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(ad.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("olx_ro: detail %s: empty title", ad.URL)
	}
	posted := ad.Posted
	if posted.IsZero() {
		posted = parsePosted(detailDatePosted(body))
	}
	var countries []string
	if country := countryFromDetail(body); country != "" {
		countries = []string{country}
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     companyName(state),
		URL:         ad.URL,
		Description: ad.Description,
		PostedAt:    posted,
		SalaryRaw:   ad.Salary,
		Location: schema.Location{
			Type:      ad.Workplace,
			Countries: countries,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func companyName(state json.RawMessage) string {
	return findCompany(bytes.TrimSpace(state))
}

func findCompany(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		for _, item := range items {
			if name := findCompany(item); name != "" {
				return name
			}
		}
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return ""
		}
		if box, ok := obj["postingComponent"]; ok {
			var posting struct {
				CompanyName string `json:"companyName"`
			}
			if err := json.Unmarshal(box, &posting); err == nil {
				if name := strings.TrimSpace(posting.CompanyName); name != "" {
					return name
				}
			}
		}
		for _, v := range obj {
			if name := findCompany(v); name != "" {
				return name
			}
		}
	}
	return ""
}

func countryFromDetail(body []byte) string {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "olx_ro")
	if err != nil {
		return ""
	}
	return countryFromLocation(jp.JobLocation)
}

func detailDatePosted(body []byte) string {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "olx_ro")
	if err != nil {
		return ""
	}
	return jp.DatePosted
}

func countryFromLocation(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '[' {
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return ""
		}
		for _, part := range parts {
			if country := countryFromLocation(part); country != "" {
				return country
			}
		}
		return ""
	}
	var place struct {
		Address struct {
			AddressCountry json.RawMessage `json:"addressCountry"`
		} `json:"address"`
	}
	if err := json.Unmarshal(raw, &place); err != nil {
		return ""
	}
	return countryCode(place.Address.AddressCountry)
}

func countryCode(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return normalizeCountry(text)
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return normalizeCountry(obj.Name)
	}
	return ""
}

func normalizeCountry(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ro", "romania", "românia":
		return "RO"
	default:
		s := strings.TrimSpace(raw)
		if len(s) == 2 {
			return strings.ToUpper(s)
		}
		return s
	}
}
