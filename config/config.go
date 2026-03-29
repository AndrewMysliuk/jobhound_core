package config

// Config holds runtime settings loaded from env and optional files (wired in a later tier).
// Secret values must live only in .env (not committed).
type Config struct {
	AnthropicAPIKey     string
	TelegramBotToken    string
	TelegramChatID      string
	SQLitePath          string
	LinkedInCookiesPath string
	HTTPUserAgent       string
	IncludeKeywords     []string
	ExcludeKeywords     []string
}
