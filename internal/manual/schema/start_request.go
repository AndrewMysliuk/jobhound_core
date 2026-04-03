package schema

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/google/uuid"
)

// ManualSlotRunStartRequest is the stable JSON body for starting a manual slot run (009); maps to [ManualSlotRunWorkflowInput].
type ManualSlotRunStartRequest struct {
	SlotID             string                `json:"slot_id"`
	UserID             *string               `json:"user_id,omitempty"`
	Kind               RunKind               `json:"kind"`
	Profile            string                `json:"profile,omitempty"`
	SourceIDs          []string              `json:"source_ids,omitempty"`
	ExplicitRefresh    bool                  `json:"explicit_refresh,omitempty"`
	BroadRules         *BroadFilterRulesJSON `json:"broad_rules,omitempty"`
	KeywordRules       *KeywordRulesJSON     `json:"keyword_rules,omitempty"`
	BroadFilterKeyHash string                `json:"broad_filter_key_hash,omitempty"`
	PipelineRunID      *int64                `json:"pipeline_run_id,omitempty"`
}

// ManualSlotRunStartResponse is the JSON response for a completed manual slot run (009); same fields as [ManualSlotRunAggregate].
type ManualSlotRunStartResponse = ManualSlotRunAggregate

// BroadFilterRulesJSON mirrors [pipeline.BroadFilterRules] with RFC3339/RFC3339Nano strings for date bounds.
type BroadFilterRulesJSON struct {
	From             *string  `json:"from,omitempty"`
	To               *string  `json:"to,omitempty"`
	RoleSynonyms     []string `json:"role_synonyms,omitempty"`
	RemoteOnly       bool     `json:"remote_only,omitempty"`
	CountryAllowlist []string `json:"country_allowlist,omitempty"`
}

// KeywordRulesJSON mirrors [pipeline.KeywordRules] for JSON boundaries.
type KeywordRulesJSON struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// ToPipeline converts JSON DTOs into pipeline broad-filter rules.
func (j *BroadFilterRulesJSON) ToPipeline() (pipeline.BroadFilterRules, error) {
	if j == nil {
		return pipeline.BroadFilterRules{}, nil
	}
	out := pipeline.BroadFilterRules{
		RoleSynonyms:     append([]string(nil), j.RoleSynonyms...),
		RemoteOnly:       j.RemoteOnly,
		CountryAllowlist: append([]string(nil), j.CountryAllowlist...),
	}
	from, err := parseRFC3339Ptr(j.From)
	if err != nil {
		return out, fmt.Errorf("broad_rules.from: %w", err)
	}
	to, err := parseRFC3339Ptr(j.To)
	if err != nil {
		return out, fmt.Errorf("broad_rules.to: %w", err)
	}
	out.From = from
	out.To = to
	return out, nil
}

// ToPipeline converts JSON DTOs into pipeline keyword rules.
func (j *KeywordRulesJSON) ToPipeline() pipeline.KeywordRules {
	if j == nil {
		return pipeline.KeywordRules{}
	}
	return pipeline.KeywordRules{
		Include: append([]string(nil), j.Include...),
		Exclude: append([]string(nil), j.Exclude...),
	}
}

// ToWorkflowInput builds workflow input from the HTTP-oriented request (009).
func (req ManualSlotRunStartRequest) ToWorkflowInput() (ManualSlotRunWorkflowInput, error) {
	slotID, err := uuid.Parse(strings.TrimSpace(req.SlotID))
	if err != nil {
		return ManualSlotRunWorkflowInput{}, fmt.Errorf("slot_id: %w", err)
	}
	in := ManualSlotRunWorkflowInput{
		SlotID:             slotID,
		UserID:             req.UserID,
		Kind:               req.Kind,
		Profile:            req.Profile,
		SourceIDs:          append([]string(nil), req.SourceIDs...),
		ExplicitRefresh:    req.ExplicitRefresh,
		BroadFilterKeyHash: req.BroadFilterKeyHash,
		PipelineRunID:      req.PipelineRunID,
	}
	if req.BroadRules != nil {
		br, err := req.BroadRules.ToPipeline()
		if err != nil {
			return ManualSlotRunWorkflowInput{}, err
		}
		in.BroadRules = br
	}
	if req.KeywordRules != nil {
		in.KeywordRules = req.KeywordRules.ToPipeline()
	}
	return in, nil
}

func parseRFC3339Ptr(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*s)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid RFC3339 time %q", raw)
}
