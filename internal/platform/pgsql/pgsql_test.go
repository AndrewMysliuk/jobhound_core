package pgsql

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
)

func TestOpenFromEnv_missingURL(t *testing.T) {
	t.Setenv(config.EnvDatabaseURL, "")
	_, err := OpenFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error when database URL env is unset")
	}
	if !strings.Contains(err.Error(), config.EnvDatabaseURL) {
		t.Fatalf("error should mention %q: %v", config.EnvDatabaseURL, err)
	}
}
