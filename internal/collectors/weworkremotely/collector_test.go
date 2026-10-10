package weworkremotely

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobsFromRSS_companyWebsite(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<rss><channel>
<item>
  <title>Acme: Go Engineer</title>
  <link>https://weworkremotely.com/remote-jobs/acme-go-engineer</link>
  <description><![CDATA[<p><strong>URL:</strong> <a href="https://acme.example">site</a></p><p><a href="https://other.example/blog">blog</a></p>]]></description>
</item>
<item>
  <title>Other: Designer</title>
  <link>https://weworkremotely.com/remote-jobs/other-designer</link>
  <description><![CDATA[<p><a href="https://other.example">not the labeled field</a></p>]]></description>
</item>
</channel></rss>`)

	jobs, err := jobsFromRSS(body, nil)
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	require.Equal(t, "https://weworkremotely.com/remote-jobs/acme-go-engineer", jobs[0].URL)
	require.Empty(t, jobs[0].ApplyURL)
	require.Equal(t, "https://acme.example", jobs[0].CompanyWebsite)

	require.Empty(t, jobs[1].ApplyURL)
	require.Empty(t, jobs[1].CompanyWebsite)
}
