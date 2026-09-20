// Package weworkremotely implements the We Work Remotely board collector (RSS catalog only).
package weworkremotely

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "we_work_remotely"

// DefaultFeedURL is the global remote jobs RSS feed.
const DefaultFeedURL = "https://weworkremotely.com/remote-jobs.rss"

// WeWorkRemotely fetches jobs via GET RSS (no HTML).
type WeWorkRemotely struct {
	HTTPClient *http.Client
	// FeedURL defaults to DefaultFeedURL.
	FeedURL string
	Countries *utils.CountryResolver
}

// Name implements collectors.Collector.
func (*WeWorkRemotely) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *WeWorkRemotely) Fetch(ctx context.Context) ([]schema.Job, error) {
	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	feedURL := c.FeedURL
	if feedURL == "" {
		feedURL = DefaultFeedURL
	}
	body, err := fetchFeedBytes(ctx, client, feedURL)
	if err != nil {
		return nil, fmt.Errorf("we work remotely: %w", err)
	}
	jobs, err := jobsFromRSS(body, c.Countries)
	if err != nil {
		return nil, fmt.Errorf("we work remotely: %w", err)
	}
	return jobs, nil
}
