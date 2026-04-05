package utils

import (
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

// ApplyKeywordFilter returns jobs that pass stage 2 rules, preserving input order.
// Include constraints are evaluated first, then exclude (same outcome as a single pass:
// a job must satisfy all includes and must not match any exclude pattern).
func ApplyKeywordFilter(jobs []schema.Job, rules pipeline.KeywordRules) []schema.Job {
	inc := nonEmptyPatterns(rules.Include)
	exc := nonEmptyPatterns(rules.Exclude)

	out := make([]schema.Job, 0, len(jobs))
	for _, j := range jobs {
		hay := keywordHaystack(j)
		if !matchesAllIncludes(hay, inc) {
			continue
		}
		if matchesAnyExclude(hay, exc) {
			continue
		}
		out = append(out, j)
	}
	return out
}

func keywordHaystack(j schema.Job) string {
	return strings.ToLower(j.Title + " " + j.Description)
}

func nonEmptyPatterns(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matchesAllIncludes(hayLower string, includes []string) bool {
	for _, p := range includes {
		if !strings.Contains(hayLower, strings.ToLower(p)) {
			return false
		}
	}
	return true
}

func matchesAnyExclude(hayLower string, excludes []string) bool {
	for _, p := range excludes {
		if strings.Contains(hayLower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
