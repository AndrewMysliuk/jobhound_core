// Package archinect implements the archinect.com remote-region jobs collector.
package archinect

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "archinect"

const (
	siteOrigin   = "https://archinect.com"
	remotePath   = "/jobs/region/Remote/all/remote"
	pageSize     = 30
	maxListPages = 40
	requestGap   = 5 * time.Second
)

// Archinect fetches the remote region with net/http.
type Archinect struct {
	HTTPClient *http.Client
	// Gap is the wait between requests. Zero uses 5s. Negative skips the wait.
	Gap time.Duration
}

// One request to the site at a time, then at least Gap before the next.
var (
	siteMu  sync.Mutex
	lastHit time.Time
)

// Name implements collectors.Collector.
func (*Archinect) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Archinect) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// Only the remote child is fetched. Any other query, including another region, fetches nothing.
func (c *Archinect) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	if !strings.EqualFold(strings.TrimSpace(query), "remote") {
		return nil, nil
	}
	client := c.httpClient()
	seenURL := make(map[string]struct{})
	seenID := make(map[string]struct{})
	var out []schema.Job
	total := -1
	for page := 0; page < maxListPages; page++ {
		offset := page * pageSize
		if total >= 0 && offset >= total {
			break
		}
		body, err := c.get(ctx, client, listURL(offset))
		if err != nil {
			return nil, err
		}
		rows, pageTotal, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		if pageTotal >= 0 {
			total = pageTotal
		}
		if len(rows) == 0 {
			break
		}
		jobs, err := c.jobsFromRows(ctx, client, rows, seenURL, seenID)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func listURL(offset int) string {
	if offset <= 0 {
		return siteOrigin + remotePath
	}
	return siteOrigin + remotePath + "/" + strconv.Itoa(offset)
}

func (c *Archinect) jobsFromRows(ctx context.Context, client *http.Client, rows []listRow, seenURL, seenID map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, row := range rows {
		if _, dup := seenURL[row.URL]; dup {
			continue
		}
		seenURL[row.URL] = struct{}{}
		body, err := c.get(ctx, client, row.URL)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, row)
		if err != nil {
			return nil, err
		}
		if _, dup := seenID[job.ID]; dup {
			continue
		}
		seenID[job.ID] = struct{}{}
		out = append(out, job)
	}
	return out, nil
}

func (c *Archinect) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func (c *Archinect) get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if !allowedURL(rawURL) {
		return nil, fmt.Errorf("archinect: refused %s", rawURL)
	}
	siteMu.Lock()
	defer siteMu.Unlock()
	if err := c.pause(ctx); err != nil {
		return nil, err
	}
	body, err := fetch(ctx, client, rawURL)
	lastHit = time.Now()
	return body, err
}

func fetch(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("archinect: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("archinect: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("archinect: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}

func (c *Archinect) pause(ctx context.Context) error {
	d := c.gap()
	if d <= 0 || lastHit.IsZero() {
		return nil
	}
	wait := d - time.Since(lastHit)
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Archinect) gap() time.Duration {
	if c != nil && c.Gap < 0 {
		return 0
	}
	if c != nil && c.Gap > requestGap {
		return c.Gap
	}
	return requestGap
}

func allowedURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.RawQuery != "" || u.Host == "" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "archinect.com" {
		return false
	}
	path := strings.TrimRight(u.Path, "/")
	if path == remotePath {
		return true
	}
	if rest, ok := strings.CutPrefix(path, remotePath+"/"); ok {
		n, err := strconv.Atoi(rest)
		return err == nil && n > 0 && n%pageSize == 0
	}
	return entryPathRE.MatchString(u.Path)
}
