package config

import "os"

// Anthropic (Claude) API — used when wiring real LLM scoring; optional until stage 3 is implemented.
const (
	EnvAnthropicAPIKey    = "JOBHOUND_ANTHROPIC_API_KEY"
	EnvAnthropicModel     = "JOBHOUND_ANTHROPIC_MODEL"
	DefaultAnthropicModel = "claude-3-5-haiku-20241022"
)

// LoadAnthropicAPIKeyFromEnv returns the API key from the environment (may be empty).
func LoadAnthropicAPIKeyFromEnv() string {
	return os.Getenv(EnvAnthropicAPIKey)
}

// LoadAnthropicModelFromEnv returns the Claude model id; empty means caller should use DefaultAnthropicModel.
func LoadAnthropicModelFromEnv() string {
	return os.Getenv(EnvAnthropicModel)
}
