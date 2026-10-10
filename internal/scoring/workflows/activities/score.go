// Package scoring_activities hosts Temporal activities for a profile run.
package scoring_activities

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	scoringimpl "github.com/andrewmysliuk/jobhound_core/internal/scoring/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

const (
	// LoadProfileActivityName re-reads the profile YAML for child expansion.
	LoadProfileActivityName = "LoadProfile"
	// ScoreProfileActivityName evaluates jobs in the score window and upserts matches.
	ScoreProfileActivityName = "ScoreProfile"
	// MarkRunFinishedActivityName writes the terminal status and counters.
	MarkRunFinishedActivityName = "MarkRunFinished"
	// SetRunCountersActivityName updates the running row without closing it.
	SetRunCountersActivityName = "SetRunCounters"
)

// Activities scores one profile run. It re-reads the YAML; the workflow payload does not carry phrases.
type Activities struct {
	Profiles profiles.Store
	Runs     *storage.Repository
	Jobs     jobs.JobRepository
	Log      zerolog.Logger
}

// LoadProfile reads the current profile file for this id.
func (a *Activities) LoadProfile(ctx context.Context, profileID string) (profileschema.Profile, error) {
	if a == nil || a.Profiles == nil {
		return profileschema.Profile{}, fmt.Errorf("score activity: profiles store is required")
	}
	profile, err := a.Profiles.Get(ctx, profileID)
	if err != nil {
		return profileschema.Profile{}, err
	}
	return profile, nil
}

// ScoreProfile loads jobs first seen inside the score window and upserts every result, including rejects.
func (a *Activities) ScoreProfile(ctx context.Context, in schema.ProfileRunInput) (schema.ScoreProfileResult, error) {
	if a == nil || a.Profiles == nil || a.Runs == nil || a.Jobs == nil {
		return schema.ScoreProfileResult{}, fmt.Errorf("score activity: profiles, runs, and jobs are required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, ScoreProfileActivityName))
	n, err := scoringimpl.New(a.Profiles, a.Runs, a.Jobs, nil).Score(ctx, in.ProfileID, in.RunID)
	if err != nil {
		return schema.ScoreProfileResult{}, err
	}
	log.Debug().Int("jobs_scored", n).Str("profile_id", in.ProfileID).Msg("score done")
	return schema.ScoreProfileResult{JobsScored: n}, nil
}

// SetRunCounters updates the open run so a poll can show scored jobs and skipped sources.
func (a *Activities) SetRunCounters(ctx context.Context, in schema.RunCountersInput) error {
	if a == nil || a.Runs == nil {
		return fmt.Errorf("score activity: runs repository is required")
	}
	return a.Runs.SetRunCounters(ctx, in.RunID, in.JobsScored, in.SourcesSkipped)
}

// MarkRunFinished writes SUCCEEDED or FAILED and the counters from this run.
func (a *Activities) MarkRunFinished(ctx context.Context, in schema.MarkRunFinishedInput) error {
	if a == nil || a.Runs == nil {
		return fmt.Errorf("score activity: runs repository is required")
	}
	return a.Runs.FinishRun(ctx, in.RunID, in.Status, in.JobsScored, in.SourcesSkipped, time.Now().UTC())
}
