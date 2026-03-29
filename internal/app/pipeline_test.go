package app_test

import (
	"context"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/adapters/noop"
	"github.com/andrewmysliuk/jobhound_core/internal/app"
)

func TestPipeline_Run_nilDependency(t *testing.T) {
	p := &app.Pipeline{
		Collector: noop.Collector{},
		Filter:    noop.Filter{},
		Scorer:    noop.Scorer{},
		Dedup:     noop.Dedup{},
		Notify:    nil,
	}
	if err := p.Run(context.Background()); err == nil {
		t.Fatal("expected error for nil Notify")
	}
}

func TestPipeline_Run_noop(t *testing.T) {
	p := &app.Pipeline{
		Collector: noop.Collector{},
		Filter:    noop.Filter{},
		Scorer:    noop.Scorer{},
		Dedup:     noop.Dedup{},
		Notify:    noop.Notifier{},
	}
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}
