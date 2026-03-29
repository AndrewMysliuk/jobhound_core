package db

import (
	"context"
	"strings"
	"testing"
)

func TestOpenFromEnv_missingURL(t *testing.T) {
	t.Setenv(envDatabaseURL, "")
	_, err := OpenFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error when JOBHOUND_DATABASE_URL is unset")
	}
	if !strings.Contains(err.Error(), envDatabaseURL) {
		t.Fatalf("error should mention %q: %v", envDatabaseURL, err)
	}
}
