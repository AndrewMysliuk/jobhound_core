package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// DOU.ua collector env (specs/005-job-collectors/contracts/environment.md).
const (
	EnvCollectorDouSearch                = "JOBHOUND_COLLECTOR_DOU_SEARCH"
	EnvCollectorDouInterRequestDelayMS   = "JOBHOUND_COLLECTOR_DOU_INTER_REQUEST_DELAY_MS"
	EnvCollectorDouMaxJobsPerFetch       = "JOBHOUND_COLLECTOR_DOU_MAX_JOBS_PER_FETCH"
	defaultCollectorDouSearch            = "go"
	defaultCollectorDouInterRequestDelay = 400 * time.Millisecond
	defaultCollectorDouMaxJobsPerFetch   = 100
)

// DouCollectorConfig holds defaults for the DOU collector (internal/collectors/dou).
type DouCollectorConfig struct {
	Search            string
	InterRequestDelay time.Duration
	MaxJobsPerFetch   int
}

// LoadDouCollectorFromEnv reads optional JOBHOUND_COLLECTOR_DOU_* variables.
func LoadDouCollectorFromEnv() DouCollectorConfig {
	cfg := DouCollectorConfig{
		Search:            strings.TrimSpace(os.Getenv(EnvCollectorDouSearch)),
		InterRequestDelay: defaultCollectorDouInterRequestDelay,
		MaxJobsPerFetch:   defaultCollectorDouMaxJobsPerFetch,
	}
	if cfg.Search == "" {
		cfg.Search = defaultCollectorDouSearch
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorDouInterRequestDelayMS)); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			cfg.InterRequestDelay = time.Duration(ms) * time.Millisecond
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvCollectorDouMaxJobsPerFetch)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxJobsPerFetch = n
		}
	}
	return cfg
}
