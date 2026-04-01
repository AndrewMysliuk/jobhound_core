package storage

import (
	"slices"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// jobEqualForIngestSkip compares persisted vacancy fields used for ingest skip (006 spec.md):
// all fields except description; excludes created_at/updated_at (not on domain.Job).
func jobEqualForIngestSkip(a, b domain.Job) bool {
	if a.ID != b.ID || a.Source != b.Source || a.Title != b.Title || a.Company != b.Company ||
		a.URL != b.URL || a.ApplyURL != b.ApplyURL || a.SalaryRaw != b.SalaryRaw || a.CountryCode != b.CountryCode {
		return false
	}
	if !a.PostedAt.Equal(b.PostedAt) {
		return false
	}
	if !ptrEqualBool(a.Remote, b.Remote) {
		return false
	}
	if !ptrEqualString(a.Position, b.Position) {
		return false
	}
	if !ptrEqualString(a.UserID, b.UserID) {
		return false
	}
	if !ptrEqualString(a.Stage1Status, b.Stage1Status) {
		return false
	}
	return stringSliceEqualSorted(a.Tags, b.Tags)
}

func ptrEqualString(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrEqualBool(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stringSliceEqualSorted(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	slices.Sort(ac)
	slices.Sort(bc)
	return slices.Equal(ac, bc)
}

func normalizeStage1(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
