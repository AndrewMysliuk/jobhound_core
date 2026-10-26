package ingest

import (
	"testing"
	"time"

	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

func TestSkipIngestByPostedAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	tests := []struct {
		name   string
		posted time.Time
		want   bool
	}{
		{
			name:   "zero posted_at is never skipped",
			posted: time.Time{},
			want:   false,
		},
		{
			name:   "sixty days is skipped",
			posted: now.Add(-60 * day),
			want:   true,
		},
		{
			name:   "ten days is kept",
			posted: now.Add(-10 * day),
			want:   false,
		},
		{
			name:   "exactly reject days is kept",
			posted: now.Add(-time.Duration(scoringschema.PostedAtRejectDays) * day),
			want:   false,
		},
		{
			name:   "strictly older than reject days is skipped",
			posted: now.Add(-time.Duration(scoringschema.PostedAtRejectDays)*day - time.Second),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SkipIngestByPostedAt(tt.posted, now)
			if got != tt.want {
				t.Fatalf("SkipIngestByPostedAt(%v) = %v, want %v", tt.posted, got, tt.want)
			}
		})
	}
}
