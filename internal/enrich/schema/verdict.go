package schema

import "time"

// Verdict is one stored facts reply for a job at a prompt version, model, and input hash.
type Verdict struct {
	JobID         string
	PromptVersion int
	Model         string
	InputHash     string
	Facts         ModelReply
	CreatedAt     time.Time
}
