package schema

import "encoding/json"

// CollectorsPOSTBody is the optional JSON body for POST /debug/collectors/* (dev-only debughttp).
// Omitted keys use defaults; Working Nomads–specific fields are ignored on the Europe Remotely route;
// Europe-specific feed fields are ignored on working_nomads.
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
}
