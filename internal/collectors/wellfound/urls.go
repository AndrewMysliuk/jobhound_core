package wellfound

import (
	"fmt"
	"strings"
)

const siteOrigin = "https://wellfound.com"

func roleListingURL(slug string, page int) string {
	slug = strings.Trim(strings.TrimSpace(slug), "/")
	u := fmt.Sprintf("%s/role/r/%s", siteOrigin, slug)
	if page > 1 {
		u = fmt.Sprintf("%s?page=%d", u, page)
	}
	return u
}

func jobListingURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return siteOrigin + path
}
