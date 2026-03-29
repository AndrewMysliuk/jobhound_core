package impl

import (
	"context"
	"fmt"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

// Pipeline wires pipeline contracts and runs one collect → filter → score → dedup → notify pass.
// It is orchestration only: no domain “service calling service”; persistence goes through Dedup.
type Pipeline struct {
	Collector pipeline.Collector
	Filter    pipeline.Filter
	Scorer    pipeline.Scorer
	Dedup     pipeline.Dedup
	Notify    pipeline.Notifier
}

// Run executes a single pipeline pass.
func (p *Pipeline) Run(ctx context.Context) error {
	if p.Collector == nil || p.Filter == nil || p.Scorer == nil || p.Dedup == nil || p.Notify == nil {
		return fmt.Errorf("pipeline.impl.Pipeline: nil dependency")
	}

	raw, err := p.Collector.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("collect %q: %w", p.Collector.Name(), err)
	}

	filtered := p.Filter.Apply(raw)
	scored, err := p.Scorer.Score(ctx, filtered)
	if err != nil {
		return fmt.Errorf("score: %w", err)
	}

	var toSend []domain.ScoredJob
	for _, sj := range scored {
		sent, err := p.Dedup.WasSent(ctx, sj.Job.ID)
		if err != nil {
			return fmt.Errorf("dedup WasSent %q: %w", sj.Job.ID, err)
		}
		if !sent {
			toSend = append(toSend, sj)
		}
	}

	if len(toSend) == 0 {
		return nil
	}

	if err := p.Notify.Send(ctx, toSend); err != nil {
		return fmt.Errorf("notify: %w", err)
	}

	for _, sj := range toSend {
		if err := p.Dedup.MarkSent(ctx, sj.Job.ID); err != nil {
			return fmt.Errorf("dedup MarkSent %q: %w", sj.Job.ID, err)
		}
	}

	return nil
}
