package utils

import (
	"encoding/json"
	"fmt"
)

// ParseScoringJSON extracts score and rationale from the LLM JSON contract (score + rationale required).
func ParseScoringJSON(data []byte) (score int, rationale string, err error) {
	var v struct {
		Score     *int    `json:"score"`
		Rationale *string `json:"rationale"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, "", err
	}
	if v.Score == nil || v.Rationale == nil {
		return 0, "", fmt.Errorf("llm utils: scoring JSON: missing required score or rationale")
	}
	return *v.Score, *v.Rationale, nil
}
