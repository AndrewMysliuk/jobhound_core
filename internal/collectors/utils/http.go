package utils

import (
	"net"
	"net/http"
	"time"
)

// DefaultUserAgent is sent on collector HTTP requests (see specs/005-job-collectors/contracts/collector.md).
const DefaultUserAgent = "JobHound/1.0 (+collector; https://github.com/andrewmysliuk/jobhound_core)"

// SetCollectorUserAgent sets the standard collector User-Agent on req.
func SetCollectorUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", DefaultUserAgent)
}

// SetCollectFormPostHeaders sets Content-Type and User-Agent for application/x-www-form-urlencoded POSTs.
func SetCollectFormPostHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	SetCollectorUserAgent(req)
}

// SetCollectJSONPostHeaders sets Content-Type and User-Agent for JSON POST bodies.
func SetCollectJSONPostHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	SetCollectorUserAgent(req)
}

// DefaultHTTPTimeout is the client-level timeout for one collector HTTP round-trip (no retries).
var DefaultHTTPTimeout = 30 * time.Second

// NewHTTPClient returns an *http.Client with DefaultHTTPTimeout and a shared transport tuned for collectors.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   DefaultHTTPTimeout,
		Transport: collectorTransport(),
	}
}

func collectorTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 4
	t.Proxy = http.ProxyFromEnvironment
	t.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	return t
}
