package config

import (
	"os"
	"strconv"
	"strings"
)

// Cache and ingest coordination (specs/006-cache-and-ingest/contracts/environment.md).
const (
	EnvRedisURL                 = "JOBHOUND_REDIS_URL"
	EnvIngestExplicitRefresh    = "JOBHOUND_INGEST_EXPLICIT_REFRESH"
	EnvIngestLockTTLSec         = "JOBHOUND_INGEST_LOCK_TTL_SEC"
	EnvIngestCooldownTTLSec     = "JOBHOUND_INGEST_COOLDOWN_TTL_SEC"
	DefaultIngestLockTTLSec     = 600
	DefaultIngestCooldownTTLSec = 3600
)

// Ingest holds Redis URL and explicit-refresh for per-source ingest coordination.
type Ingest struct {
	// RedisURL is the Redis connection URL (e.g. redis://localhost:6379/0). Empty if unset.
	RedisURL string
	// ExplicitRefresh allows bypassing ingest cooldown when true; lock is still required.
	ExplicitRefresh bool
	// LockTTLSeconds is Redis lock TTL (seconds); always positive after load.
	LockTTLSeconds int
	// CooldownTTLSeconds is cooldown key TTL after successful ingest (seconds); always positive after load.
	CooldownTTLSeconds int
}

// LoadIngestFromEnv reads JOBHOUND_REDIS_URL, JOBHOUND_INGEST_EXPLICIT_REFRESH, and optional TTL overrides.
// Explicit refresh defaults to false when unset or when the value is not a valid strconv.ParseBool string.
// Invalid or non-positive TTL env values fall back to defaults (600 / 3600).
func LoadIngestFromEnv() Ingest {
	s := strings.TrimSpace(os.Getenv(EnvIngestExplicitRefresh))
	explicit := false
	if s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			explicit = b
		}
	}
	return Ingest{
		RedisURL:           strings.TrimSpace(os.Getenv(EnvRedisURL)),
		ExplicitRefresh:    explicit,
		LockTTLSeconds:     ingestTTLSecondsFromEnv(EnvIngestLockTTLSec, DefaultIngestLockTTLSec),
		CooldownTTLSeconds: ingestTTLSecondsFromEnv(EnvIngestCooldownTTLSec, DefaultIngestCooldownTTLSec),
	}
}

func ingestTTLSecondsFromEnv(key string, defaultSec int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultSec
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultSec
	}
	return n
}
