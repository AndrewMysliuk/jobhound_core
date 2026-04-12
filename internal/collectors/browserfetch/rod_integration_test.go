//go:build integration

package browserfetch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Requires local Chromium/Chrome; set JOBHOUND_BROWSER_INTEGRATION=1 to run.
func TestRodFetcher_FetchHTMLDocument_smoke(t *testing.T) {
	if os.Getenv("JOBHOUND_BROWSER_INTEGRATION") == "" {
		t.Skip("set JOBHOUND_BROWSER_INTEGRATION=1 to run Rod smoke against the network")
	}
	f, err := NewRodFetcher(RodOptions{NavTimeout: 2 * time.Minute})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	b, err := f.FetchHTMLDocument(ctx, "https://example.com/")
	require.NoError(t, err)
	require.Contains(t, string(b), "Example Domain")
}
