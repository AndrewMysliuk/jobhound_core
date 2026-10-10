package impl

import (
	"reflect"
	"testing"

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
