package builtin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractBuiltinApplyURLFromDetailHTML_howToApplyInScript(t *testing.T) {
	t.Parallel()
	page := "https://builtin.com/job/full-stack-engineer/8428200"
	html := `<!DOCTYPE html><html><body><script type="module">
Builtin.jobPostInit({"job":{"id":8428200,"howToApply":"https://jobs.lever.co/qonto/5e9f184e/apply?x=1\u0026y=2"}});
</script></body></html>`
	got := extractBuiltinApplyURLFromDetailHTML(html, page)
	require.Equal(t, "https://jobs.lever.co/qonto/5e9f184e/apply?x=1&y=2", got)
}

func TestExtractBuiltinApplyURLFromDetailHTML_relativeApplyButton(t *testing.T) {
	t.Parallel()
	page := "https://builtin.com/job/foo/1"
	html := `<html><body><a id="applyButton" href="/r?u=https%3A%2F%2Fats.example%2Fapply">Go</a></body></html>`
	got := extractBuiltinApplyURLFromDetailHTML(html, page)
	require.Equal(t, "https://builtin.com/r?u=https%3A%2F%2Fats.example%2Fapply", got)
}
