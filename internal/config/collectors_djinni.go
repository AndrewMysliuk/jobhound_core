package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Djinni collector env (specs/005-job-collectors/contracts/environment.md).
const (
	EnvCollectorDjinniInterRequestDelayMS   = "JOBHOUND_COLLECTOR_DJINNI_INTER_REQUEST_DELAY_MS"
	EnvCollectorDjinniMaxJobsPerFetch       = "JOBHOUND_COLLECTOR_DJINNI_MAX_JOBS_PER_FETCH"
	defaultCollectorDjinniInterRequestDelay = 400 * time.Millisecond
	defaultCollectorDjinniMaxJobsPerFetch   = 100
)

// DjinniCollectorConfig holds defaults for the Djinni collector (internal/collectors/djinni).
type DjinniCollectorConfig struct {
	InterRequestDelay time.Duration
	MaxJobsPerFetch   int
}

// LoadDjinniCollectorFromEnv reads optional JOBHOUND_COLLECTOR_DJINNI_* variables.
func LoadDjinniCollectorFromEnv() DjinniCollectorConfig {
	cfg := DjinniCollectorConfig{
		InterRequestDelay: defaultCollectorDjinniInterRequestDelay,
		MaxJobsPerFetch:   defaultCollectorDjinniMaxJobsPerFetch,
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorDjinniInterRequestDelayMS)); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			cfg.InterRequestDelay = time.Duration(ms) * time.Millisecond
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorDjinniMaxJobsPerFetch)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxJobsPerFetch = n
		}
	}
	return cfg
}
