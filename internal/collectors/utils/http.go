package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent is sent on collector HTTP requests (see specs/005-job-collectors/contracts/collector.md).
const DefaultUserAgent = "JobHound/1.0 (+collector; https://github.com/andrewmysliuk/jobhound_core)"

// BrowserUserAgent is the shared browser User-Agent for architecture-profile collectors.
const BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

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

// HTTPClientWithProxy returns a collector client that sends HTTPS via an HTTP CONNECT proxy.
// The proxy URL must be http://host:port. The shared base transport is not modified.
func HTTPClientWithProxy(base *http.Client, proxyRaw string) (*http.Client, error) {
	u, err := url.Parse(strings.TrimSpace(proxyRaw))
	if err != nil {
		return nil, fmt.Errorf("proxy url: %w", err)
	}
	if u.Scheme != "http" || u.Host == "" {
		return nil, fmt.Errorf("proxy url must be http://host:port")
	}
	timeout := DefaultHTTPTimeout
	if base != nil && base.Timeout > 0 {
		timeout = base.Timeout
	}
	tr := collectorTransport()
	tr.Proxy = http.ProxyURL(u)
	return &http.Client{Timeout: timeout, Transport: tr}, nil
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
