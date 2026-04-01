package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
)

// Pipeline wires pipeline contracts and runs one collect → stage 1 → stage 2 → stage 3 → dedup → notify pass.
// It is orchestration only: no persistence or Telegram inside stage functions.
type Pipeline struct {
	Collector collectors.Collector
	// Clock is used for stage 1 default date window; nil means time.Now.
	Clock func() time.Time
	// BroadRules and KeywordRules are per-run (event) parameters.
	BroadRules   pipeline.BroadFilterRules
	KeywordRules pipeline.KeywordRules
	// Profile is user CV / preferences text for stage 3.
	Profile string

	Scorer llm.Scorer
	Dedup  pipeline.Dedup
	Notify pipeline.Notifier
}

// Run executes a single pipeline pass.
func (p *Pipeline) Run(ctx context.Context) error {
	if p.Collector == nil || p.Scorer == nil || p.Dedup == nil || p.Notify == nil {
		return fmt.Errorf("pipeline.impl.Pipeline: nil dependency")
	}

	raw, err := p.Collector.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("collect %q: %w", p.Collector.Name(), err)
	}

	clock := p.Clock
	if clock == nil {
		clock = time.Now
	}
	stage1, err := pipeutils.ApplyBroadFilter(clock, p.BroadRules, raw)
	if err != nil {
		return fmt.Errorf("broad filter: %w", err)
	}
	stage2 := pipeutils.ApplyKeywordFilter(stage1, p.KeywordRules)

	scored, err := pipeutils.ScoreJobs(ctx, p.Profile, stage2, p.Scorer)
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
