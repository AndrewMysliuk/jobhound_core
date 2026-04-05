// Package multi combines several collectors.Collector implementations into one Fetch
// while isolating per-source failures (contracts/collector.md).
package multi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

// All runs each collector in order, merges successful job lists, and does not abort
// the batch when one source returns an error (unless every source fails).
type All struct {
	Collectors []collectors.Collector
	// OnSourceError is called for each failed Fetch after others have still run. Optional.
	OnSourceError func(sourceName string, err error)
	// Log is used when OnSourceError is nil: one Warn per failed source (FieldSourceID). Optional.
	Log *zerolog.Logger
}

// Name implements collectors.Collector.
func (*All) Name() string { return "mvp_sources" }

// Fetch implements collectors.Collector.
func (a *All) Fetch(ctx context.Context) ([]domain.Job, error) {
	if a == nil {
		return nil, fmt.Errorf("multi.All: nil receiver")
	}
	var jobs []domain.Job
	var errs []error
	for _, c := range a.Collectors {
		if c == nil {
			continue
		}
		got, err := c.Fetch(ctx)
		if err != nil {
			wrapped := fmt.Errorf("%s: %w", c.Name(), err)
			errs = append(errs, wrapped)
			if a.OnSourceError != nil {
				a.OnSourceError(c.Name(), err)
			} else if a.Log != nil {
				src := strings.ToLower(strings.TrimSpace(c.Name()))
				a.Log.Warn().
					Str(logging.FieldSourceID, src).
					Err(err).
					Msg("collector fetch failed")
			}
			continue
		}
		jobs = append(jobs, got...)
	}
	if len(jobs) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return jobs, nil
}
