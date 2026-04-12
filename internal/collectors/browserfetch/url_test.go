package browserfetch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequireAbsoluteHTTPS(t *testing.T) {
	t.Parallel()
	_, err := requireAbsoluteHTTPS("")
	require.Error(t, err)

	_, err = requireAbsoluteHTTPS("http://example.com/")
	require.Error(t, err)

	_, err = requireAbsoluteHTTPS("/jobs/remote")
	require.Error(t, err)

	u, err := requireAbsoluteHTTPS("https://builtin.com/jobs/remote?search=go")
	require.NoError(t, err)
	require.Contains(t, u, "https://builtin.com/")
}
