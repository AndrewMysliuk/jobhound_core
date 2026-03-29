package noop

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// Collector returns no jobs.
type Collector struct{}

func (Collector) Name() string { return "noop" }

func (Collector) Fetch(context.Context) ([]domain.Job, error) { return nil, nil }

// Filter passes jobs through unchanged.
type Filter struct{}

func (Filter) Apply(jobs []domain.Job) []domain.Job { return jobs }

// Scorer returns an empty slice.
type Scorer struct{}

func (Scorer) Score(context.Context, []domain.Job) ([]domain.ScoredJob, error) { return nil, nil }

// Dedup always reports not sent; MarkSent is a no-op.
type Dedup struct{}

func (Dedup) WasSent(context.Context, string) (bool, error) { return false, nil }

func (Dedup) MarkSent(context.Context, string) error { return nil }

// Notifier succeeds without doing I/O.
type Notifier struct{}

func (Notifier) Send(context.Context, []domain.ScoredJob) error { return nil }

// SessionProvider returns an empty path.
type SessionProvider struct{}

func (SessionProvider) CookieFilePath() string { return "" }
