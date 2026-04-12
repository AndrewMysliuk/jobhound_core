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
	defaultCollectorBuiltinInterRequestDelay = 300 * time.Millisecond
)

// BuiltinCollectorConfig holds defaults for the Built In collector (internal/collectors/builtin).
type BuiltinCollectorConfig struct {
	InterRequestDelay time.Duration
}

// LoadBuiltinCollectorFromEnv reads optional JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS.
func LoadBuiltinCollectorFromEnv() BuiltinCollectorConfig {
	cfg := BuiltinCollectorConfig{
		InterRequestDelay: defaultCollectorBuiltinInterRequestDelay,
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorBuiltinInterRequestDelayMS)); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			cfg.InterRequestDelay = time.Duration(ms) * time.Millisecond
		}
	}
	return cfg
}
