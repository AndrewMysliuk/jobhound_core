package schema

import "encoding/json"

// CollectorsPOSTBody is the optional JSON body for POST /debug/collectors/* (dev-only debughttp).
// Omitted keys use defaults; per-route handlers ignore keys that do not apply to that source.
type CollectorsPOSTBody struct {
	Limit *int `json:"limit"`
	// Working Nomads → jobsapi/_search (ignored on europe_remotely).
	Query            *json.RawMessage `json:"query"`
	Sort             *json.RawMessage `json:"sort"`
	PageSize         *int             `json:"page_size"`
	SourceFieldNames []string         `json:"_source"`
	// Europe Remotely → merged into POST admin-ajax form after bootstrap fields (ignored on working_nomads).
	FeedForm map[string]string `json:"feed_form,omitempty"`
	// Europe Remotely: sets form field search_keywords (applied after feed_form).
	SearchKeywords *string `json:"search_keywords,omitempty"`
	// DOU.ua: listing query param search (resources/dou.md); ignored on other routes.
	Search *string `json:"search,omitempty"`
	// DOU.ua: inter-request delay override in milliseconds (ignored on other routes).
	DouInterRequestDelayMs *int `json:"dou_inter_request_delay_ms,omitempty"`
	// Himalayas: search free-text q (ignored on other routes).
	Q *string `json:"q,omitempty"`
	// Himalayas: 1-based search page (ignored on other routes).
	Page *int `json:"page,omitempty"`
	// Himalayas: use /jobs/api/search instead of browse (ignored on other routes).
	UseSearch *bool `json:"use_search,omitempty"`
	// Himalayas: cap HTTP pages per run (ignored on other routes).
	MaxPages *int `json:"max_pages,omitempty"`
	// Djinni: listing all_keywords (ignored on other routes).
	AllKeywords *string `json:"all_keywords,omitempty"`
	// Djinni: first listing page (1-based; ignored on other routes).
	DjinniPage *int `json:"djinni_page,omitempty"`
	// Djinni: inter-request delay override in milliseconds (ignored on other routes).
	DjinniInterRequestDelayMs *int `json:"djinni_inter_request_delay_ms,omitempty"`
}
