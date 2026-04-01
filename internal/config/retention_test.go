package config

import (
	"testing"
)

func TestLoadJobRetentionScheduleUpsertFromEnv(t *testing.T) {
	t.Setenv(EnvJobRetentionScheduleUpsert, "")
	if !LoadJobRetentionScheduleUpsertFromEnv() {
		t.Fatal("empty env should default to true")
	}
	t.Setenv(EnvJobRetentionScheduleUpsert, "false")
	if LoadJobRetentionScheduleUpsertFromEnv() {
		t.Fatal("false should disable")
	}
	t.Setenv(EnvJobRetentionScheduleUpsert, "true")
	if !LoadJobRetentionScheduleUpsertFromEnv() {
		t.Fatal("true should enable")
	}
	t.Setenv(EnvJobRetentionScheduleUpsert, "garbage")
	if LoadJobRetentionScheduleUpsertFromEnv() {
		t.Fatal("invalid value should be treated as false")
	}
}
