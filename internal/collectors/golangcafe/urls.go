package golangcafe

import (
	"fmt"
	"net/url"
	"strings"
)

const siteOrigin = "https://golang.cafe"

// SearchAPIURL builds GET /api/jobPosts/search with optional query and before (epoch ms).
func SearchAPIURL(query string, beforeMs int64) string {
	v := url.Values{}
	v.Set("query", query)
	if beforeMs > 0 {
		v.Set("before", fmt.Sprintf("%d", beforeMs))
	}
	return siteOrigin + "/api/jobPosts/search?" + v.Encode()
}

// JobCardURL returns the public job page on golang.cafe (identity URL).
func JobCardURL(company, title, id string) string {
	seg := slugSegment(company) + "-" + slugSegment(title) + "-" + strings.TrimSpace(id)
	return siteOrigin + "/jobs/" + seg
}

func slugSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), "-")
}
