// Package golangcafe implements the golang.cafe board collector (JSON API via browserfetch).
package golangcafe

import (
	"context"
	"fmt"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "golang_cafe"

// DefaultMaxSearchRounds caps API pagination when MaxSearchRounds is 0.
const DefaultMaxSearchRounds = 5

// GolangCafe fetches the Europe/remote catalog via rod-backed JSON API calls.
type GolangCafe struct {
	HTMLDocumentFetcher browserfetch.HTMLDocumentFetcher
	// MaxSearchRounds: 0 → DefaultMaxSearchRounds; -1 → no cap; >0 → explicit cap.
	MaxSearchRounds int
	// MaxJobs stops after this many jobs (0 = unlimited).
	MaxJobs int
}

// Name implements collectors.Collector.
func (*GolangCafe) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *GolangCafe) Fetch(ctx context.Context) ([]schema.Job, error) {
	if c == nil || c.HTMLDocumentFetcher == nil {
		return nil, fmt.Errorf("golang cafe: HTMLDocumentFetcher is nil")
	}
	posts, err := c.fetchAllSearchPosts(ctx, "")
	if err != nil {
		return nil, err
	}
	return jobsFromPosts(posts, c.maxJobsEffective())
}

func (c *GolangCafe) maxSearchRoundsEffective() int {
	if c == nil {
		return DefaultMaxSearchRounds
	}
	switch {
	case c.MaxSearchRounds == 0:
		return DefaultMaxSearchRounds
	case c.MaxSearchRounds < 0:
		return 0
	default:
		return c.MaxSearchRounds
	}
}

func (c *GolangCafe) maxJobsEffective() int {
	if c == nil {
		return 0
	}
	return c.MaxJobs
}
