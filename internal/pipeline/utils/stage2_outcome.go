package utils

import (
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
)

const stage2DuplicateRuleID = "duplicate"

// Stage2Evaluation is the outcome of evaluating all rules on one job.
type Stage2Evaluation struct {
	Status pipelineschema.RunJobStatus
	Hits   []pipeline.Stage2Hit
	Boost  int
}

// EvaluateStage2 parses the listing and evaluates rules in order.
func EvaluateStage2(j schema.Job, rules []pipelineschema.Stage2Rule, duplicateOf map[string]string) (Stage2Evaluation, error) {
	if duplicateOf != nil {
		if survivorID, dup := duplicateOf[j.ID]; dup {
			return Stage2Evaluation{
				Status: pipelineschema.RunJobRejectedStage2,
				Hits: []pipeline.Stage2Hit{{
					RuleID:  stage2DuplicateRuleID,
					Action:  string(pipelineschema.RuleActionReject),
					Matched: survivorID,
				}},
			}, nil
		}
	}
	parsed, err := ParseListing(j)
	if err != nil {
		return Stage2Evaluation{}, err
	}
	if len(rules) == 0 {
		return Stage2Evaluation{Status: pipelineschema.RunJobUnknownStage2}, nil
	}
	var hits []pipeline.Stage2Hit
	boost := 0
	var reject, passBoost bool
	for _, r := range rules {
		matched, ok := stage2RuleFires(parsed, j, r)
		if !ok {
			continue
		}
		hits = append(hits, pipeline.Stage2Hit{
			RuleID:  r.ID,
			Action:  string(r.Action),
			Matched: matched,
		})
		switch r.Action {
		case pipelineschema.RuleActionReject:
			reject = true
		case pipelineschema.RuleActionBoost:
			if r.Weight != nil && *r.Weight > 0 {
				passBoost = true
				boost += *r.Weight
			}
		case pipelineschema.RuleActionPenalty:
			if r.Weight != nil {
				boost += *r.Weight
			}
		}
	}
	var st pipelineschema.RunJobStatus
	switch {
	case reject:
		st = pipelineschema.RunJobRejectedStage2
	case passBoost:
		st = pipelineschema.RunJobPassedStage2
	default:
		st = pipelineschema.RunJobUnknownStage2
	}
	return Stage2Evaluation{Status: st, Hits: hits, Boost: boost}, nil
}

// Stage2StatusForJob returns only the status from [EvaluateStage2].
func Stage2StatusForJob(j schema.Job, rules []pipelineschema.Stage2Rule, duplicateOf map[string]string) pipelineschema.RunJobStatus {
	ev, err := EvaluateStage2(j, rules, duplicateOf)
	if err != nil {
		return pipelineschema.RunJobUnknownStage2
	}
	return ev.Status
}

// JobsAfterStage2 returns stage-1 jobs that are not rejected at stage 2, preserving order.
// When duplicateOf is nil, duplicates are computed from jobs.
func JobsAfterStage2(jobs []schema.Job, rules []pipelineschema.Stage2Rule, duplicateOf map[string]string) []schema.Job {
	if duplicateOf == nil {
		duplicateOf = DuplicateJobIDs(jobs)
	}
	out := make([]schema.Job, 0, len(jobs))
	for _, j := range jobs {
		if Stage2StatusForJob(j, rules, duplicateOf) != pipelineschema.RunJobRejectedStage2 {
			out = append(out, j)
		}
	}
	return out
}

func stage2RuleFires(parsed pipelineschema.ParsedListing, j schema.Job, r pipelineschema.Stage2Rule) (matched string, ok bool) {
	when, _ := r.When.FromValue(string(r.When))
	switch r.Field {
	case pipelineschema.RuleFieldPosition:
		return positionAnyMatch(parsed.Position, r.Values)
	case pipelineschema.RuleFieldCountriesAllowed:
		return countryRuleMatch(parsed.CountriesAllowed, r, when)
	case pipelineschema.RuleFieldTitle:
		return phraseFieldMatch(j.Title, r)
	case pipelineschema.RuleFieldBody:
		return phraseFieldMatch(j.Description, r)
	case pipelineschema.RuleFieldTitleBody:
		if m, hit := phraseFieldMatch(j.Title, r); hit {
			return m, true
		}
		return phraseFieldMatch(j.Description, r)
	default:
		return "", false
	}
}

func positionAnyMatch(position string, values []string) (string, bool) {
	position = strings.ToLower(strings.TrimSpace(position))
	if position == "" {
		return "", false
	}
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if position == v {
			return v, true
		}
	}
	return "", false
}

func countryRuleMatch(countries []string, r pipelineschema.Stage2Rule, when pipelineschema.RuleWhen) (string, bool) {
	if r.Op == pipelineschema.RuleOpExcludes {
		if when != pipelineschema.RuleWhenAlways && len(countries) == 0 {
			return "", false
		}
	}
	if r.Op == pipelineschema.RuleOpAny {
		return countryAnyMatch(countries, r.Values)
	}
	if r.Op == pipelineschema.RuleOpExcludes {
		return countryExcludesMatch(countries, r.Values)
	}
	return "", false
}

func phraseFieldMatch(text string, r pipelineschema.Stage2Rule) (string, bool) {
	if r.Op != pipelineschema.RuleOpPhrase {
		return "", false
	}
	ok, matched := phraseMatchesWithNegation(text, r.Values, r.NegationWindow)
	return matched, ok
}
