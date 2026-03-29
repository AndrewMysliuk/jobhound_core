package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// idSep is the unit separator (U+001E); unlikely in source keys or URLs.
const idSep = "\x1e"

// NormalizeListingURL returns a canonical form of an absolute http(s) job listing URL for identity.
// Rules match specs/001-agent-skeleton-and-domain/spec.md (URL normalization v1).
func NormalizeListingURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("empty listing URL")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("URL host is required")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String(), nil
}

// StableJobID returns the dedup/history key for one vacancy as seen on one source
// (source + normalized listing URL). Callers pass ApplyURL as the second argument only when
// the listing URL is missing (see spec fallback).
func StableJobID(source, listingURL string) (string, error) {
	src := strings.TrimSpace(source)
	if src == "" {
		return "", errors.New("empty source")
	}
	src = strings.ToLower(src)

	norm, err := NormalizeListingURL(listingURL)
	if err != nil {
		return "", err
	}
	return src + idSep + norm, nil
}

// AssignStableID sets j.ID from Source and listing URL, or ApplyURL if listing URL is empty (spec fallback).
func AssignStableID(j *Job) error {
	if j == nil {
		return errors.New("nil job")
	}
	listing := j.URL
	if strings.TrimSpace(listing) == "" {
		listing = j.ApplyURL
	}
	id, err := StableJobID(j.Source, listing)
	if err != nil {
		return err
	}
	j.ID = id
	return nil
}
