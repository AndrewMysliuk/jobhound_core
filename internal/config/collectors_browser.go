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

// BrowserConfig controls optional headless Chromium for internal/collectors/browserfetch.
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
func LoadBrowserFromEnv() BrowserConfig {
	cfg := BrowserConfig{
		NavTimeout: defaultBrowserNavTimeout,
	}
	if v := strings.TrimSpace(os.Getenv(EnvBrowserEnabled)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.Enabled = true
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
