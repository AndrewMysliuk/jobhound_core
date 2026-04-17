package builtin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLooksLikeCloudflareChallengeHTML(t *testing.T) {
	t.Parallel()
	cfTitleAndScript := `<html lang="en-US"><head><title>Just a moment...</title></head><body>` +
		`<script src="/cdn-cgi/challenge-platform/h/g/orchestrate/chl_page/v1?ray=x"></script></body></html>`
	cases := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "cloudflare_challenge_user_log_shape",
			html: cfTitleAndScript,
			want: true,
		},
		{
			name: "challenge_platform_path_only",
			html: `<html><body><script src="https://builtin.com/cdn-cgi/challenge-platform/foo"></script></body></html>`,
			want: true,
		},
		{
			name: "cf_browser_verification_meta",
			html: `<html><head><meta name="cf-browser-verification" content="x"></head><body></body></html>`,
			want: true,
		},
		{
			name: "normal_job_detail_snippet",
			html: `<html><script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"JobPosting","title":"Hi"}]}</script></html>`,
			want: false,
		},
		{
			name: "empty",
			html: "",
			want: false,
		},
		{
			name: "just_moment_without_cdn_cgi",
			html: `<html><head><title>Just a moment...</title></head><body>please wait</body></html>`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := looksLikeCloudflareChallengeHTML([]byte(tc.html))
			require.Equal(t, tc.want, got)
		})
	}
}
