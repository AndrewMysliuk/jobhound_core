package config

// Epic 008 (manual-search-workflow) adds no new JOBHOUND_* keys — see specs/008-manual-search-workflow/contracts/environment.md.
// Epic 010 (observability): JOBHOUND_LOG_LEVEL, JOBHOUND_LOG_FORMAT — see specs/010-observability/contracts/environment.md.

// Config is the single place for env-backed application and infrastructure settings.
// Use Load() in cmd/* and pass nested structs (e.g. Database) into internal packages;
// do not scatter os.Getenv across feature modules — add fields and parsing here instead.
type Config struct {
	Database Database
	API      API
	Ingest   Ingest
	Pipeline Pipeline
	Logging  Logging
	// DataDir is the directory containing countries.json (see EnvDataDir). Empty means use "data" relative to the process working directory.
	DataDir string
	// DebugHTTPAddr enables cmd/agent local debug HTTP when non-empty (see EnvDebugHTTPAddr); flag -debug-http-addr overrides.
	DebugHTTPAddr string
	// DouCollector configures the DOU.ua collector (005-job-collectors).
	DouCollector DouCollectorConfig
	// HimalayasCollector configures the Himalayas JSON collector (005-job-collectors).
	HimalayasCollector HimalayasCollectorConfig
	// DjinniCollector configures the Djinni HTML + JSON-LD collector (005-job-collectors).
	DjinniCollector DjinniCollectorConfig
	// BuiltinCollector configures the Built In remote listing collector (005-job-collectors).
	BuiltinCollector BuiltinCollectorConfig
	// Browser configures Tier-3 headless document fetch (005-job-collectors / browserfetch); Enabled defaults true (JOBHOUND_BROWSER_ENABLED=0 to opt out).
	Browser BrowserConfig

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
		Database:           LoadDatabaseFromEnv(),
		API:                LoadAPIFromEnv(),
		Ingest:             LoadIngestFromEnv(),
		Pipeline:           LoadPipelineFromEnv(),
		Logging:            LoadLoggingFromEnv(),
		DataDir:            loadDataDirFromEnv(),
		DebugHTTPAddr:      loadDebugHTTPAddrFromEnv(),
		DouCollector:       LoadDouCollectorFromEnv(),
		HimalayasCollector: LoadHimalayasCollectorFromEnv(),
		DjinniCollector:    LoadDjinniCollectorFromEnv(),
		BuiltinCollector:   LoadBuiltinCollectorFromEnv(),
		Browser:            LoadBrowserFromEnv(),
		AnthropicAPIKey:    LoadAnthropicAPIKeyFromEnv(),
		AnthropicModel:     model,
	}
}
