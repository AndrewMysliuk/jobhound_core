package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

// ValidateBroadFilterRules returns an error when explicit date bounds are inconsistent.
func ValidateBroadFilterRules(rules pipeline.BroadFilterRules) error {
	if (rules.From == nil) != (rules.To == nil) {
		return fmt.Errorf("pipeline utils: BroadFilterRules: From and To must both be set or both unset")
	}
	if rules.From != nil && rules.To != nil && rules.From.After(*rules.To) {
		return fmt.Errorf("pipeline utils: BroadFilterRules: From after To")
	}
	return nil
}

// ApplyBroadFilter returns jobs that pass stage 1 rules, preserving input order.
// clock supplies "now" for the default 7-day window when From/To are unset; if nil, time.Now is used.
func ApplyBroadFilter(clock func() time.Time, rules pipeline.BroadFilterRules, jobs []domain.Job) ([]domain.Job, error) {
	if err := ValidateBroadFilterRules(rules); err != nil {
		return nil, err
	}
	if clock == nil {
		clock = time.Now
	}

	now := clock().UTC()
	var winFrom, winTo time.Time
	if rules.From == nil {
		winTo = now
		winFrom = now.Add(-7 * 24 * time.Hour)
	} else {
		winFrom = rules.From.UTC()
		winTo = rules.To.UTC()
	}

	syns := effectiveSynonyms(rules.RoleSynonyms)
	allow := effectiveCountryAllowlist(rules.CountryAllowlist)

	out := make([]domain.Job, 0, len(jobs))
	for _, j := range jobs {
		if !postedInWindow(j, winFrom, winTo) {
			continue
		}
		if !matchesRoleSynonyms(j, syns) {
			continue
		}
		if !passesRemoteRule(j, rules.RemoteOnly) {
			continue
		}
		if !passesCountryRule(j, allow) {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

func postedInWindow(j domain.Job, windowFrom, windowTo time.Time) bool {
	if j.PostedAt.IsZero() {
		return false
	}
	posted := j.PostedAt.UTC()
	return !posted.Before(windowFrom) && !posted.After(windowTo)
}

func effectiveSynonyms(synonyms []string) []string {
	var out []string
	for _, s := range synonyms {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func matchesRoleSynonyms(j domain.Job, synonyms []string) bool {
	if len(synonyms) == 0 {
		return true
	}
	hay := strings.ToLower(j.Title + " " + j.Description)
	for _, syn := range synonyms {
		if strings.Contains(hay, strings.ToLower(syn)) {
			return true
		}
	}
	return false
}

func passesRemoteRule(j domain.Job, remoteOnly bool) bool {
	if !remoteOnly {
		return true
	}
	return j.Remote != nil && *j.Remote
}

func effectiveCountryAllowlist(list []string) []string {
	var out []string
	for _, c := range list {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, strings.ToUpper(c))
		}
	}
	return out
}

func passesCountryRule(j domain.Job, allowUpper []string) bool {
	if len(allowUpper) == 0 {
		return true
	}
	if strings.TrimSpace(j.CountryCode) == "" {
		return false
	}
	code := strings.ToUpper(strings.TrimSpace(j.CountryCode))
	for _, a := range allowUpper {
		if code == a {
			return true
		}
	}
	return false
}
