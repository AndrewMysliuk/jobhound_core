package config

import "testing"

func TestLoadProfilesDirAndRetentionDays(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv(EnvProfilesDir, "")
		t.Setenv(EnvJobRetentionDays, "  ")
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
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
