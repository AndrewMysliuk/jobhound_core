package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

func TestScoreJobs_happy(t *testing.T) {
	jobs := []domain.Job{{ID: "a", Title: "T"}, {ID: "b", Title: "U"}}
	scorer := stubScorer(func(_ context.Context, _ string, j domain.Job) (domain.ScoredJob, error) {
		return domain.ScoredJob{Job: j, Score: 50, Reason: "r"}, nil
	})
	out, err := ScoreJobs(context.Background(), "profile", jobs, scorer)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Job.ID != "a" || out[1].Score != 50 {
		t.Fatalf("got %+v", out)
	}
}

func TestScoreJobs_propagatesError(t *testing.T) {
	jobs := []domain.Job{{ID: "a"}, {ID: "b"}}
	want := errors.New("boom")
	n := 0
	scorer := stubScorer(func(_ context.Context, _ string, j domain.Job) (domain.ScoredJob, error) {
		n++
		if j.ID == "a" {
			return domain.ScoredJob{Job: j}, want
		}
		return domain.ScoredJob{Job: j}, nil
	})
	_, err := ScoreJobs(context.Background(), "", jobs, scorer)
	if !errors.Is(err, want) {
		t.Fatalf("got %v want %v", err, want)
	}
	if n != 1 {
		t.Fatalf("expected stop after first job, calls=%d", n)
	}
}

type stubScorer func(context.Context, string, domain.Job) (domain.ScoredJob, error)

func (f stubScorer) Score(ctx context.Context, profile string, job domain.Job) (domain.ScoredJob, error) {
	return f(ctx, profile, job)
}
