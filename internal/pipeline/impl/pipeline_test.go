package impl_test

import (
	"context"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline/mock"
)

func TestPipeline_Run_nilDependency(t *testing.T) {
	p := &impl.Pipeline{
		Collector: mock.Collector{},
		Filter:    mock.Filter{},
		Scorer:    mock.Scorer{},
		Dedup:     mock.Dedup{},
		Notify:    nil,
	}
	if err := p.Run(context.Background()); err == nil {
		t.Fatal("expected error for nil Notify")
	}
}

func TestPipeline_Run_noop(t *testing.T) {
	p := &impl.Pipeline{
		Collector: mock.Collector{},
		Filter:    mock.Filter{},
		Scorer:    mock.Scorer{},
		Dedup:     mock.Dedup{},
		Notify:    mock.Notifier{},
	}
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}
