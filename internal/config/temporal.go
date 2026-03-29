package config

import (
	"fmt"
	"os"
)

// Temporal env keys (contract: specs/003-temporal-orchestration/contracts/environment.md).
const (
	EnvTemporalAddress   = "JOBHOUND_TEMPORAL_ADDRESS"
	EnvTemporalNamespace = "JOBHOUND_TEMPORAL_NAMESPACE"
	EnvTemporalTaskQueue = "JOBHOUND_TEMPORAL_TASK_QUEUE"
)

// Defaults aligned with specs/003 (namespace and task queue).
const (
	DefaultTemporalNamespace = "default"
	DefaultTemporalTaskQueue = "jobhound"
)

// Temporal holds dial and routing settings for workers and clients.
type Temporal struct {
	Address   string
	Namespace string
	TaskQueue string
}

// LoadTemporalFromEnv loads Temporal settings from the environment.
// Address is required; namespace and task queue fall back to defaults.
func LoadTemporalFromEnv() (Temporal, error) {
	addr := os.Getenv(EnvTemporalAddress)
	if addr == "" {
		return Temporal{}, fmt.Errorf("%s is required", EnvTemporalAddress)
	}
	ns := os.Getenv(EnvTemporalNamespace)
	if ns == "" {
		ns = DefaultTemporalNamespace
	}
	q := os.Getenv(EnvTemporalTaskQueue)
	if q == "" {
		q = DefaultTemporalTaskQueue
	}
	return Temporal{
		Address:   addr,
		Namespace: ns,
		TaskQueue: q,
	}, nil
}
