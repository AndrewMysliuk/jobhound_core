package config

import (
	"os"
	"strings"
)

// Env keys for the browser-facing HTTP API (specs/009-http-public-api/contracts/environment.md).
const (
	EnvAPIListen      = "JOBHOUND_API_LISTEN"
	EnvAPICORSOrigins = "JOBHOUND_API_CORS_ORIGINS"
)

const (
	// DefaultAPIListen is the default TCP address for cmd/api when JOBHOUND_API_LISTEN is unset.
	DefaultAPIListen = "127.0.0.1:3000"
	// DefaultAPICORSOrigins is used when JOBHOUND_API_CORS_ORIGINS is unset (local dev).
	// Values are browser page origins (dev servers), not JOBHOUND_API_LISTEN.
	DefaultAPICORSOrigins = "http://localhost:5173,http://localhost:3002"
)

// API holds listen address and CORS allowlist for cmd/api.
// CORSAllowedOrigins empty means no Access-Control-Allow-Origin is sent (browser requests without same-origin will not see CORS headers).
type API struct {
	Listen             string
	CORSAllowedOrigins []string
}

// LoadAPIFromEnv loads API settings. Listen defaults to DefaultAPIListen.
// CORS: if JOBHOUND_API_CORS_ORIGINS is unset, defaults to DefaultAPICORSOrigins;
// if set to empty string, allowed origins are empty (no CORS allowance).
func LoadAPIFromEnv() API {
	listen := strings.TrimSpace(os.Getenv(EnvAPIListen))
	if listen == "" {
		listen = DefaultAPIListen
	}
	var origins []string
	if _, ok := os.LookupEnv(EnvAPICORSOrigins); ok {
		origins = splitCommaTrim(os.Getenv(EnvAPICORSOrigins))
	} else {
		origins = splitCommaTrim(DefaultAPICORSOrigins)
	}
	return API{
		Listen:             listen,
		CORSAllowedOrigins: origins,
	}
}

func splitCommaTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
