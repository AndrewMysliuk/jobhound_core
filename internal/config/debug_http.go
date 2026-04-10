package config

import "os"

// EnvDebugHTTPAddr, when non-empty, enables the optional local debug HTTP API in cmd/agent
// (GET /health, POST /debug/collectors/* including europe_remotely, working_nomads, dou_ua, himalayas, djinni).
// Prefer binding to loopback in production-adjacent setups.
const EnvDebugHTTPAddr = "JOBHOUND_DEBUG_HTTP_ADDR"

func loadDebugHTTPAddrFromEnv() string {
	return os.Getenv(EnvDebugHTTPAddr)
}
