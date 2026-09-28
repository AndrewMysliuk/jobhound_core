package schema

import pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"

// Stage2RunRequest is POST …/stages/2/run body.
type Stage2RunRequest struct {
	Rules []pipelineschema.Stage2Rule `json:"rules"`
}

// Stage3RunRequest is POST …/stages/3/run body.
type Stage3RunRequest struct {
	MaxJobs int `json:"max_jobs"`
}

// StageRunAcceptedResponse is 202 body after starting a stage 2 or 3 run.
type StageRunAcceptedResponse struct {
	SlotID string `json:"slot_id"`
	Stage  int    `json:"stage"`
}
