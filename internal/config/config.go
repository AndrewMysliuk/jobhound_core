package config

// Config holds runtime settings loaded from env and optional files (wired in cmd/ later).
// Secret values must live only in .env (not committed).
// PostgreSQL is the system of record; DSNs use JOBHOUND_DATABASE_URL / JOBHOUND_MIGRATE_DATABASE_URL
// (see specs/002-postgres-gorm-migrations/contracts/environment.md).
type Config struct {
	DatabaseURL         string
	MigrateDatabaseURL  string // optional migrate-only DSN override
	AnthropicAPIKey     string
	TelegramBotToken    string
	TelegramChatID      string
	LinkedInCookiesPath string
	HTTPUserAgent       string
	IncludeKeywords     []string
	ExcludeKeywords     []string
}
