package main

import (
	"context"
	"fmt"
	"os"

	"github.com/andrewmysliuk/jobhound_core/internal/adapters/noop"
	"github.com/andrewmysliuk/jobhound_core/internal/app"
)

func main() {
	p := &app.Pipeline{
		Collector: noop.Collector{},
		Filter:    noop.Filter{},
		Scorer:    noop.Scorer{},
		Dedup:     noop.Dedup{},
		Notify:    noop.Notifier{},
	}
	if err := p.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "jobhound_core: noop pipeline run ok")
}
