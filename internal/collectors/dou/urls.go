package dou

import (
	"net/url"
	"strings"
)

// FeedURL builds the DOU RSS list URL: ?search={query}&remote (bare remote flag).
// Empty/whitespace query returns "" — DOU is query-driven, not a catalog.
func FeedURL(base, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = DefaultFeedBase
	}
	base = strings.TrimRight(base, "?&")
	return base + "?search=" + url.QueryEscape(query) + "&remote"
}

func (d *Dou) feedURL(query string) string {
	base := DefaultFeedBase
	if d != nil && strings.TrimSpace(d.FeedBase) != "" {
		base = d.FeedBase
	}
	return FeedURL(base, query)
}
