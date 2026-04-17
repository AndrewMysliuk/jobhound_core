package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Shared Tier-3 browser / Rod (specs/005-job-collectors/contracts/environment.md).
const (
	EnvBrowserEnabled        = "JOBHOUND_BROWSER_ENABLED"
	EnvBrowserBin            = "JOBHOUND_BROWSER_BIN"
	EnvBrowserUserDataDir    = "JOBHOUND_BROWSER_USER_DATA_DIR"
	EnvBrowserNavTimeoutMS   = "JOBHOUND_BROWSER_NAV_TIMEOUT_MS"
	EnvBrowserNoSandbox      = "JOBHOUND_BROWSER_NO_SANDBOX"
	defaultBrowserNavTimeout = 2 * time.Minute
)

// BrowserConfig controls headless Chromium for internal/collectors/browserfetch (Enabled defaults true).
type BrowserConfig struct {
	Enabled bool
	Bin     string
	// UserDataDir is optional persistent Chromium profile storage.
	UserDataDir string
	NavTimeout  time.Duration
	// NoSandbox enables Chromium --no-sandbox (typical in Docker as root). Unsafe on shared hostile workloads.
	NoSandbox bool
}

// LoadBrowserFromEnv reads JOBHOUND_BROWSER_* variables.
// Tier-3 (Rod) is on by default so Built In can use browserfetch without extra env;
// set JOBHOUND_BROWSER_ENABLED=0 (or false/no/off) to skip launching Chromium when it is unavailable.
func LoadBrowserFromEnv() BrowserConfig {
	cfg := BrowserConfig{
		NavTimeout: defaultBrowserNavTimeout,
		Enabled:    true,
	}
	if v := strings.TrimSpace(os.Getenv(EnvBrowserEnabled)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.Enabled = true
		case "0", "false", "no", "off":
			cfg.Enabled = false
		default:
			cfg.Enabled = false
		}
	}
	cfg.Bin = strings.TrimSpace(os.Getenv(EnvBrowserBin))
	cfg.UserDataDir = strings.TrimSpace(os.Getenv(EnvBrowserUserDataDir))
	if v := strings.TrimSpace(os.Getenv(EnvBrowserNavTimeoutMS)); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			cfg.NavTimeout = time.Duration(ms) * time.Millisecond
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvBrowserNoSandbox)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.NoSandbox = true
		}
	}
	return cfg
}
