package scoring_workflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	ingest_activities "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows/activities"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	scoringstorage "github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

func TestProfileRunWorkflow_scoresKeepsHiddenAndSkipsMissingSource(t *testing.T) {
	db := testRunDB(t)
	getter := pgsql.NewGetter(db)
	jobsRepo := jobsstorage.NewRepository(getter)
	runs := scoringstorage.NewRepository(getter)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	vueJob := jobdata.Job{
		ID:          "job-vue",
		Source:      "himalayas",
		Title:       "Vue Engineer",
		Company:     "Acme",
		URL:         "https://example.test/vue",
		Description: "frontend-engineer builds vue",
		Location:    jobdata.Location{Type: jobdata.LocationRemote, Raw: "Berlin"},
	}
	recruiterJob := jobdata.Job{
		ID:          "job-recruiter",
		Source:      "himalayas",
		Title:       "Recruiter",
		Company:     "Acme",
		URL:         "https://example.test/recruiter",
		Description: "vue and frontend-engineer",
		Location:    jobdata.Location{Type: jobdata.LocationRemote, Raw: "Berlin"},
	}
	profile := profileschema.Profile{
		ID:             "andrew",
		Name:           "Andrew",
		Domain:         profileschema.DomainSoftware,
		Sources:        []string{"himalayas", "wellfound", "builtin"},
		Queries:        []string{"vue"},
		WellfoundRoles: []string{"frontend-engineer"},
		ExcludeTitle:   []string{"recruiter"},
	}
	deps := Deps{
		Profiles: staticProfiles{profile: profile},
		Runs:     runs,
		Jobs:     jobsRepo,
		Log:      logging.Nop(),
	}
	ing := &ingest_activities.IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       jobsRepo,
		Watermarks: ingest.NewGormWatermarkStore(getter),
		Collectors: map[string]collectors.Collector{
			"himalayas": stubCollector{jobs: []jobdata.Job{vueJob, recruiterJob}},
			"wellfound": stubCollector{},
		},
		Log: logging.Nop(),
	}

	first := createRunning(t, runs, profile.ID)
	runWorkflow(t, deps, ing, first.ID)
	assertRun(t, db, first.ID, string(scoringschema.RunStatusSucceeded), 2, 1)
	assertMatch(t, db, vueJob.ID, string(scoringschema.BucketPassed), 1, string(scoringschema.UserStatusNew), scoringschema.SignalQuery, "vue")
	assertMatch(t, db, recruiterJob.ID, string(scoringschema.BucketRejected), 0, string(scoringschema.UserStatusNew), scoringschema.SignalCutExcludeTitle, "recruiter")

	if err := db.Exec(
		`UPDATE profile_matches SET user_status = ? WHERE profile_id = ? AND job_id = ?`,
		string(scoringschema.UserStatusHidden), profile.ID, vueJob.ID,
	).Error; err != nil {
		t.Fatal(err)
	}

	second := createRunning(t, runs, profile.ID)
	runWorkflow(t, deps, ing, second.ID)
	// himalayas and wellfound are cooling down; builtin is still missing.
	assertRun(t, db, second.ID, string(scoringschema.RunStatusSucceeded), 2, 3)
	assertMatch(t, db, vueJob.ID, string(scoringschema.BucketPassed), 1, string(scoringschema.UserStatusHidden), scoringschema.SignalQuery, "vue")
	assertMatch(t, db, recruiterJob.ID, string(scoringschema.BucketRejected), 0, string(scoringschema.UserStatusNew), scoringschema.SignalCutExcludeTitle, "recruiter")

	var n int64
	if err := db.Model(&scoringstorage.ProfileMatch{}).Where("profile_id = ?", profile.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("matches = %d, want 2", n)
	}
}

func runWorkflow(t *testing.T, deps Deps, ing *ingest_activities.IngestActivities, runID int64) {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetTestTimeout(time.Minute)
	New(env, deps)
	env.RegisterActivityWithOptions(ing.RunIngestSource, activity.RegisterOptions{
		Name: ingest_activities.RunIngestSourceActivityName,
	})
	env.RegisterWorkflowWithOptions(ingest_workflows.IngestSourceWorkflow, workflow.RegisterOptions{
		Name: ingest_workflows.IngestSourceWorkflowName,
	})
	env.ExecuteWorkflow(ProfileRunWorkflow, scoringschema.ProfileRunInput{
		ProfileID: "andrew",
		RunID:     runID,
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
}

func createRunning(t *testing.T, runs *scoringstorage.Repository, profileID string) scoringschema.Run {
	t.Helper()
	run, err := runs.CreateRun(context.Background(), scoringschema.Run{
		ProfileID:      profileID,
		Status:         scoringschema.RunStatusRunning,
		StartedAt:      time.Now().UTC(),
		IdempotencyKey: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func assertRun(t *testing.T, db *gorm.DB, id int64, status string, jobsScored, sourcesSkipped int) {
	t.Helper()
	var row scoringstorage.ProfileRun
	if err := db.First(&row, id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != status || row.JobsScored != jobsScored || row.SourcesSkipped != sourcesSkipped || row.FinishedAt == nil {
		t.Fatalf("run %d status=%s scored=%d skipped=%d finished=%v", id, row.Status, row.JobsScored, row.SourcesSkipped, row.FinishedAt)
	}
}

func assertMatch(t *testing.T, db *gorm.DB, jobID, bucket string, score int, userStatus string, code scoringschema.SignalCode, term string) {
	t.Helper()
	var row scoringstorage.ProfileMatch
	if err := db.Where("job_id = ?", jobID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Bucket != bucket || row.Score != score || row.UserStatus != userStatus {
		t.Fatalf("job %s bucket=%s score=%d user_status=%s", jobID, row.Bucket, row.Score, row.UserStatus)
	}
	var signals []scoringschema.Signal
	if err := json.Unmarshal(row.Signals, &signals); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 || signals[0].Code != code || len(signals[0].Terms) != 1 || signals[0].Terms[0] != term {
		t.Fatalf("job %s signals %#v", jobID, signals)
	}
}

type staticProfiles struct {
	profile profileschema.Profile
}

func (s staticProfiles) List(context.Context) ([]profileschema.Profile, error) {
	return []profileschema.Profile{s.profile}, nil
}

func (s staticProfiles) Get(_ context.Context, id string) (profileschema.Profile, error) {
	if id != s.profile.ID {
		return profileschema.Profile{}, profiles.ErrProfileNotFound
	}
	return s.profile, nil
}

type stubCollector struct {
	jobs []jobdata.Job
}

func (stubCollector) Name() string { return "stub" }

func (s stubCollector) Fetch(context.Context) ([]jobdata.Job, error) {
	return s.jobs, nil
}

func (s stubCollector) FetchWithQuery(context.Context, string) ([]jobdata.Job, error) {
	return s.jobs, nil
}

func testRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			sources TEXT NOT NULL DEFAULT '[]',
			title TEXT NOT NULL DEFAULT '',
			company TEXT NOT NULL DEFAULT '',
			company_key TEXT NOT NULL DEFAULT '',
			company_website TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			apply_url TEXT,
			description TEXT NOT NULL DEFAULT '',
			posted_at TIMESTAMP,
			location TEXT NOT NULL DEFAULT '{}',
			salary_raw TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			position TEXT,
			first_seen_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE profile_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMP NOT NULL,
			finished_at TIMESTAMP,
			jobs_scored INTEGER NOT NULL DEFAULT 0,
			sources_skipped INTEGER NOT NULL DEFAULT 0,
			idempotency_key TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE profile_matches (
			profile_id TEXT NOT NULL,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			bucket TEXT NOT NULL CHECK (bucket IN ('PASSED', 'REJECTED')),
			score INTEGER NOT NULL,
			signals TEXT NOT NULL,
			user_status TEXT NOT NULL DEFAULT 'NEW' CHECK (user_status IN ('NEW', 'HIDDEN')),
			run_id INTEGER NOT NULL REFERENCES profile_runs(id),
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (profile_id, job_id)
		)`,
		`CREATE TABLE ingest_watermarks (
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id)
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}
