package config

import (
	"testing"
)

func TestLoadLoggingFromEnv_defaults(t *testing.T) {
	t.Setenv(EnvLogLevel, "")
	t.Setenv(EnvLogFormat, "")
	got := LoadLoggingFromEnv()
	if got.Level != DefaultLogLevel {
		t.Fatalf("level: got %q want %q", got.Level, DefaultLogLevel)
	}
	if got.Format != DefaultLogFormat {
		t.Fatalf("format: got %q want %q", got.Format, DefaultLogFormat)
	}
}

func TestLoadLoggingFromEnv_explicit(t *testing.T) {
	t.Setenv(EnvLogLevel, "  DEBUG ")
	t.Setenv(EnvLogFormat, " JSON ")
	got := LoadLoggingFromEnv()
	if got.Level != "debug" {
		t.Fatalf("level: got %q", got.Level)
	}
	if got.Format != "json" {
		t.Fatalf("format: got %q", got.Format)
	}
}
