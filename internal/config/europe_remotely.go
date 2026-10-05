package config

import (
	"os"
	"strings"
)

// EnvEuropeRemotelyProxy is an HTTP CONNECT proxy used only for euremotejobs.com.
// Empty dials directly. make docker-up sets http://host.docker.internal:18080 in Compose
// and listens on the host with cmd/ipv6proxy.
const EnvEuropeRemotelyProxy = "JOBHOUND_EUROPE_REMOTELY_PROXY"

// EuropeRemotelyConfig is the Europe Remotely collector's env-backed settings.
type EuropeRemotelyConfig struct {
	// ProxyURL is an HTTP proxy URL (http://host:port). Empty means a direct connection.
	ProxyURL string
}

// LoadEuropeRemotelyFromEnv reads JOBHOUND_EUROPE_REMOTELY_PROXY.
func LoadEuropeRemotelyFromEnv() EuropeRemotelyConfig {
	return EuropeRemotelyConfig{
		ProxyURL: strings.TrimSpace(os.Getenv(EnvEuropeRemotelyProxy)),
	}
}
