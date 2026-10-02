package utils

import (
	"fmt"
	"strings"
	"unicode"

	jobschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
)

const stage2ReservedDuplicateRuleID = "duplicate"

// ValidateStage2Rules checks semantic constraints not covered by JSON Schema alone.
func ValidateStage2Rules(rules []pipelineschema.Stage2Rule) error {
	seen := make(map[string]struct{}, len(rules))
	for i, r := range rules {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			return fmt.Errorf("rules[%d]: id is required", i)
		}
		if strings.EqualFold(id, stage2ReservedDuplicateRuleID) {
			return fmt.Errorf("rules[%d]: rule id %q is reserved", i, stage2ReservedDuplicateRuleID)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate rule id %q", id)
		}
		seen[id] = struct{}{}

		if _, err := r.Field.FromValue(string(r.Field)); err != nil {
			return fmt.Errorf("rules[%d]: %w", i, err)
		}
		if _, err := r.Op.FromValue(string(r.Op)); err != nil {
			return fmt.Errorf("rules[%d]: %w", i, err)
		}
		if _, err := r.Action.FromValue(string(r.Action)); err != nil {
			return fmt.Errorf("rules[%d]: %w", i, err)
		}
		if r.When != "" {
			if _, err := r.When.FromValue(string(r.When)); err != nil {
				return fmt.Errorf("rules[%d]: %w", i, err)
			}
		}
		if err := validateStage2FieldOp(i, r.Field, r.Op); err != nil {
			return err
		}
		if err := validateStage2Values(i, r.Field, r.Values); err != nil {
			return err
		}
		if err := validateStage2Weight(i, r.Action, r.Weight); err != nil {
			return err
		}
		if r.NegationWindow != 0 && r.Op != pipelineschema.RuleOpPhrase {
			return fmt.Errorf("rules[%d]: negation_window is only allowed when op is phrase", i)
		}
	}
	return nil
}

func validateStage2FieldOp(i int, field pipelineschema.RuleField, op pipelineschema.RuleOp) error {
	switch field {
	case pipelineschema.RuleFieldPosition:
		if op != pipelineschema.RuleOpAny {
			return fmt.Errorf("rules[%d]: field %q requires op %q", i, field, pipelineschema.RuleOpAny)
		}
	case pipelineschema.RuleFieldCountriesAllowed:
		if op != pipelineschema.RuleOpAny && op != pipelineschema.RuleOpExcludes {
			return fmt.Errorf("rules[%d]: field %q requires op any or excludes", i, field)
		}
	case pipelineschema.RuleFieldTitle, pipelineschema.RuleFieldBody, pipelineschema.RuleFieldTitleBody, pipelineschema.RuleFieldListing:
		if op != pipelineschema.RuleOpPhrase {
			return fmt.Errorf("rules[%d]: field %q requires op %q", i, field, pipelineschema.RuleOpPhrase)
		}
	default:
		return fmt.Errorf("rules[%d]: unknown field %q", i, field)
	}
	return nil
}

func validateStage2Values(i int, field pipelineschema.RuleField, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("rules[%d]: values must not be empty", i)
	}
	for j, v := range values {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("rules[%d]: values[%d] must not be empty", i, j)
		}
	}
	switch field {
	case pipelineschema.RuleFieldPosition:
		for j, v := range values {
			if _, err := jobschema.PositionLabel("").FromValue(v); err != nil {
				return fmt.Errorf("rules[%d]: values[%d]: %w", i, j, err)
			}
		}
	case pipelineschema.RuleFieldCountriesAllowed:
		if err := loadGeoRegions(); err != nil {
			return err
		}
		for j, v := range values {
			v = strings.TrimSpace(v)
			if isAlpha2CountryCode(v) {
				continue
			}
			if _, ok := ExpandRegionGroup(v); ok {
				continue
			}
			return fmt.Errorf("rules[%d]: values[%d]: must be ISO alpha-2 or a known geo region group", i, j)
		}
	}
	return nil
}

func isAlpha2CountryCode(s string) bool {
	if len(s) != 2 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func validateStage2Weight(i int, action pipelineschema.RuleAction, weight *int) error {
	switch action {
	case pipelineschema.RuleActionBoost:
		if weight == nil || *weight <= 0 {
			return fmt.Errorf("rules[%d]: boost requires weight > 0", i)
		}
	case pipelineschema.RuleActionPenalty:
		if weight == nil || *weight >= 0 {
			return fmt.Errorf("rules[%d]: penalty requires weight < 0", i)
		}
	case pipelineschema.RuleActionReject, pipelineschema.RuleActionFlag:
		if weight != nil {
			return fmt.Errorf("rules[%d]: %s must not include weight", i, action)
		}
	}
	return nil
}
