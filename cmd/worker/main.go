package main

import (
	"log"
	"os"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	"github.com/andrewmysliuk/jobhound_core/internal/llm/anthropic"
	llmmock "github.com/andrewmysliuk/jobhound_core/internal/llm/mock"
	pipeline_workflows "github.com/andrewmysliuk/jobhound_core/internal/pipeline/workflows"
	reference_workflows "github.com/andrewmysliuk/jobhound_core/internal/reference/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	cfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		log.Printf("temporal worker: %v", err)
		os.Exit(1)
	}

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Address,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		log.Printf("temporal worker: dial: %v", err)
		os.Exit(1)
	}
	defer c.Close()

	w := worker.New(c, cfg.TaskQueue, worker.Options{})

	appCfg := config.Load()
	var scorer llm.Scorer
	if strings.TrimSpace(appCfg.AnthropicAPIKey) != "" {
		scorer = anthropic.NewScorer(appCfg.AnthropicAPIKey, appCfg.AnthropicModel)
	} else {
		scorer = llmmock.Scorer{}
	}
	pipeline_workflows.RegisterActivities(w, pipeline_workflows.ActivitiesDeps{Scorer: scorer})

	reference_workflows.Register(w)

	log.Printf("temporal worker: polling queue %q namespace %q address %s", cfg.TaskQueue, cfg.Namespace, cfg.Address)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Printf("temporal worker: %v", err)
		os.Exit(1)
	}
}
