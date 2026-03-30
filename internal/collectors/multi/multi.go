// Package multi combines several pipeline.Collector implementations into one Fetch
// while isolating per-source failures (contracts/collector.md).
package multi

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

// All runs each collector in order, merges successful job lists, and does not abort
// the batch when one source returns an error (unless every source fails).
type All struct {
	Collectors []pipeline.Collector
	// OnSourceError is called for each failed Fetch after others have still run. Optional.
	OnSourceError func(sourceName string, err error)
}

// Name implements pipeline.Collector.
func (*All) Name() string { return "mvp_sources" }

// Fetch implements pipeline.Collector.
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
			} else {
				log.Printf("collector: %v", wrapped)
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
