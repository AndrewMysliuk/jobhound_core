package main

import (
	"log"
	"os"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
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
	reference_workflows.Register(w)

	log.Printf("temporal worker: polling queue %q namespace %q address %s", cfg.TaskQueue, cfg.Namespace, cfg.Address)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Printf("temporal worker: %v", err)
		os.Exit(1)
	}
}
