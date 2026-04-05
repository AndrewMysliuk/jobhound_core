package impl

import (
	"context"
	"strings"
	"testing"
	"time"

	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	pipelinestorage "github.com/andrewmysliuk/jobhound_core/internal/pipeline/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotstorage "github.com/andrewmysliuk/jobhound_core/internal/slots/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeTemporal struct {
	describe *client.WorkflowExecutionDescription
	descErr  error

	gotWorkflow interface{}
	gotArgs     []interface{}
	execErr     error
}

func (f *fakeTemporal) DescribeWorkflow(context.Context, string, string) (*client.WorkflowExecutionDescription, error) {
	if f.descErr != nil {
		return nil, f.descErr
	}
	return f.describe, nil
}

func (f *fakeTemporal) ExecuteWorkflow(_ context.Context, _ client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	f.gotWorkflow = workflow
	f.gotArgs = args
	return nil, f.execErr
}

func (f *fakeTemporal) TerminateWorkflow(context.Context, string, string, string, ...interface{}) error {
	return nil
}

type stubProfiles struct {
	text string
}

func (s stubProfiles) GetText(context.Context) (string, error) {
	return s.text, nil
}

func stageRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	mem := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:stage_runs_"+mem+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	for _, s := range []string{
		`CREATE TABLE slots (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE pipeline_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP NOT NULL,
			slot_id TEXT,
			broad_filter_key_hash TEXT
		)`,
		`CREATE TABLE pipeline_run_jobs (
			pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
			job_id TEXT NOT NULL,
			status TEXT NOT NULL,
			stage3_rationale TEXT,
			PRIMARY KEY (pipeline_run_id, job_id)
		)`,
	} {
		require.NoError(t, db.Exec(s).Error)
	}
	return db
}

func TestRunStage2_startsWorkflowWithKeywordRules(t *testing.T) {
	ctx := context.Background()
	db := stageRunTestDB(t)
	slotID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO slots (id, name, created_at) VALUES (?, 's', ?)`, slotID.String(), now).Error)

	ft := &fakeTemporal{describe: &client.WorkflowExecutionDescription{
		WorkflowExecutionMetadata: client.WorkflowExecutionMetadata{
			Status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		},
	}}
	svc := NewService(
		slotstorage.NewRepository(pgsql.NewGetter(db)),
		nil,
		pipelinestorage.NewRepository(pgsql.NewGetter(db)),
		stubProfiles{text: "p"},
		ft,
		"q",
		[]string{"src"},
		logging.Nop(),
	)
	out, err := svc.RunStage2(ctx, slotID.String(), []string{"a", "b"}, []string{"x"})
	require.NoError(t, err)
	require.Equal(t, slotID.String(), out.SlotID)
	require.Equal(t, 2, out.Stage)
	require.Equal(t, manualschema.ManualSlotRunWorkflowName, ft.gotWorkflow)
	require.Len(t, ft.gotArgs, 1)
	in, ok := ft.gotArgs[0].(manualschema.ManualSlotRunWorkflowInput)
	require.True(t, ok)
	require.Equal(t, manualschema.RunKindPipelineStage2, in.Kind)
	require.Equal(t, []string{"a", "b"}, in.KeywordRules.Include)
	require.Equal(t, []string{"x"}, in.KeywordRules.Exclude)
}

func TestRunStage2_stageAlreadyRunning(t *testing.T) {
	ctx := context.Background()
	db := stageRunTestDB(t)
	slotID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO slots (id, name, created_at) VALUES (?, 's', ?)`, slotID.String(), now).Error)

	ft := &fakeTemporal{describe: &client.WorkflowExecutionDescription{
		WorkflowExecutionMetadata: client.WorkflowExecutionMetadata{
			Status: enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
		},
	}}
	svc := NewService(
		slotstorage.NewRepository(pgsql.NewGetter(db)),
		nil,
		pipelinestorage.NewRepository(pgsql.NewGetter(db)),
		stubProfiles{},
		ft,
		"q",
		[]string{"src"},
		logging.Nop(),
	)
	_, err := svc.RunStage2(ctx, slotID.String(), []string{"a"}, []string{"b"})
	require.ErrorIs(t, err, slots.ErrStageAlreadyRunning)
	require.Nil(t, ft.gotWorkflow)
}

func TestRunStage3_startsWorkflowWithRunKindAndMaxJobs(t *testing.T) {
	ctx := context.Background()
	db := stageRunTestDB(t)
	slotID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO slots (id, name, created_at) VALUES (?, 's', ?)`, slotID.String(), now).Error)
	require.NoError(t, db.Exec(`INSERT INTO pipeline_runs (created_at, slot_id) VALUES (?, ?)`, now, slotID.String()).Error)

	ft := &fakeTemporal{describe: &client.WorkflowExecutionDescription{
		WorkflowExecutionMetadata: client.WorkflowExecutionMetadata{
			Status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		},
	}}
	svc := NewService(
		slotstorage.NewRepository(pgsql.NewGetter(db)),
		nil,
		pipelinestorage.NewRepository(pgsql.NewGetter(db)),
		stubProfiles{text: "my profile"},
		ft,
		"q",
		[]string{"src"},
		logging.Nop(),
	)
	out, err := svc.RunStage3(ctx, slotID.String(), 7)
	require.NoError(t, err)
	require.Equal(t, 3, out.Stage)
	require.Equal(t, manualschema.ManualSlotRunWorkflowName, ft.gotWorkflow)
	in := ft.gotArgs[0].(manualschema.ManualSlotRunWorkflowInput)
	require.Equal(t, manualschema.RunKindPipelineStage3, in.Kind)
	require.Equal(t, "my profile", in.Profile)
	require.Equal(t, 7, in.Stage3MaxJobs)
	require.NotNil(t, in.PipelineRunID)
	require.Equal(t, int64(1), *in.PipelineRunID)
}

func TestRunStage3_profileRequired(t *testing.T) {
	ctx := context.Background()
	db := stageRunTestDB(t)
	slotID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO slots (id, name, created_at) VALUES (?, 's', ?)`, slotID.String(), now).Error)
	require.NoError(t, db.Exec(`INSERT INTO pipeline_runs (created_at, slot_id) VALUES (?, ?)`, now, slotID.String()).Error)

	ft := &fakeTemporal{describe: &client.WorkflowExecutionDescription{
		WorkflowExecutionMetadata: client.WorkflowExecutionMetadata{
			Status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		},
	}}
	svc := NewService(
		slotstorage.NewRepository(pgsql.NewGetter(db)),
		nil,
		pipelinestorage.NewRepository(pgsql.NewGetter(db)),
		stubProfiles{text: "   "},
		ft,
		"q",
		[]string{"src"},
		logging.Nop(),
	)
	_, err := svc.RunStage3(ctx, slotID.String(), 5)
	require.ErrorIs(t, err, slots.ErrProfileRequired)
}

func TestRunStage3_noPipelineRun(t *testing.T) {
	ctx := context.Background()
	db := stageRunTestDB(t)
	slotID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO slots (id, name, created_at) VALUES (?, 's', ?)`, slotID.String(), now).Error)

	ft := &fakeTemporal{describe: &client.WorkflowExecutionDescription{
		WorkflowExecutionMetadata: client.WorkflowExecutionMetadata{
			Status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		},
	}}
	svc := NewService(
		slotstorage.NewRepository(pgsql.NewGetter(db)),
		nil,
		pipelinestorage.NewRepository(pgsql.NewGetter(db)),
		stubProfiles{text: "p"},
		ft,
		"q",
		[]string{"src"},
		logging.Nop(),
	)
	_, err := svc.RunStage3(ctx, slotID.String(), 5)
	require.ErrorIs(t, err, slots.ErrNoPipelineRun)
}
