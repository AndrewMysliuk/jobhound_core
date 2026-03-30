package config

// Config is the single place for env-backed application and infrastructure settings.
// Use Load() in cmd/* and pass nested structs (e.g. Database) into internal packages;
// do not scatter os.Getenv across feature modules — add fields and parsing here instead.
type Config struct {
	Database Database
	// DataDir is the directory containing countries.json (see EnvDataDir). Empty means use "data" relative to the process working directory.
	DataDir string
	// DebugHTTPAddr enables cmd/agent local debug HTTP when non-empty (see EnvDebugHTTPAddr); flag -debug-http-addr overrides.
	DebugHTTPAddr string

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
		DataDir:         loadDataDirFromEnv(),
		DebugHTTPAddr:   loadDebugHTTPAddrFromEnv(),
		AnthropicAPIKey: LoadAnthropicAPIKeyFromEnv(),
		AnthropicModel:  model,
	}
}
