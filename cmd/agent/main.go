package main

import (
	"context"
	"fmt"
	"os"

	llmmock "github.com/andrewmysliuk/jobhound_core/internal/llm/mock"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline/mock"
)

func main() {
	p := &impl.Pipeline{
		Collector: mock.Collector{},
		Scorer: llmmock.Scorer{},
		Dedup:     mock.Dedup{},
		Notify:    mock.Notifier{},
	}
	if err := p.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "jobhound_core: noop pipeline run ok")
}
