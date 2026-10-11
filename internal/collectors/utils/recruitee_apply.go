package utils

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func isRecruiteeHost(host string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(host)), "recruitee.com")
}

// RecruiteeJobPageMissing GETs a normalized Recruitee job URL and reports the public "job not found" page.
func RecruiteeJobPageMissing(ctx context.Context, client *http.Client, rawApplyURL string) (bool, error) {
	norm, err := NormalizeApplyURL(rawApplyURL)
	if err != nil {
		return false, err
	}
	if norm == "" {
		return false, nil
	}
	u, err := url.Parse(norm)
	if err != nil {
		return false, err
	}
	if !isRecruiteeHost(u.Host) {
		return false, nil
	}
	if client == nil {
		client = NewHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, norm, http.NoBody)
	if err != nil {
		return false, err
	}
	SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return true, nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	if err != nil {
		return false, err
	}
	return recruiteeNotFoundBody(string(b)), nil
}

func recruiteeNotFoundBody(html string) bool {
	lower := strings.ToLower(html)
	return strings.Contains(lower, "find this job") &&
		(strings.Contains(lower, "doesn't exist") || strings.Contains(lower, "does not exist") || strings.Contains(lower, "was removed"))
}
