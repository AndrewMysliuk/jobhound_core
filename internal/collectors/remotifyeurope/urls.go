package remotifyeurope

import (
	"fmt"
	"net/url"
	"strings"
)

const siteOrigin = "https://remotifyeurope.com"

// DefaultListingURL is the catalog listing page (POST target for server actions).
const DefaultListingURL = siteOrigin + "/remote-jobs"

// ListingJobURL returns the board card URL for a job id.
func ListingJobURL(id string) string {
	id = strings.TrimSpace(id)
	return siteOrigin + "/listing/" + id
}

// encodeChunkPath escapes parentheses in Next.js chunk paths for HTTP GET.
func encodeChunkPath(path string) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.ReplaceAll(strings.ReplaceAll(path, "(", "%28"), ")", "%29")
}

func absoluteChunkURL(base *url.URL, chunkPath string) string {
	if base == nil {
		u, _ := url.Parse(siteOrigin)
		base = u
	}
	ref, err := url.Parse(encodeChunkPath(chunkPath))
	if err != nil {
		return siteOrigin + encodeChunkPath(chunkPath)
	}
	return base.ResolveReference(ref).String()
}

func listingPOSTURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = DefaultListingURL
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("remotify europe: listing url: %w", err)
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Host == "" {
		u.Host = "remotifyeurope.com"
	}
	return u.String(), nil
}
