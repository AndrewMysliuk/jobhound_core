package config

import "os"

// Anthropic (Claude) API — used when wiring real LLM scoring; optional until stage 3 is implemented.
const (
	EnvAnthropicAPIKey    = "JOBHOUND_ANTHROPIC_API_KEY"
	EnvAnthropicModel     = "JOBHOUND_ANTHROPIC_MODEL"
	// Models that support Messages API structured outputs (output_config.format json_schema); see Anthropic structured outputs docs.
	//
	// Toggle default for local A/B: comment one line, uncomment the other (or set JOBHOUND_ANTHROPIC_MODEL).
	DefaultAnthropicModel = "claude-sonnet-4-5"
	// DefaultAnthropicModel = "claude-haiku-4-5"
)

// LoadAnthropicAPIKeyFromEnv returns the API key from the environment (may be empty).
func LoadAnthropicAPIKeyFromEnv() string {
	return os.Getenv(EnvAnthropicAPIKey)
}

// LoadAnthropicModelFromEnv returns the Claude model id; empty means caller should use DefaultAnthropicModel.
func LoadAnthropicModelFromEnv() string {
	return os.Getenv(EnvAnthropicModel)
}
