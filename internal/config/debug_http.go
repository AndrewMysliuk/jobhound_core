package config

import "os"

// EnvDebugHTTPAddr, when non-empty, enables the optional local debug HTTP API in cmd/agent
// (GET /health, POST /debug/collectors/europe_remotely, POST /debug/collectors/working_nomads).
// Prefer binding to loopback in production-adjacent setups.
const EnvDebugHTTPAddr = "JOBHOUND_DEBUG_HTTP_ADDR"

func loadDebugHTTPAddrFromEnv() string {
	return os.Getenv(EnvDebugHTTPAddr)
}
