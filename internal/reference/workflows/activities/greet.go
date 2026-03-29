package reference_activities

import "context"

// ReferenceGreetActivity returns a deterministic greeting for the reference workflow (v0).
// No I/O, Postgres, or secrets — see specs/003-temporal-orchestration/contracts/reference-workflow.md.
func ReferenceGreetActivity(ctx context.Context, name string) (string, error) {
	_ = ctx
	if name == "" {
		name = "world"
	}
	return "Hello, " + name + "!", nil
}
