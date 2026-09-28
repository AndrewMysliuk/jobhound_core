package utils

import (
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// DuplicateJobIDs maps duplicate job IDs to the survivor job ID for the same normalized title+company key.
func DuplicateJobIDs(jobs []schema.Job) map[string]string {
	type group struct {
		jobs []schema.Job
	}
	byKey := make(map[string]*group)
	for _, j := range jobs {
		key := dedupKey(j)
		g, ok := byKey[key]
		if !ok {
			g = &group{}
			byKey[key] = g
		}
		g.jobs = append(g.jobs, j)
	}
	out := make(map[string]string)
	for _, g := range byKey {
		if len(g.jobs) < 2 {
			continue
		}
		survivor := pickDedupSurvivor(g.jobs)
		for _, j := range g.jobs {
			if j.ID == survivor.ID {
				continue
			}
			out[j.ID] = survivor.ID
		}
	}
	return out
}

func dedupKey(j schema.Job) string {
	title := strings.ToLower(strings.TrimSpace(j.Title))
	company := strings.ToLower(strings.TrimSpace(j.Company))
	return title + "|" + company
}

func pickDedupSurvivor(jobs []schema.Job) schema.Job {
	best := jobs[0]
	for _, j := range jobs[1:] {
		if dedupPostedBefore(j.PostedAt, best.PostedAt) {
			best = j
			continue
		}
		if j.PostedAt.Equal(best.PostedAt) && strings.Compare(j.ID, best.ID) < 0 {
			best = j
		}
	}
	return best
}

func dedupPostedBefore(a, b time.Time) bool {
	aZero := a.IsZero()
	bZero := b.IsZero()
	if aZero && bZero {
		return false
	}
	if aZero {
		return false
	}
	if bZero {
		return true
	}
	return a.Before(b)
}
