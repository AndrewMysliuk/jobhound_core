package pipeline

import "errors"

// ErrManualPatchNotInScope is returned when a manual bucket PATCH targets a missing or wrong-status row (009).
var ErrManualPatchNotInScope = errors.New("pipeline run job not in scope for manual bucket patch")
