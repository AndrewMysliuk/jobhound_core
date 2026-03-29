package domain

import "time"

// Job is the normalized vacancy flowing through the pipeline.
// ID is set via StableJobID / AssignStableID (see specs/001-agent-skeleton-and-domain/spec.md).
type Job struct {
	ID          string
	Source      string // e.g. "himalayas", "djinni"
	Title       string
	Company     string
	URL         string // canonical job posting (listing) page; used for stable id before ApplyURL fallback
	ApplyURL    string // optional external apply/ATS link; empty if unknown or same as listing
	Description string
	PostedAt    time.Time // zero if unknown
	UserID      *string   // optional; nil/empty = unset (future multi-user scope)
}

// ScoredJob is the post–stage-3 shape handed to notification.
type ScoredJob struct {
	Job    Job
	Score  int // 0–100 or agreed scale
	Reason string
}
