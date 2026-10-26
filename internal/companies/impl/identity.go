package impl

import (
	"net/url"
	"strings"

	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// NormalizeName lowercases the display name and strips Inc/GmbH/Ltd/LLC/SRL.
// Same rule as jobs.company_key (domain/utils.CompanyKey).
func NormalizeName(name string) string {
	return domainutils.CompanyKey(name)
}

// NormalizeDomain returns a bare host for use as a company key: lowercase,
// without a leading www. or careers. prefix. Empty input yields "".
func NormalizeDomain(website string) string {
	s := strings.TrimSpace(website)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		host := strings.ToLower(strings.TrimSpace(website))
		return stripDomainPrefixes(host)
	}
	host := strings.ToLower(u.Hostname())
	return stripDomainPrefixes(host)
}

func stripDomainPrefixes(host string) string {
	for {
		switch {
		case strings.HasPrefix(host, "www."):
			host = strings.TrimPrefix(host, "www.")
		case strings.HasPrefix(host, "careers."):
			host = strings.TrimPrefix(host, "careers.")
		default:
			return host
		}
	}
}

// ChooseKey picks the immutable company key at birth: domain if a website is
// known, otherwise the normalized name. The key is never promoted later.
func ChooseKey(name, website string) string {
	if d := NormalizeDomain(website); d != "" {
		return d
	}
	return NormalizeName(name)
}

// The ats_slug is never normalized. Lowercasing breaks Ashby case-significant
// boards (Coder, Deepnote, Mastra); stripping dots breaks primer.io and
// ecosia.org; name-derived guessing also fails for Form3's slug 541. Store and
// send the slug byte-for-byte as seeded or resolved.
