package impl

import (
	"reflect"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()

	query := func(terms ...string) schema.Signal {
		return schema.Signal{Code: schema.SignalQuery, Points: schema.QueryPoints, Terms: terms}
	}
	penalty := func(terms ...string) schema.Signal {
		return schema.Signal{Code: schema.SignalPenalty, Points: schema.PenaltyPoints, Terms: terms}
	}
	cutTitle := func(phrase string) schema.Signal {
		return schema.Signal{Code: schema.SignalCutExcludeTitle, Terms: []string{phrase}}
	}
	cutText := func(phrase string) schema.Signal {
		return schema.Signal{Code: schema.SignalCutExcludeText, Terms: []string{phrase}}
	}
	cutLocation := func(terms ...string) schema.Signal {
		return schema.Signal{Code: schema.SignalCutLocation, Terms: terms}
	}
	locations := profileschema.Profile{
		LocationCountries: []string{"US", "DE", "RO", "GB"},
		LocationRegions:   []string{"WORLDWIDE", "EUROPE", "EMEA"},
		Queries:           []string{"vue"},
	}

	tests := []struct {
		name    string
		profile profileschema.Profile
		job     jobdata.Job
		want    Outcome
	}{
		{
			name: "title exclude first",
			profile: profileschema.Profile{
				ExcludeTitle: []string{"intern", "recruiter"},
				ExcludeText:  []string{"php"},
				Queries:      []string{"vue"},
				Penalties:    []string{"work permit required"},
			},
			job: jobdata.Job{
				Title:       "RECRUITER",
				Description: "php, vue, and work permit required",
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutTitle("recruiter")},
			},
		},
		{
			name: "exclude title ignores the description",
			profile: profileschema.Profile{
				ExcludeTitle: []string{"recruiter"},
				Queries:      []string{"vue"},
			},
			job: jobdata.Job{
				Title:       "Engineer",
				Description: "a recruiter will send the vue test",
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("vue")},
			},
		},
		{
			name: "text exclude",
			profile: profileschema.Profile{
				ExcludeTitle: []string{"recruiter"},
				ExcludeText:  []string{"php"},
				Queries:      []string{"vue"},
			},
			job: jobdata.Job{
				Title:       "Backend Engineer",
				Description: "Experience with php and vue",
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutText("php")},
			},
		},
		{
			name: "text exclude in the title",
			profile: profileschema.Profile{
				ExcludeTitle: []string{"recruiter"},
				ExcludeText:  []string{"php"},
			},
			job: jobdata.Job{Title: "PHP Developer"},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutText("php")},
			},
		},
		{
			name: "penalty keeps passed",
			profile: profileschema.Profile{
				Penalties: []string{"work permit required"},
				Queries:   []string{"vue"},
			},
			job: jobdata.Job{
				Title:       "Backend Engineer",
				Description: "Work permit required. Work permit required. vue",
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints + schema.PenaltyPoints,
				Signals: []schema.Signal{query("vue"), penalty("work permit required")},
			},
		},
		{
			name: "each query phrase once",
			profile: profileschema.Profile{
				Queries: []string{"vue", "react", "golang"},
			},
			job: jobdata.Job{
				Title:       "Vue Vue Engineer",
				Description: "react and react and vue",
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   2 * schema.QueryPoints,
				Signals: []schema.Signal{query("vue"), query("react")},
			},
		},
		{
			name: "case fold",
			profile: profileschema.Profile{
				Queries: []string{"AI native"},
			},
			job: jobdata.Job{
				Title:       "Engineer",
				Description: "Building an ai native product",
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("AI native")},
			},
		},
		{
			name: "java does not match javascript",
			profile: profileschema.Profile{
				ExcludeText: []string{"java"},
				Queries:     []string{"java"},
			},
			job: jobdata.Job{
				Title:       "Engineer",
				Description: "We use javascript",
			},
			want: Outcome{Bucket: schema.BucketPassed},
		},
		{
			name: "java matches a bounded word",
			profile: profileschema.Profile{
				ExcludeText: []string{"java"},
				Queries:     []string{"java"},
			},
			job: jobdata.Job{
				Title:       "Engineer",
				Description: "We use Java.",
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutText("java")},
			},
		},
		{
			name: "frontend adds nothing",
			profile: profileschema.Profile{
				Queries:        []string{"vue", "typescript", "react", "golang", "node"},
				WellfoundRoles: []string{"frontend-engineer"},
			},
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "frontend-engineer, frontend work",
			},
			want: Outcome{Bucket: schema.BucketPassed},
		},
		{
			name: "frontend scores when it is a query",
			profile: profileschema.Profile{
				Queries: []string{"frontend"},
			},
			job: jobdata.Job{Title: "Frontend Engineer"},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("frontend")},
			},
		},
		{
			name: "location raw is an exclude",
			profile: profileschema.Profile{
				ExcludeText: []string{"india"},
				Queries:     []string{"vue"},
			},
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue",
				Location:    jobdata.Location{Raw: "India"},
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutText("india")},
			},
		},
		{
			name:    "empty location stays",
			profile: locations,
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue",
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("vue")},
			},
		},
		{
			name:    "allowed country stays",
			profile: locations,
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue and an office in India",
				Location:    jobdata.Location{Countries: []string{"DE", "BR"}},
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("vue")},
			},
		},
		{
			name:    "allowed region stays",
			profile: locations,
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue",
				Location:    jobdata.Location{Regions: []string{"EMEA"}},
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("vue")},
			},
		},
		{
			name:    "worldwide stays",
			profile: locations,
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue",
				Location:    jobdata.Location{Regions: []string{"worldwide"}},
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("vue")},
			},
		},
		{
			name:    "location outside the lists is rejected",
			profile: locations,
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue",
				Location:    jobdata.Location{Countries: []string{"BR", "AR"}, Regions: []string{"LATAM"}},
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutLocation("BR", "AR", "LATAM")},
			},
		},
		{
			name: "text exclude still wins over an allowed country",
			profile: profileschema.Profile{
				LocationCountries: []string{"DE"},
				ExcludeText:       []string{"php"},
				Queries:           []string{"vue"},
			},
			job: jobdata.Job{
				Title:       "Backend Engineer",
				Description: "php and vue",
				Location:    jobdata.Location{Countries: []string{"DE"}},
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{cutText("php")},
			},
		},
		{
			name: "empty location lists skip the check",
			profile: profileschema.Profile{
				Queries: []string{"vue"},
			},
			job: jobdata.Job{
				Title:       "Frontend Engineer",
				Description: "vue",
				Location:    jobdata.Location{Countries: []string{"BR"}},
			},
			want: Outcome{
				Bucket:  schema.BucketPassed,
				Score:   schema.QueryPoints,
				Signals: []schema.Signal{query("vue")},
			},
		},
		{
			name: "score 0 is passed",
			profile: profileschema.Profile{
				Queries:      []string{"vue"},
				ExcludeTitle: []string{"recruiter"},
				ExcludeText:  []string{"php"},
				Penalties:    []string{"work permit required"},
			},
			job: jobdata.Job{
				Title:       "Platform Engineer",
				Description: "Build internal tools",
			},
			want: Outcome{Bucket: schema.BucketPassed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Evaluate(tt.profile, tt.job)
			if got.Bucket != tt.want.Bucket || got.Score != tt.want.Score || !reflect.DeepEqual(got.Signals, tt.want.Signals) {
				t.Fatalf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_postedAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	month := time.Duration(schema.PostedAtRejectDays) * 24 * time.Hour
	profile := profileschema.Profile{
		ExcludeTitle: []string{"recruiter"},
		Queries:      []string{"vue"},
	}
	passed := Outcome{
		Bucket:  schema.BucketPassed,
		Score:   schema.QueryPoints,
		Signals: []schema.Signal{{Code: schema.SignalQuery, Points: schema.QueryPoints, Terms: []string{"vue"}}},
	}

	tests := []struct {
		name string
		job  jobdata.Job
		want Outcome
	}{
		{
			name: "unknown posted_at is scored",
			job:  jobdata.Job{Title: "Vue Engineer", Description: "vue"},
			want: passed,
		},
		{
			name: "posted inside a month is scored",
			job: jobdata.Job{
				Title:       "Vue Engineer",
				Description: "vue",
				PostedAt:    now.Add(-month).Add(time.Second),
			},
			want: passed,
		},
		{
			name: "posted exactly a month ago is scored",
			job: jobdata.Job{
				Title:       "Vue Engineer",
				Description: "vue",
				PostedAt:    now.Add(-month),
			},
			want: passed,
		},
		{
			name: "posted older than a month is rejected before title excludes",
			job: jobdata.Job{
				Title:       "Recruiter",
				Description: "vue",
				PostedAt:    now.Add(-month).Add(-time.Second),
			},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{{
					Code:  schema.SignalCutPostedAt,
					Terms: []string{now.Add(-month).Add(-time.Second).Format(time.RFC3339)},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := evaluateAt(profile, tt.job, now)
			if got.Bucket != tt.want.Bucket || got.Score != tt.want.Score || !reflect.DeepEqual(got.Signals, tt.want.Signals) {
				t.Fatalf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}
