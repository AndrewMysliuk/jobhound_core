package utils

import (
	"net/url"
	"strings"

	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// NormalizeApplyURL applies host-specific cleanup to external apply links before persistence.
// Recruitee apply buttons often use /o/{slug}/c/new with tracking query params; job pages use /o/{slug}.
func NormalizeApplyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	norm, err := domainutils.NormalizeListingURL(raw)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(norm)
	if err != nil {
		return "", err
	}
	if !isRecruiteeHost(u.Host) {
		return norm, nil
	}
	path := u.Path
	if i := strings.Index(path, "/c/"); i >= 0 && strings.HasPrefix(path, "/o/") {
		path = path[:i]
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	u.Path = path
	return u.String(), nil
}
