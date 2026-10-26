package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// EnvCompaniesDir is the directory holding blocklist.yaml and the seed file.
	EnvCompaniesDir = "JOBHOUND_COMPANIES_DIR"
	// EnvCompaniesPollMaxPerRun caps board requests per run (duration / politeness), not memory; memory is bounded by ATSConcurrency.
	EnvCompaniesPollMaxPerRun = "JOBHOUND_COMPANIES_POLL_MAX_PER_RUN"
	// EnvCompaniesPollIntervalHours is the per-company due interval in hours.
	EnvCompaniesPollIntervalHours = "JOBHOUND_COMPANIES_POLL_INTERVAL_HOURS"
	// EnvCompaniesATSConcurrency is the parallel poll limit per ATS.
	EnvCompaniesATSConcurrency = "JOBHOUND_COMPANIES_ATS_CONCURRENCY"
	// EnvCompaniesResolveRetryDays is the UNRESOLVED → next_resolve_at offset in days.
	EnvCompaniesResolveRetryDays = "JOBHOUND_COMPANIES_RESOLVE_RETRY_DAYS"

	// DefaultCompaniesDir is used when JOBHOUND_COMPANIES_DIR is unset or blank.
	DefaultCompaniesDir = "config/companies"
	// DefaultCompaniesPollMaxPerRun is used when JOBHOUND_COMPANIES_POLL_MAX_PER_RUN is unset or blank.
	// Cap on requests per run (duration / politeness to boards), not a memory guard; memory is bounded by ATSConcurrency.
	DefaultCompaniesPollMaxPerRun = 300
	// DefaultCompaniesPollIntervalHours is used when JOBHOUND_COMPANIES_POLL_INTERVAL_HOURS is unset or blank.
	DefaultCompaniesPollIntervalHours = 6
	// DefaultCompaniesATSConcurrency is used when JOBHOUND_COMPANIES_ATS_CONCURRENCY is unset or blank.
	DefaultCompaniesATSConcurrency = 5
	// DefaultCompaniesResolveRetryDays is used when JOBHOUND_COMPANIES_RESOLVE_RETRY_DAYS is unset or blank.
	DefaultCompaniesResolveRetryDays = 14
)

// Companies holds company registry / ATS poll settings.
type Companies struct {
	// Dir is the directory holding blocklist.yaml and the seed file (see EnvCompaniesDir).
	Dir string
	// PollMaxPerRun caps board requests per run (duration / politeness), not memory; memory is bounded by ATSConcurrency.
	PollMaxPerRun int
	// PollIntervalHours is the per-company due interval.
	PollIntervalHours int
	// ATSConcurrency is the parallel poll limit per ATS.
	ATSConcurrency int
	// ResolveRetryDays is the UNRESOLVED → next_resolve_at offset in days.
	ResolveRetryDays int
}

func loadCompaniesFromEnv() (Companies, error) {
	pollMax, err := positiveIntFromEnv(EnvCompaniesPollMaxPerRun, DefaultCompaniesPollMaxPerRun)
	if err != nil {
		return Companies{}, err
	}
	intervalHours, err := positiveIntFromEnv(EnvCompaniesPollIntervalHours, DefaultCompaniesPollIntervalHours)
	if err != nil {
		return Companies{}, err
	}
	concurrency, err := positiveIntFromEnv(EnvCompaniesATSConcurrency, DefaultCompaniesATSConcurrency)
	if err != nil {
		return Companies{}, err
	}
	retryDays, err := positiveIntFromEnv(EnvCompaniesResolveRetryDays, DefaultCompaniesResolveRetryDays)
	if err != nil {
		return Companies{}, err
	}
	return Companies{
		Dir:               loadCompaniesDirFromEnv(),
		PollMaxPerRun:     pollMax,
		PollIntervalHours: intervalHours,
		ATSConcurrency:    concurrency,
		ResolveRetryDays:  retryDays,
	}, nil
}

func loadCompaniesDirFromEnv() string {
	dir := strings.TrimSpace(os.Getenv(EnvCompaniesDir))
	if dir == "" {
		return DefaultCompaniesDir
	}
	return dir
}

func positiveIntFromEnv(key string, defaultVal int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}
