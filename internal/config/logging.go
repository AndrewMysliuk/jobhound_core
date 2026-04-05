package config

import (
	"os"
	"strings"
)

// Env keys for structured logging (specs/010-observability/contracts/environment.md).
const (
	EnvLogLevel  = "JOBHOUND_LOG_LEVEL"
	EnvLogFormat = "JOBHOUND_LOG_FORMAT"
)

const (
	// DefaultLogLevel is used when JOBHOUND_LOG_LEVEL is unset (cmd/api, cmd/worker, cmd/agent, cmd/retention).
	DefaultLogLevel = "info"
	// DefaultLogFormat is human-readable console output for local dev when JOBHOUND_LOG_FORMAT is unset.
	DefaultLogFormat = "console"
)

// Logging holds parsed log level and output format (wired from env in one place).
type Logging struct {
	Level  string
	Format string
}

// LoadLoggingFromEnv reads JOBHOUND_LOG_LEVEL and JOBHOUND_LOG_FORMAT with defaults from this package.
func LoadLoggingFromEnv() Logging {
	level := strings.TrimSpace(strings.ToLower(os.Getenv(EnvLogLevel)))
	if level == "" {
		level = DefaultLogLevel
	}
	format := strings.TrimSpace(strings.ToLower(os.Getenv(EnvLogFormat)))
	if format == "" {
		format = DefaultLogFormat
	}
	return Logging{Level: level, Format: format}
}
