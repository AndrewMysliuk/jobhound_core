package ejobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

type listPage struct {
	Cards    []listCard
	Count    int
	Jobs     []schema.Job
	FromList bool
}

type listCard struct {
	Title string
	URL   string
}

func parseListHTML(body []byte) (listPage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return listPage{}, fmt.Errorf("ejobs: list html: %w", err)
	}
	if node := doc.Find("#__NUXT_DATA__").First(); node.Length() > 0 {
		page, ok, err := listFromNuxt([]byte(node.Text()))
		if err != nil {
			return listPage{}, err
		}
		if ok {
			return page, nil
		}
	}
	var found *listPage
	var scanErr error
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		if found != nil || scanErr != nil {
			return
		}
		kind, _ := s.Attr("type")
		if !strings.Contains(strings.ToLower(kind), "ld+json") {
			return
		}
		page, ok, err := decodeItemList([]byte(s.Text()))
		if err != nil {
			scanErr = err
			return
		}
		if ok {
			found = &page
		}
	})
	if scanErr != nil {
		return listPage{}, scanErr
	}
	if found == nil {
		if cloudflareBlocked(body) {
			return listPage{}, fmt.Errorf("ejobs: cloudflare blocked")
		}
		return listPage{}, fmt.Errorf("ejobs: ItemList ld+json not found")
	}
	return *found, nil
}

func listFromNuxt(raw []byte) (listPage, bool, error) {
	root, err := decodeNuxtPayload(raw)
	if err != nil {
		return listPage{}, false, err
	}
	jobs, ok := nuxtMap(root, "pinia", "jobs")
	if !ok {
		return listPage{}, false, nil
	}
	items, ok := jobs["_listItems"].([]any)
	if !ok {
		return listPage{}, false, nil
	}
	parsed, err := jobsFromNuxt(items)
	if err != nil {
		return listPage{}, false, err
	}
	count := len(parsed)
	if next, ok := jobs["_nextBtnVisible"].(bool); ok {
		if next {
			count = pageSize
		} else if count >= pageSize {
			count = pageSize - 1
		}
	}
	return listPage{Jobs: parsed, Count: count, FromList: true}, true, nil
}

func jobsFromNuxt(items []any) ([]schema.Job, error) {
	var out []schema.Job
	seen := make(map[string]struct{})
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, ok := nuxtInt(obj["id"])
		if !ok || id <= 0 {
			continue
		}
		title, _ := obj["title"].(string)
		slug, _ := obj["slug"].(string)
		title = strings.TrimSpace(title)
		slug = strings.Trim(strings.TrimSpace(slug), "/")
		if title == "" || slug == "" || strings.ContainsAny(slug, `/\?#`) {
			continue
		}
		link := siteOrigin + "/user/locuri-de-munca/" + url.PathEscape(slug) + "/" + strconv.FormatInt(id, 10)
		listing, ok := canonicalListing(link)
		if !ok {
			continue
		}
		if _, dup := seen[listing]; dup {
			continue
		}
		seen[listing] = struct{}{}
		city, countries := nuxtPlace(obj["locations"])
		job := schema.Job{
			Source:      SourceName,
			Title:       title,
			Company:     nuxtCompany(obj["company"]),
			URL:         listing,
			Description: "",
			PostedAt:    nuxtPosted(obj["creationDate"]),
			SalaryRaw:   nuxtText(obj["salary"]),
			Location: schema.Location{
				Raw:       city,
				Countries: countries,
			},
		}
		if err := domainutils.AssignStableID(&job); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func nuxtCompany(v any) string {
	switch c := v.(type) {
	case string:
		return strings.TrimSpace(c)
	case map[string]any:
		return nuxtText(c["name"])
	default:
		return ""
	}
}

func nuxtText(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func nuxtPlace(v any) (string, []string) {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return "", nil
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return "", nil
	}
	addr := nuxtText(first["address"])
	if addr == "" {
		return "", nil
	}
	city := addr
	var countries []string
	if i := strings.LastIndex(addr, ","); i >= 0 {
		tail := strings.TrimSpace(addr[i+1:])
		if strings.EqualFold(tail, "România") || strings.EqualFold(tail, "Romania") {
			city = strings.TrimSpace(addr[:i])
			countries = []string{"RO"}
		}
	}
	return city, countries
}

func nuxtPosted(v any) time.Time {
	switch n := v.(type) {
	case string:
		return parsePosted(n)
	case int64:
		return unixPosted(n)
	case float64:
		return unixPosted(int64(n))
	default:
		return time.Time{}
	}
}

func unixPosted(n int64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	if n > 1_000_000_000_000 {
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}

func nuxtMap(root any, keys ...string) (map[string]any, bool) {
	cur := root
	for _, key := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur = obj[key]
	}
	obj, ok := cur.(map[string]any)
	return obj, ok
}

func nuxtInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func decodeNuxtPayload(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	dec.UseNumber()
	var slots []any
	if err := dec.Decode(&slots); err != nil {
		return nil, fmt.Errorf("ejobs: nuxt payload: %w", err)
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("ejobs: nuxt payload: empty")
	}
	cache := make([]any, len(slots))
	ready := make([]bool, len(slots))
	var walk func(int) (any, error)
	walk = func(i int) (any, error) {
		if i < 0 || i >= len(slots) {
			return nil, fmt.Errorf("ejobs: nuxt payload: index %d", i)
		}
		if ready[i] {
			return cache[i], nil
		}
		switch v := slots[i].(type) {
		case nil, string, bool:
			ready[i] = true
			cache[i] = v
			return v, nil
		case json.Number:
			ready[i] = true
			if parsed, err := v.Int64(); err == nil {
				cache[i] = parsed
				return parsed, nil
			}
			parsed, err := v.Float64()
			if err != nil {
				return nil, fmt.Errorf("ejobs: nuxt payload: %w", err)
			}
			cache[i] = parsed
			return parsed, nil
		case []any:
			if inner, ok := nuxtReviver(v, len(slots)); ok {
				ready[i] = true
				if inner < 0 {
					return nil, nil
				}
				val, err := walk(inner)
				if err != nil {
					return nil, err
				}
				cache[i] = val
				return val, nil
			}
			arr := make([]any, len(v))
			cache[i] = arr
			ready[i] = true
			for j, el := range v {
				idx, ok := nuxtIndex(el, len(slots))
				if !ok {
					arr[j] = el
					continue
				}
				val, err := walk(idx)
				if err != nil {
					return nil, err
				}
				arr[j] = val
			}
			return arr, nil
		case map[string]any:
			obj := make(map[string]any, len(v))
			cache[i] = obj
			ready[i] = true
			for key, el := range v {
				idx, ok := nuxtIndex(el, len(slots))
				if !ok {
					obj[key] = el
					continue
				}
				val, err := walk(idx)
				if err != nil {
					return nil, err
				}
				obj[key] = val
			}
			return obj, nil
		default:
			ready[i] = true
			cache[i] = v
			return v, nil
		}
	}
	return walk(0)
}

func nuxtReviver(v []any, n int) (int, bool) {
	if len(v) == 0 || len(v) > 2 {
		return 0, false
	}
	name, ok := v[0].(string)
	if !ok || !nuxtReviverName(name) {
		return 0, false
	}
	if len(v) == 1 {
		return -1, true
	}
	idx, ok := nuxtIndex(v[1], n)
	if !ok {
		return 0, false
	}
	return idx, true
}

func nuxtReviverName(name string) bool {
	switch name {
	case "EmptyShallowRef", "EmptyRef", "ShallowRef", "ShallowReactive", "Ref", "Reactive", "NuxtError":
		return true
	default:
		return false
	}
}

func nuxtIndex(v any, n int) (int, bool) {
	num, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := num.Int64()
	if err != nil || i < 0 || i >= int64(n) {
		return 0, false
	}
	return int(i), true
}

func decodeItemList(raw []byte) (listPage, bool, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return listPage{}, false, nil
	}
	switch raw[0] {
	case '[':
		var blocks []json.RawMessage
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return listPage{}, false, fmt.Errorf("ejobs: list json: %w", err)
		}
		for _, block := range blocks {
			page, ok, err := decodeItemList(block)
			if err != nil || ok {
				return page, ok, err
			}
		}
		return listPage{}, false, nil
	case '{':
		var doc struct {
			Type            json.RawMessage   `json:"@type"`
			NumberOfItems   *int              `json:"numberOfItems"`
			ItemListElement []json.RawMessage `json:"itemListElement"`
			Graph           []json.RawMessage `json:"@graph"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return listPage{}, false, fmt.Errorf("ejobs: list json: %w", err)
		}
		if jsonTypeIs(doc.Type, "ItemList") {
			cards, err := cardsFromElements(doc.ItemListElement)
			if err != nil {
				return listPage{}, false, err
			}
			count := len(cards)
			if doc.NumberOfItems != nil {
				count = *doc.NumberOfItems
			}
			return listPage{Cards: cards, Count: count}, true, nil
		}
		for _, block := range doc.Graph {
			page, ok, err := decodeItemList(block)
			if err != nil || ok {
				return page, ok, err
			}
		}
		return listPage{}, false, nil
	default:
		return listPage{}, false, nil
	}
}

func cardsFromElements(raw []json.RawMessage) ([]listCard, error) {
	var cards []listCard
	seen := make(map[string]struct{})
	for _, block := range raw {
		card, ok, err := cardFromElement(block)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, dup := seen[card.URL]; dup {
			continue
		}
		seen[card.URL] = struct{}{}
		cards = append(cards, card)
	}
	return cards, nil
}

func cardFromElement(raw json.RawMessage) (listCard, bool, error) {
	var el struct {
		Name string `json:"name"`
		URL  string `json:"url"`
		Item *struct {
			Name  string `json:"name"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"item"`
	}
	if err := json.Unmarshal(raw, &el); err != nil {
		return listCard{}, false, fmt.Errorf("ejobs: list item: %w", err)
	}
	title := strings.TrimSpace(el.Name)
	link := strings.TrimSpace(el.URL)
	if el.Item != nil {
		if title == "" {
			title = strings.TrimSpace(el.Item.Name)
		}
		if title == "" {
			title = strings.TrimSpace(el.Item.Title)
		}
		if link == "" {
			link = strings.TrimSpace(el.Item.URL)
		}
	}
	listing, ok := canonicalListing(link)
	if !ok {
		return listCard{}, false, nil
	}
	return listCard{Title: title, URL: listing}, true, nil
}

func canonicalListing(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "www.ejobs.ro"
	}
	host := strings.ToLower(u.Host)
	if host != "www.ejobs.ro" && host != "ejobs.ro" {
		return "", false
	}
	if !strings.Contains(u.Path, "/user/locuri-de-munca/") {
		return "", false
	}
	u.Scheme = "https"
	u.Host = "www.ejobs.ro"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return "", false
	}
	return listing, true
}

func parseDetailHTML(body []byte, listing, listTitle string) (schema.Job, error) {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "ejobs")
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(jp.Title)
	if title == "" {
		title = strings.TrimSpace(listTitle)
	}
	if title == "" {
		return schema.Job{}, fmt.Errorf("ejobs: detail %s: empty title", listing)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("ejobs: detail %s: %w", listing, err)
	}
	city, country := placeFromJobLocation(jp.JobLocation)
	var countries []string
	if country != "" {
		countries = []string{country}
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(jp.Company),
		URL:         listing,
		Description: utils.StripHTMLToPlainText(jp.Description),
		PostedAt:    parsePosted(jp.DatePosted),
		SalaryRaw:   labeledValue(doc, "Salariu"),
		Location: schema.Location{
			Type:      workplace(labeledValue(doc, "Mod de lucru")),
			Countries: countries,
			Raw:       city,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func placeFromJobLocation(raw json.RawMessage) (string, string) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", ""
	}
	if raw[0] == '[' {
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return "", ""
		}
		for _, part := range parts {
			city, country := placeFromJobLocation(part)
			if city != "" || country != "" {
				return city, country
			}
		}
		return "", ""
	}
	var place struct {
		Address struct {
			AddressLocality string          `json:"addressLocality"`
			AddressCountry  json.RawMessage `json:"addressCountry"`
		} `json:"address"`
	}
	if err := json.Unmarshal(raw, &place); err != nil {
		return "", ""
	}
	return strings.TrimSpace(place.Address.AddressLocality), countryCode(place.Address.AddressCountry)
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

func labeledValue(doc *goquery.Document, label string) string {
	if doc == nil {
		return ""
	}
	var found string
	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		if found != "" || s.Children().Length() > 0 {
			return
		}
		own := strings.Trim(strings.TrimSpace(s.Text()), ":")
		if !strings.EqualFold(own, label) {
			return
		}
		if next := utils.NormalizePlainText(s.Next().Text()); next != "" {
			found = next
			return
		}
		parent := utils.NormalizePlainText(s.Parent().Text())
		rest := strings.Trim(strings.TrimSpace(strings.TrimPrefix(parent, utils.NormalizePlainText(own))), ":")
		rest = utils.NormalizePlainText(rest)
		if rest != "" && !strings.Contains(strings.ToLower(rest), "mod de lucru") && !strings.Contains(strings.ToLower(rest), "salariu") {
			found = rest
			return
		}
		if next := utils.NormalizePlainText(s.Parent().Next().Text()); next != "" {
			found = next
		}
	})
	return found
}

func workplace(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "remote":
		return schema.LocationRemote
	case "hibrid":
		return schema.LocationHybrid
	default:
		return schema.LocationOffice
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

func cloudflareBlocked(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "cf-error-details") || strings.Contains(s, "Sorry, you have been blocked") || strings.Contains(s, "Just a moment")
}

func jsonTypeIs(raw json.RawMessage, want string) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.EqualFold(strings.TrimSpace(text), want)
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return false
	}
	for _, v := range arr {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
	}
	return false
}
