package config

import (
	"testing"
)

func TestLoadIngestFromEnv(t *testing.T) {
	t.Setenv(EnvRedisURL, "")
	t.Setenv(EnvIngestExplicitRefresh, "")
	t.Setenv(EnvIngestLockTTLSec, "")
	t.Setenv(EnvIngestCooldownTTLSec, "")
	cfg := LoadIngestFromEnv()
	if cfg.RedisURL != "" || cfg.ExplicitRefresh {
		t.Fatalf("empty env: got %#v", cfg)
	}
	if cfg.LockTTLSeconds != DefaultIngestLockTTLSec || cfg.CooldownTTLSeconds != DefaultIngestCooldownTTLSec {
		t.Fatalf("default TTLs: got lock=%d cooldown=%d", cfg.LockTTLSeconds, cfg.CooldownTTLSeconds)
	}

	t.Setenv(EnvRedisURL, "  redis://localhost:6379/0  ")
	t.Setenv(EnvIngestExplicitRefresh, "true")
	t.Setenv(EnvIngestLockTTLSec, "120")
	t.Setenv(EnvIngestCooldownTTLSec, "1800")
	cfg = LoadIngestFromEnv()
	if cfg.RedisURL != "redis://localhost:6379/0" || !cfg.ExplicitRefresh {
		t.Fatalf("trim + true: got %#v", cfg)
	}
	if cfg.LockTTLSeconds != 120 || cfg.CooldownTTLSeconds != 1800 {
		t.Fatalf("TTL overrides: got %#v", cfg)
	}

	t.Setenv(EnvIngestExplicitRefresh, "bogus")
	t.Setenv(EnvIngestLockTTLSec, "0")
	t.Setenv(EnvIngestCooldownTTLSec, "-5")
	cfg = LoadIngestFromEnv()
	if cfg.ExplicitRefresh {
		t.Fatalf("invalid bool should stay false: got %#v", cfg)
	}
	if cfg.LockTTLSeconds != DefaultIngestLockTTLSec || cfg.CooldownTTLSeconds != DefaultIngestCooldownTTLSec {
		t.Fatalf("invalid TTL env should fall back to defaults: got %#v", cfg)
	}
}
