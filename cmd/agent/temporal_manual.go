package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	manual_workflows "github.com/andrewmysliuk/jobhound_core/internal/manual/workflows"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// temporalManualOpts holds flag values for the optional Temporal client path (009 remains primary trigger).
type temporalManualOpts struct {
	slotID          string
	runKind         string
	workflowID      string
	sourceIDs       string
	profile         string
	pipelineRunID   int64
	explicitRefresh bool
}

func runTemporalManualSlotRun(ctx context.Context, o temporalManualOpts) error {
	slotID, err := uuid.Parse(strings.TrimSpace(o.slotID))
	if err != nil {
		return fmt.Errorf("manual-slot-id: %w", err)
	}
	kind := manualschema.RunKind(strings.TrimSpace(o.runKind))
	if kind == "" {
		return fmt.Errorf("manual-run-kind is required")
	}

	in := manualschema.ManualSlotRunWorkflowInput{
		SlotID:          slotID,
		Kind:            kind,
		SourceIDs:       splitCommaNonEmpty(o.sourceIDs),
		Profile:         strings.TrimSpace(o.profile),
		ExplicitRefresh: o.explicitRefresh,
	}
	if o.pipelineRunID > 0 {
		id := o.pipelineRunID
		in.PipelineRunID = &id
	}
	if err := in.Validate(); err != nil {
		return err
	}

	cfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		return err
	}
	c, err := client.Dial(client.Options{
		HostPort:  cfg.Address,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return fmt.Errorf("temporal dial: %w", err)
	}
	defer c.Close()

	wfID := strings.TrimSpace(o.workflowID)
	if wfID == "" {
		wfID = fmt.Sprintf("agent-manual-slot-%d", time.Now().UnixNano())
	}

	agg, err := manual_workflows.StartManualSlotRunWorkflow(ctx, c, cfg.TaskQueue, wfID, in)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(agg); err != nil {
		return err
	}
	return nil
}

func splitCommaNonEmpty(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
