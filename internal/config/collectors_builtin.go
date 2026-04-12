package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Built In collector env (specs/005-job-collectors/contracts/environment.md).
const (
	EnvCollectorBuiltinInterRequestDelayMS   = "JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS"
	EnvCollectorBuiltinUseBrowser            = "JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER"
	defaultCollectorBuiltinInterRequestDelay = 500 * time.Millisecond
)

// BuiltinCollectorConfig holds defaults for the Built In collector (internal/collectors/builtin).
type BuiltinCollectorConfig struct {
	InterRequestDelay time.Duration
	// UseBrowserForHTML: when true (default), Built In uses browserfetch (rod) for listing/detail HTML when
	// bootstrap constructs a fetcher (JOBHOUND_BROWSER_ENABLED). Set JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER=0 to force net/http.
	UseBrowserForHTML bool
}

// LoadBuiltinCollectorFromEnv reads optional JOBHOUND_COLLECTOR_BUILTIN_* variables.
func LoadBuiltinCollectorFromEnv() BuiltinCollectorConfig {
	cfg := BuiltinCollectorConfig{
		InterRequestDelay: defaultCollectorBuiltinInterRequestDelay,
		UseBrowserForHTML: true,
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorBuiltinInterRequestDelayMS)); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			cfg.InterRequestDelay = time.Duration(ms) * time.Millisecond
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorBuiltinUseBrowser)); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "no", "off":
			cfg.UseBrowserForHTML = false
		}
	}
	return cfg
}
