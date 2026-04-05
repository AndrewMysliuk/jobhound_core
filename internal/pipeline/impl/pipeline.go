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
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

// Pipeline wires pipeline contracts and runs one collect → stage 1 → stage 2 → stage 3 → dedup → notify pass.
// It is orchestration only: no persistence or side-channel notifications inside stage functions.
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
	// Log is optional; when set, Run emits structured errors at the service boundary (e.g. cmd/agent).
	Log zerolog.Logger
}

// Run executes a single pipeline pass.
func (p *Pipeline) Run(ctx context.Context) error {
	log := logging.EnrichWithContext(ctx, p.Log.With().Str(logging.FieldService, "pipeline").Str(logging.FieldMethod, "Run").Logger())
	if p.Collector == nil || p.Scorer == nil || p.Dedup == nil || p.Notify == nil {
		err := fmt.Errorf("pipeline.impl.Pipeline: nil dependency")
		log.Error().Err(err).Msg("run")
		return err
	}

	raw, err := p.Collector.Fetch(ctx)
	if err != nil {
		err = fmt.Errorf("collect %q: %w", p.Collector.Name(), err)
		log.Error().Err(err).Msg("collect")
		return err
	}

	clock := p.Clock
	if clock == nil {
		clock = time.Now
	}
	stage1, err := pipeutils.ApplyBroadFilter(clock, p.BroadRules, raw)
	if err != nil {
		err = fmt.Errorf("broad filter: %w", err)
		log.Error().Err(err).Msg("broad filter")
		return err
	}
	stage2 := pipeutils.ApplyKeywordFilter(stage1, p.KeywordRules)

	scored, err := pipeutils.ScoreJobs(ctx, p.Profile, stage2, p.Scorer)
	if err != nil {
		err = fmt.Errorf("score: %w", err)
		log.Error().Err(err).Msg("score")
		return err
	}

	var toSend []domain.ScoredJob
	for _, sj := range scored {
		sent, err := p.Dedup.WasSent(ctx, sj.Job.ID)
		if err != nil {
			err = fmt.Errorf("dedup WasSent %q: %w", sj.Job.ID, err)
			log.Error().Err(err).Msg("dedup WasSent")
			return err
		}
		if !sent {
			toSend = append(toSend, sj)
		}
	}

	if len(toSend) == 0 {
		return nil
	}

	if err := p.Notify.Send(ctx, toSend); err != nil {
		err = fmt.Errorf("notify: %w", err)
		log.Error().Err(err).Msg("notify")
		return err
	}

	for _, sj := range toSend {
		if err := p.Dedup.MarkSent(ctx, sj.Job.ID); err != nil {
			err = fmt.Errorf("dedup MarkSent %q: %w", sj.Job.ID, err)
			log.Error().Err(err).Msg("dedup MarkSent")
			return err
		}
	}

	return nil
}
