package europeremotely

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverNonce(t *testing.T) {
	const want = "abc123dead"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><script>
        var ERJ = {
            ajaxUrl : 'https://example.com/wp-admin/admin-ajax.php',
            nonce   : '` + want + `',
            page    : 1
        };
        </script>`))
	}))
	t.Cleanup(srv.Close)

	got, err := DiscoverNonceFromURL(context.Background(), srv.Client(), srv.URL)
	require.NoError(t, err)
	require.Equal(t, want, got)
}
