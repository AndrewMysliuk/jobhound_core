package storage

import (
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func mergeIngestJob(existing, incoming schema.Job, now time.Time) schema.Job {
	out := existing
	out.Sources = mergeSourceIDs(existing.Sources, incoming.Source, incoming.Sources)
	out.LastSeenAt = now
	existingATS := sourcesIncludeATS(existing.Sources)
	incomingATS := sourcesIncludeATS(append(append([]string{}, incoming.Sources...), incoming.Source))
	out.URL = preferURL(existing.URL, incoming.URL, existingATS, incomingATS)
	out.ApplyURL = preferURL(existing.ApplyURL, incoming.ApplyURL, existingATS, incomingATS)
	return out
}

func preferURL(existing, incoming string, existingATS, incomingATS bool) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if incomingATS && incoming != "" {
		return incoming
	}
	if existingATS && existing != "" {
		return existing
	}
	if existing != "" {
		return existing
	}
	return incoming
}

func sourcesIncludeATS(ids []string) bool {
	for _, id := range ids {
		if atsSource(id) {
			return true
		}
	}
	return false
}

func atsSource(id string) bool {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "greenhouse", "lever", "ashby", "workable", "recruitee", "personio", "smartrecruiters":
		return true
	default:
		return false
	}
}
