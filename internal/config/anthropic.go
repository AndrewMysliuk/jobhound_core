package config

import "os"

// Anthropic (Claude) API — used when wiring real LLM scoring; optional until stage 3 is implemented.
const EnvAnthropicAPIKey = "JOBHOUND_ANTHROPIC_API_KEY"

// LoadAnthropicAPIKeyFromEnv returns the API key from the environment (may be empty).
func LoadAnthropicAPIKeyFromEnv() string {
	return os.Getenv(EnvAnthropicAPIKey)
}
