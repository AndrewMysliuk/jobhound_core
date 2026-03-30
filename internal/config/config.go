package config

// Config is the single place for env-backed application and infrastructure settings.
// Use Load() in cmd/* and pass nested structs (e.g. Database) into internal packages;
// do not scatter os.Getenv across feature modules — add fields and parsing here instead.
type Config struct {
	Database Database

	AnthropicAPIKey     string
	AnthropicModel      string
	TelegramBotToken    string
	TelegramChatID      string
	LinkedInCookiesPath string
	HTTPUserAgent       string
	IncludeKeywords     []string
	ExcludeKeywords     []string
}

// Load reads supported environment variables into Config.
// For Temporal (worker / client), call LoadTemporalFromEnv separately — it enforces a required address.
func Load() Config {
	model := LoadAnthropicModelFromEnv()
	if model == "" {
		model = DefaultAnthropicModel
	}
	return Config{
		Database:        LoadDatabaseFromEnv(),
		AnthropicAPIKey: LoadAnthropicAPIKeyFromEnv(),
		AnthropicModel:  model,
	}
}
