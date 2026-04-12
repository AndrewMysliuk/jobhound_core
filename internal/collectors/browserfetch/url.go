package browserfetch

import (
	"fmt"
	"net/url"
	"strings"
)

func requireAbsoluteHTTPS(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("browserfetch: empty URL")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("browserfetch: parse URL: %w", err)
	}
	if !u.IsAbs() || u.Host == "" {
		return "", fmt.Errorf("browserfetch: URL must be absolute with host")
	}
	if strings.ToLower(u.Scheme) != "https" {
		return "", fmt.Errorf("browserfetch: only https URLs are supported (got scheme %q)", u.Scheme)
	}
	return u.String(), nil
}
