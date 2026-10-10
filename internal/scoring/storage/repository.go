package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrRunNotFound is returned when a profile_runs row does not exist.
	ErrRunNotFound = errors.New("scoring storage: run not found")
	// ErrMatchNotFound is returned when this profile has no match for the job.
	ErrMatchNotFound = errors.New("scoring storage: match not found")
	// ErrDuplicateIdempotencyKey is returned when profile_runs.idempotency_key is already taken.
	ErrDuplicateIdempotencyKey = errors.New("scoring storage: duplicate idempotency key")
)

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

// Repository persists profile runs and matches.
type Repository struct {
	get pgsql.GormGetter
}

// NewRepository wires scoring persistence.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// CreateRun inserts one profile_runs row. idempotency_key is unique.
func (r *Repository) CreateRun(ctx context.Context, run schema.Run) (schema.Run, error) {
	if run.ProfileID == "" {
		return schema.Run{}, fmt.Errorf("profile id is required")
	}
	if run.Status == "" {
		return schema.Run{}, fmt.Errorf("run status is required")
	}
	if run.IdempotencyKey == uuid.Nil {
		return schema.Run{}, fmt.Errorf("idempotency key is required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	row := newProfileRun(run)
	if err := r.get().WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicateKey(err) {
			return schema.Run{}, fmt.Errorf("%w: %w", ErrDuplicateIdempotencyKey, err)
		}
		return schema.Run{}, err
	}
	return row.toDomain()
}

// RunByIdempotencyKey loads the run inserted with this key.
func (r *Repository) RunByIdempotencyKey(ctx context.Context, key uuid.UUID) (schema.Run, error) {
	if key == uuid.Nil {
		return schema.Run{}, fmt.Errorf("idempotency key is required")
	}
	return r.firstRun(ctx, "idempotency_key = ?", key.String())
}

// RunningRun returns the newest RUNNING row for the profile.
func (r *Repository) RunningRun(ctx context.Context, profileID string) (schema.Run, error) {
	if profileID == "" {
		return schema.Run{}, fmt.Errorf("profile id is required")
	}
	return r.firstRun(ctx, "profile_id = ? AND status = ?", profileID, string(schema.RunStatusRunning))
}

// LatestRun returns the newest run for the profile.
func (r *Repository) LatestRun(ctx context.Context, profileID string) (schema.Run, error) {
	if profileID == "" {
		return schema.Run{}, fmt.Errorf("profile id is required")
	}
	return r.firstRun(ctx, "profile_id = ?", profileID)
}

func (r *Repository) firstRun(ctx context.Context, query string, args ...any) (schema.Run, error) {
	var row ProfileRun
	err := r.get().WithContext(ctx).Where(query, args...).Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.Run{}, ErrRunNotFound
		}
		return schema.Run{}, err
	}
	return row.toDomain()
}

// GetRun loads one profile_runs row by id.
func (r *Repository) GetRun(ctx context.Context, id int64) (schema.Run, error) {
	if id == 0 {
		return schema.Run{}, fmt.Errorf("run id is required")
	}
	var row ProfileRun
	err := r.get().WithContext(ctx).First(&row, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.Run{}, ErrRunNotFound
		}
		return schema.Run{}, err
	}
	return row.toDomain()
}

// FinishRun sets status, finished_at, and the two counters. It does not touch matches.
func (r *Repository) FinishRun(ctx context.Context, id int64, status schema.RunStatus, jobsScored, sourcesSkipped int, finishedAt time.Time) error {
	if id == 0 {
		return fmt.Errorf("run id is required")
	}
	if status == "" {
		return fmt.Errorf("run status is required")
	}
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	res := r.get().WithContext(ctx).Model(&ProfileRun{}).Where("id = ?", id).Updates(map[string]any{
		"status":          string(status),
		"finished_at":     finishedAt.UTC(),
		"jobs_scored":     jobsScored,
		"sources_skipped": sourcesSkipped,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRunNotFound
	}
	return nil
}

// SetRunCounters writes jobs_scored and sources_skipped on a still-running row.
func (r *Repository) SetRunCounters(ctx context.Context, id int64, jobsScored, sourcesSkipped int) error {
	if id == 0 {
		return fmt.Errorf("run id is required")
	}
	res := r.get().WithContext(ctx).Model(&ProfileRun{}).
		Where("id = ? AND status = ?", id, string(schema.RunStatusRunning)).
		Updates(map[string]any{
			"jobs_scored":     jobsScored,
			"sources_skipped": sourcesSkipped,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRunNotFound
	}
	return nil
}

// UpsertMatch inserts a match or overwrites bucket, score, signals, run_id, and updated_at.
// user_status is written on insert only. A later upsert leaves it unchanged.
func (r *Repository) UpsertMatch(ctx context.Context, m schema.Match) error {
	if m.ProfileID == "" {
		return fmt.Errorf("profile id is required")
	}
	if m.JobID == "" {
		return fmt.Errorf("job id is required")
	}
	if m.RunID == 0 {
		return fmt.Errorf("run id is required")
	}
	if m.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	signals, err := encodeSignals(m.Signals)
	if err != nil {
		return err
	}
	row := newProfileMatch(m, signals, time.Now().UTC())
	return r.get().WithContext(ctx).
		Select("ProfileID", "JobID", "Bucket", "Score", "Signals", "UserStatus", "RunID", "UpdatedAt").
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "profile_id"}, {Name: "job_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"bucket", "score", "signals", "run_id", "updated_at",
			}),
		}).
		Create(&row).Error
}

// List returns one page of matches for the profile.
// An empty bucket means PASSED. An empty user status means NEW.
// Page less than 1 is 1. Limit less than 1 is 50, and limit above 100 is 100.
// Order is score DESC, jobs.first_seen_at DESC.
func (r *Repository) List(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error) {
	if p.ProfileID == "" {
		return schema.JobPage{}, fmt.Errorf("profile id is required")
	}
	bucket := p.Bucket
	if bucket == "" {
		bucket = schema.BucketPassed
	}
	status := p.UserStatus
	if status == "" {
		status = schema.UserStatusNew
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	limit := p.Limit
	if limit < 1 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	base := r.listQuery(ctx, p.ProfileID, string(bucket), string(status))
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return schema.JobPage{}, err
	}

	var rows []listRow
	err := r.listQuery(ctx, p.ProfileID, string(bucket), string(status)).
		Select(`profile_matches.job_id, jobs.title, jobs.company, jobs.url, jobs.apply_url,
			jobs.location, jobs.sources, jobs.posted_at, jobs.first_seen_at, jobs.last_seen_at,
			profile_matches.bucket, profile_matches.score, profile_matches.signals, profile_matches.user_status`).
		Order("profile_matches.score DESC, jobs.first_seen_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return schema.JobPage{}, err
	}

	jobs := make([]schema.ListedJob, 0, len(rows))
	for _, row := range rows {
		item, err := row.toListedJob()
		if err != nil {
			return schema.JobPage{}, err
		}
		jobs = append(jobs, item)
	}
	return schema.JobPage{
		Page:  page,
		Limit: limit,
		Total: int(total),
		Jobs:  jobs,
	}, nil
}

func (r *Repository) listQuery(ctx context.Context, profileID, bucket, status string) *gorm.DB {
	return r.get().WithContext(ctx).
		Table("profile_matches").
		Joins("JOIN jobs ON jobs.id = profile_matches.job_id").
		Where("profile_matches.profile_id = ?", profileID).
		Where("profile_matches.bucket = ?", bucket).
		Where("profile_matches.user_status = ?", status)
}

type listRow struct {
	JobID       string     `gorm:"column:job_id"`
	Title       string     `gorm:"column:title"`
	Company     string     `gorm:"column:company"`
	URL         string     `gorm:"column:url"`
	ApplyURL    *string    `gorm:"column:apply_url"`
	Location    []byte     `gorm:"column:location"`
	Sources     []byte     `gorm:"column:sources"`
	PostedAt    *time.Time `gorm:"column:posted_at"`
	FirstSeenAt time.Time  `gorm:"column:first_seen_at"`
	LastSeenAt  time.Time  `gorm:"column:last_seen_at"`
	Bucket      string     `gorm:"column:bucket"`
	Score       int        `gorm:"column:score"`
	Signals     []byte     `gorm:"column:signals"`
	UserStatus  string     `gorm:"column:user_status"`
}

func (row listRow) toListedJob() (schema.ListedJob, error) {
	loc, err := decodeLocation(row.Location)
	if err != nil {
		return schema.ListedJob{}, err
	}
	sources, err := decodeSources(row.Sources)
	if err != nil {
		return schema.ListedJob{}, err
	}
	signals, err := decodeSignals(row.Signals)
	if err != nil {
		return schema.ListedJob{}, err
	}
	item := schema.ListedJob{
		JobID:       row.JobID,
		Title:       row.Title,
		Company:     row.Company,
		URL:         row.URL,
		Location:    loc,
		Sources:     sources,
		FirstSeenAt: row.FirstSeenAt,
		LastSeenAt:  row.LastSeenAt,
		Bucket:      schema.Bucket(row.Bucket),
		Score:       row.Score,
		Signals:     signals,
		UserStatus:  schema.UserStatus(row.UserStatus),
	}
	if row.ApplyURL != nil {
		item.ApplyURL = *row.ApplyURL
	}
	if row.PostedAt != nil {
		item.PostedAt = *row.PostedAt
	}
	return item, nil
}

// SetUserStatus writes user_status on an existing match and returns the listed job.
func (r *Repository) SetUserStatus(ctx context.Context, profileID, jobID string, status schema.UserStatus) (schema.ListedJob, error) {
	if profileID == "" || jobID == "" {
		return schema.ListedJob{}, fmt.Errorf("profile id and job id are required")
	}
	if status == "" {
		return schema.ListedJob{}, fmt.Errorf("user status is required")
	}
	var n int64
	err := r.get().WithContext(ctx).Model(&ProfileMatch{}).
		Where("profile_id = ? AND job_id = ?", profileID, jobID).
		Count(&n).Error
	if err != nil {
		return schema.ListedJob{}, err
	}
	if n == 0 {
		return schema.ListedJob{}, ErrMatchNotFound
	}
	err = r.get().WithContext(ctx).Model(&ProfileMatch{}).
		Where("profile_id = ? AND job_id = ?", profileID, jobID).
		Update("user_status", string(status)).Error
	if err != nil {
		return schema.ListedJob{}, err
	}
	return r.listedJob(ctx, profileID, jobID)
}

func (r *Repository) listedJob(ctx context.Context, profileID, jobID string) (schema.ListedJob, error) {
	var row listRow
	err := r.get().WithContext(ctx).
		Table("profile_matches").
		Joins("JOIN jobs ON jobs.id = profile_matches.job_id").
		Select(`profile_matches.job_id, jobs.title, jobs.company, jobs.url, jobs.apply_url,
			jobs.location, jobs.sources, jobs.posted_at, jobs.first_seen_at, jobs.last_seen_at,
			profile_matches.bucket, profile_matches.score, profile_matches.signals, profile_matches.user_status`).
		Where("profile_matches.profile_id = ? AND profile_matches.job_id = ?", profileID, jobID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.ListedJob{}, ErrMatchNotFound
		}
		return schema.ListedJob{}, err
	}
	return row.toListedJob()
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
}
