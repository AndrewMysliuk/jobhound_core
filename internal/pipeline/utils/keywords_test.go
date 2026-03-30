package utils

import (
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

func TestApplyKeywordFilter_emptyLists(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Title: "Go Developer", Description: "backend"},
		{ID: "2", Title: "Rust", Description: "systems"},
	}
	got := ApplyKeywordFilter(jobs, pipeline.KeywordRules{})
	if len(got) != 2 {
		t.Fatalf("want both jobs, got %d", len(got))
	}
}

func TestApplyKeywordFilter_onlyInclude(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Title: "Senior Go Developer", Description: "microservices"},
		{ID: "2", Title: "Frontend", Description: "React only"},
		{ID: "3", Title: "Go", Description: "no keyword here for second include"},
	}
	rules := pipeline.KeywordRules{Include: []string{"go", "micro"}}
	got := ApplyKeywordFilter(jobs, rules)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("want job 1 only, got %+v", got)
	}
}

func TestApplyKeywordFilter_onlyExclude(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Title: "Engineer", Description: "full-time permanent"},
		{ID: "2", Title: "Contractor role", Description: "6 months"},
	}
	rules := pipeline.KeywordRules{Exclude: []string{"contract"}}
	got := ApplyKeywordFilter(jobs, rules)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("want job 1 only, got %+v", got)
	}
}

func TestApplyKeywordFilter_combinedIncludeThenExclude(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Title: "Go Developer", Description: "Kubernetes and Go"},
		{ID: "2", Title: "Go Developer", Description: "legacy PHP only"},
	}
	rules := pipeline.KeywordRules{
		Include: []string{"go"},
		Exclude: []string{"php"},
	}
	got := ApplyKeywordFilter(jobs, rules)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("want job 1 only, got %+v", got)
	}
}

func TestApplyKeywordFilter_caseInsensitive(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Title: "PYTHON", Description: "data"},
	}
	rules := pipeline.KeywordRules{Include: []string{"python"}}
	got := ApplyKeywordFilter(jobs, rules)
	if len(got) != 1 {
		t.Fatal("expected include to match case-insensitively")
	}
}

func TestApplyKeywordFilter_preservesOrder(t *testing.T) {
	jobs := []domain.Job{
		{ID: "a", Title: "x", Description: "alpha beta"},
		{ID: "b", Title: "y", Description: "alpha gamma"},
	}
	rules := pipeline.KeywordRules{Include: []string{"alpha"}}
	got := ApplyKeywordFilter(jobs, rules)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("order: %+v", got)
	}
}

func TestApplyKeywordFilter_trimsEmptyPatterns(t *testing.T) {
	jobs := []domain.Job{{ID: "1", Title: "Hi", Description: "there"}}
	rules := pipeline.KeywordRules{
		Include: []string{"  ", "hi"},
		Exclude: []string{"", "nomatch"},
	}
	got := ApplyKeywordFilter(jobs, rules)
	if len(got) != 1 {
		t.Fatalf("want 1 job, got %d", len(got))
	}
}
