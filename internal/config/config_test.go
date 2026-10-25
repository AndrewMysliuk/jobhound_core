package config

import (
	"testing"

	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

func TestLoadProfilesDirAndRetentionDays(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv(EnvProfilesDir, "")
		t.Setenv(EnvJobRetentionDays, "  ")
		t.Setenv(EnvAnthropicAPIKey, "")
		t.Setenv(EnvEnrichMinDescriptionChars, "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ProfilesDir != DefaultProfilesDir {
			t.Fatalf("ProfilesDir = %q, want %q", cfg.ProfilesDir, DefaultProfilesDir)
		}
		if cfg.JobRetentionDays != DefaultJobRetentionDays {
			t.Fatalf("JobRetentionDays = %d, want %d", cfg.JobRetentionDays, DefaultJobRetentionDays)
		}
	})

	t.Run("explicit", func(t *testing.T) {
		t.Setenv(EnvProfilesDir, "  /var/profiles  ")
		t.Setenv(EnvJobRetentionDays, "15")
		t.Setenv(EnvAnthropicAPIKey, "")
		t.Setenv(EnvEnrichMinDescriptionChars, "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ProfilesDir != "/var/profiles" {
			t.Fatalf("ProfilesDir = %q", cfg.ProfilesDir)
		}
		if cfg.JobRetentionDays != 15 {
			t.Fatalf("JobRetentionDays = %d", cfg.JobRetentionDays)
		}
	})

	for _, raw := range []string{"0", "-1", "nope"} {
		t.Run("reject "+raw, func(t *testing.T) {
			t.Setenv(EnvProfilesDir, "")
			t.Setenv(EnvJobRetentionDays, raw)
			t.Setenv(EnvAnthropicAPIKey, "")
			t.Setenv(EnvEnrichMinDescriptionChars, "")
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadEnrich(t *testing.T) {
	t.Run("empty key disables and ignores model", func(t *testing.T) {
		t.Setenv(EnvAnthropicAPIKey, "  ")
		t.Setenv(EnvEnrichModel, "claude-haiku")
		t.Setenv(EnvEnrichMinDescriptionChars, "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Enrich.Enabled || cfg.Enrich.APIKey != "" || cfg.Enrich.Model != "" {
			t.Fatalf("Enrich = %+v, want disabled", cfg.Enrich)
		}
		if cfg.Enrich.MinDescriptionChars != enrichschema.MinDescriptionChars {
			t.Fatalf("MinDescriptionChars = %d, want %d", cfg.Enrich.MinDescriptionChars, enrichschema.MinDescriptionChars)
		}
	})

	t.Run("key without model", func(t *testing.T) {
		t.Setenv(EnvAnthropicAPIKey, "sk-test")
		t.Setenv(EnvEnrichModel, "  ")
		t.Setenv(EnvEnrichMinDescriptionChars, "")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("key and model", func(t *testing.T) {
		t.Setenv(EnvAnthropicAPIKey, "  sk-test  ")
		t.Setenv(EnvEnrichModel, "  claude-haiku  ")
		t.Setenv(EnvEnrichMinDescriptionChars, "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.Enrich.Enabled || cfg.Enrich.APIKey != "sk-test" || cfg.Enrich.Model != "claude-haiku" {
			t.Fatalf("Enrich = %+v", cfg.Enrich)
		}
		if cfg.Enrich.MinDescriptionChars != enrichschema.MinDescriptionChars {
			t.Fatalf("MinDescriptionChars = %d, want %d", cfg.Enrich.MinDescriptionChars, enrichschema.MinDescriptionChars)
		}
	})

	t.Run("min description zero uses default", func(t *testing.T) {
		t.Setenv(EnvAnthropicAPIKey, "")
		t.Setenv(EnvEnrichMinDescriptionChars, "0")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Enrich.MinDescriptionChars != enrichschema.MinDescriptionChars {
			t.Fatalf("MinDescriptionChars = %d, want %d", cfg.Enrich.MinDescriptionChars, enrichschema.MinDescriptionChars)
		}
	})

	t.Run("min description override", func(t *testing.T) {
		t.Setenv(EnvAnthropicAPIKey, "")
		t.Setenv(EnvEnrichMinDescriptionChars, "  1200  ")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Enrich.MinDescriptionChars != 1200 {
			t.Fatalf("MinDescriptionChars = %d, want 1200", cfg.Enrich.MinDescriptionChars)
		}
	})

	for _, raw := range []string{"-1", "nope"} {
		t.Run("reject min description "+raw, func(t *testing.T) {
			t.Setenv(EnvAnthropicAPIKey, "")
			t.Setenv(EnvEnrichMinDescriptionChars, raw)
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
