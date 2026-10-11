package temporalopts

import "go.temporal.io/sdk/worker"

// DefaultWorkerOptions returns baseline [worker.Options] for cmd/worker.
func DefaultWorkerOptions() worker.Options {
	return worker.Options{}
}
