package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

const (
	// EnvAnthropicAPIKey is the Anthropic Message Batches key. Empty disables enrich.
	EnvAnthropicAPIKey = "JOBHOUND_ANTHROPIC_API_KEY"
	// EnvEnrichModel is the model id stored on a verdict row. Required only when the key is set.
	EnvEnrichModel = "JOBHOUND_ENRICH_MODEL"
	// EnvEnrichMinDescriptionChars skips vacancies with shorter descriptions. Empty or 0 uses schema.MinDescriptionChars.
	EnvEnrichMinDescriptionChars = "JOBHOUND_ENRICH_MIN_DESCRIPTION_CHARS"
	// EnvEnrichWindowDays is the first_seen_at window for enrich candidates. Empty or 0 uses schema.WindowDays.
	EnvEnrichWindowDays = "JOBHOUND_ENRICH_WINDOW_DAYS"
)

// Enrich is the Anthropic batch client. Enabled is false when the key is empty; Model is then ignored.
type Enrich struct {
	Enabled             bool
	APIKey              string
	Model               string
	MinDescriptionChars int
	WindowDays          int
}

func loadEnrichFromEnv() (Enrich, error) {
	minChars, err := loadEnrichMinDescriptionCharsFromEnv()
	if err != nil {
		return Enrich{}, err
	}
	windowDays, err := loadEnrichWindowDaysFromEnv()
	if err != nil {
		return Enrich{}, err
	}
	key := strings.TrimSpace(os.Getenv(EnvAnthropicAPIKey))
	if key == "" {
		return Enrich{MinDescriptionChars: minChars, WindowDays: windowDays}, nil
	}
	model := strings.TrimSpace(os.Getenv(EnvEnrichModel))
	if model == "" {
		return Enrich{}, fmt.Errorf("%s is required when %s is set", EnvEnrichModel, EnvAnthropicAPIKey)
	}
	return Enrich{
		Enabled:             true,
		APIKey:              key,
		Model:               model,
		MinDescriptionChars: minChars,
		WindowDays:          windowDays,
	}, nil
}

func loadEnrichMinDescriptionCharsFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv(EnvEnrichMinDescriptionChars))
	if raw == "" {
		return enrichschema.MinDescriptionChars, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative integer", EnvEnrichMinDescriptionChars)
	}
	if n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", EnvEnrichMinDescriptionChars)
	}
	if n == 0 {
		return enrichschema.MinDescriptionChars, nil
	}
	return n, nil
}

func loadEnrichWindowDaysFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv(EnvEnrichWindowDays))
	if raw == "" {
		return enrichschema.WindowDays, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative integer", EnvEnrichWindowDays)
	}
	if n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", EnvEnrichWindowDays)
	}
	if n == 0 {
		return enrichschema.WindowDays, nil
	}
	return n, nil
}
