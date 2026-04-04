package config

import (
	"os"
	"testing"
)

func TestLoadAPIFromEnv_listenDefault(t *testing.T) {
	t.Setenv(EnvAPIListen, "")
	t.Setenv(EnvAPICORSOrigins, "http://a.example, http://b.example ")
	cfg := LoadAPIFromEnv()
	if cfg.Listen != DefaultAPIListen {
		t.Fatalf("listen: got %q want %q", cfg.Listen, DefaultAPIListen)
	}
	want := []string{"http://a.example", "http://b.example"}
	if len(cfg.CORSAllowedOrigins) != len(want) {
		t.Fatalf("origins: got %#v want %#v", cfg.CORSAllowedOrigins, want)
	}
	for i := range want {
		if cfg.CORSAllowedOrigins[i] != want[i] {
			t.Fatalf("origins[%d]: got %q want %q", i, cfg.CORSAllowedOrigins[i], want[i])
		}
	}
}

func TestLoadAPIFromEnv_corsUnsetUsesDefault(t *testing.T) {
	if _, exists := os.LookupEnv(EnvAPICORSOrigins); exists {
		t.Skip("environment has " + EnvAPICORSOrigins + " set; unset default path not testable in-process")
	}
	t.Setenv(EnvAPIListen, "0.0.0.0:9")
	cfg := LoadAPIFromEnv()
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != DefaultAPICORSOrigins {
		t.Fatalf("origins: got %#v want [%q]", cfg.CORSAllowedOrigins, DefaultAPICORSOrigins)
	}
}

func TestLoadAPIFromEnv_corsEmptyStringDisables(t *testing.T) {
	t.Setenv(EnvAPICORSOrigins, "")
	cfg := LoadAPIFromEnv()
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("origins: got %#v want empty", cfg.CORSAllowedOrigins)
	}
}
