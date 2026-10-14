// Package utils holds stable vacancy identity helpers for the shared domain schema types.
package utils

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

const idSep = "\x1e"

var whitespaceRun = regexp.MustCompile(`\s+`)

// NormalizeListingURL returns a canonical form of an absolute http(s) job listing URL.
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
	u.RawQuery = ""
	if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String(), nil
}

// CompanyKey lowercases the display name and drops the legal suffixes
// Inc, GmbH, Ltd, LLC, and SRL.
func CompanyKey(name string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(name)))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if isLegalSuffix(strings.Trim(f, ".,")) {
			continue
		}
		out = append(out, strings.TrimRight(f, ",."))
	}
	return strings.Join(out, " ")
}

func isLegalSuffix(token string) bool {
	switch token {
	case "inc", "gmbh", "ltd", "llc", "srl":
		return true
	default:
		return false
	}
}

// StableJobID is company_key + separator + normalized title.
// Location and apply_url are not part of the key.
func StableJobID(companyKey, title string) string {
	return companyKey + idSep + normalizeIdentitySegment(title)
}

func normalizeIdentitySegment(s string) string {
	return strings.ToLower(whitespaceRun.ReplaceAllString(strings.TrimSpace(s), " "))
}

func isPlaceholderCompanyKey(key string) bool {
	switch key {
	case "", "confidential", "confidențial", "nedivulgat":
		return true
	default:
		return false
	}
}

// AssignStableID sets CompanyKey from Company and ID from that key and the title.
// A placeholder company key also appends the normalized listing URL.
func AssignStableID(j *schema.Job) error {
	if j == nil {
		return errors.New("nil job")
	}
	j.CompanyKey = CompanyKey(j.Company)
	j.ID = StableJobID(j.CompanyKey, j.Title)
	if !isPlaceholderCompanyKey(j.CompanyKey) {
		return nil
	}
	listing, err := NormalizeListingURL(j.URL)
	if err != nil {
		return err
	}
	j.ID += idSep + listing
	return nil
}
