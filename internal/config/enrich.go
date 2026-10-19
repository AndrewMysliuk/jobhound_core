package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	// EnvAnthropicAPIKey is the Anthropic Message Batches key. Empty disables enrich.
	EnvAnthropicAPIKey = "JOBHOUND_ANTHROPIC_API_KEY"
	// EnvEnrichModel is the model id stored on a verdict row. Required only when the key is set.
	EnvEnrichModel = "JOBHOUND_ENRICH_MODEL"
)

// Enrich is the Anthropic batch client. Enabled is false when the key is empty; Model is then ignored.
type Enrich struct {
	Enabled bool
	APIKey  string
	Model   string
}

func loadEnrichFromEnv() (Enrich, error) {
	key := strings.TrimSpace(os.Getenv(EnvAnthropicAPIKey))
	if key == "" {
		return Enrich{}, nil
	}
	model := strings.TrimSpace(os.Getenv(EnvEnrichModel))
	if model == "" {
		return Enrich{}, fmt.Errorf("%s is required when %s is set", EnvEnrichModel, EnvAnthropicAPIKey)
	}
	return Enrich{
		Enabled: true,
		APIKey:  key,
		Model:   model,
	}, nil
}
