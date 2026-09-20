package remotifyeurope

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

var (
	chunkPathRE = regexp.MustCompile(`/_next/static/chunks/[^"'\s<>]+\.js`)
	jobsListCallRE = regexp.MustCompile(`\(\d+,\w+\.(\w+)\)\(24,new Date\(\)\.toISOString\(\)`)
)

// DiscoverNextAction loads the listing page, fetches referenced JS chunks, and resolves the Next-Action hash for job listing POSTs.
func DiscoverNextAction(ctx context.Context, client *http.Client, listingURL string) (string, error) {
	if client == nil {
		client = utils.NewHTTPClient()
	}
	u, err := url.Parse(strings.TrimSpace(listingURL))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		parsed, err := url.Parse(DefaultListingURL)
		if err != nil {
			return "", err
		}
		u = parsed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return "", err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("remotify europe: listing page: %w", err)
	}
	defer resp.Body.Close()
	html, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("remotify europe: listing page: HTTP %s", resp.Status)
	}
	paths := chunkPathRE.FindAllString(string(html), -1)
	seen := make(map[string]struct{})
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		chunkURL := absoluteChunkURL(u, p)
		hash, ok, err := fetchJobsActionFromChunkURL(ctx, client, chunkURL)
		if err != nil {
			continue
		}
		if ok {
			return hash, nil
		}
	}
	return "", fmt.Errorf("remotify europe: Next-Action hash not found in JS bundles")
}

func fetchJobsActionFromChunkURL(ctx context.Context, client *http.Client, chunkURL string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, chunkURL, http.NoBody)
	if err != nil {
		return "", false, err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("chunk HTTP %s", resp.Status)
	}
	hash, ok := jobsActionHashFromJS(string(b))
	return hash, ok, nil
}

func jobsActionHashFromJS(js string) (string, bool) {
	if !strings.Contains(js, "(24,new Date().toISOString()") {
		return "", false
	}
	cm := jobsListCallRE.FindStringSubmatch(js)
	if len(cm) < 2 {
		return "", false
	}
	fn := cm[1]
	expRE := regexp.MustCompile(regexp.QuoteMeta(fn) + `:function\(\)\{return (\w+)\}`)
	em := expRE.FindStringSubmatch(js)
	if len(em) < 2 {
		return "", false
	}
	retVar := em[1]
	hashRE := regexp.MustCompile(regexp.QuoteMeta(retVar) + `=\(\d+,\w+\.\$\)\("([a-f0-9]{40})"\)`)
	hm := hashRE.FindStringSubmatch(js)
	if len(hm) < 2 {
		return "", false
	}
	return hm[1], true
}
