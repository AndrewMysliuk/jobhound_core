package storage

import (
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

func strPtr(s string) *string { return &s }

func strPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func domainJobEqual(a, b domain.Job) bool {
	if a.ID != b.ID || a.Source != b.Source || a.Title != b.Title || a.Company != b.Company ||
		a.URL != b.URL || a.ApplyURL != b.ApplyURL || a.Description != b.Description {
		return false
	}
	if !a.PostedAt.Equal(b.PostedAt) {
		return false
	}
	switch {
	case a.UserID == nil && b.UserID == nil:
		return true
	case a.UserID == nil || b.UserID == nil:
		return false
	default:
		return *a.UserID == *b.UserID
	}
}

func TestNewJobModel(t *testing.T) {
	posted := time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC)
	uid := "user-42"

	cases := []struct {
		name         string
		in           domain.Job
		wantApplyURL *string
		wantPostedAt *time.Time
		wantUserID   *string
	}{
		{
			name:         "empty optional fields stay nil pointers",
			in:           domain.Job{},
			wantApplyURL: nil,
			wantPostedAt: nil,
			wantUserID:   nil,
		},
		{
			name:         "apply_url set when non-empty",
			in:           domain.Job{ApplyURL: "https://apply.example/1"},
			wantApplyURL: strPtr("https://apply.example/1"),
			wantPostedAt: nil,
			wantUserID:   nil,
		},
		{
			name:         "apply_url empty leaves nil",
			in:           domain.Job{ApplyURL: ""},
			wantApplyURL: nil,
		},
		{
			name:         "posted_at zero leaves nil",
			in:           domain.Job{PostedAt: time.Time{}},
			wantPostedAt: nil,
		},
		{
			name:         "posted_at non-zero is copied",
			in:           domain.Job{PostedAt: posted},
			wantPostedAt: &posted,
		},
		{
			name:       "user_id nil stays nil",
			in:         domain.Job{UserID: nil},
			wantUserID: nil,
		},
		{
			name:       "user_id empty string pointer omitted in row",
			in:         domain.Job{UserID: strPtr("")},
			wantUserID: nil,
		},
		{
			name:       "user_id non-empty is copied",
			in:         domain.Job{UserID: &uid},
			wantUserID: &uid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewJobModel(tc.in)
			if got.ID != tc.in.ID || got.Source != tc.in.Source || got.Title != tc.in.Title ||
				got.Company != tc.in.Company || got.URL != tc.in.URL || got.Description != tc.in.Description {
				t.Fatalf("scalar fields: got %+v want fields from in %+v", got, tc.in)
			}
			if !strPtrEqual(got.ApplyURL, tc.wantApplyURL) {
				t.Fatalf("ApplyURL: got %v want %v", got.ApplyURL, tc.wantApplyURL)
			}
			if !timePtrEqual(got.PostedAt, tc.wantPostedAt) {
				t.Fatalf("PostedAt: got %v want %v", got.PostedAt, tc.wantPostedAt)
			}
			if !strPtrEqual(got.UserID, tc.wantUserID) {
				t.Fatalf("UserID: got %v want %v", got.UserID, tc.wantUserID)
			}
		})
	}
}

func TestJob_ToDomain(t *testing.T) {
	posted := time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)
	apply := "https://apply"
	uid := "u1"
	empty := ""

	cases := []struct {
		name string
		m    Job
		want domain.Job
	}{
		{
			name: "posted_at nil maps to zero time",
			m:    Job{PostedAt: nil},
			want: domain.Job{PostedAt: time.Time{}},
		},
		{
			name: "posted_at set",
			m:    Job{PostedAt: &posted},
			want: domain.Job{PostedAt: posted},
		},
		{
			name: "apply_url nil is empty string",
			m:    Job{ApplyURL: nil},
			want: domain.Job{ApplyURL: ""},
		},
		{
			name: "apply_url non-nil including empty",
			m:    Job{ApplyURL: &empty},
			want: domain.Job{ApplyURL: ""},
		},
		{
			name: "apply_url value",
			m:    Job{ApplyURL: &apply},
			want: domain.Job{ApplyURL: apply},
		},
		{
			name: "user_id nil",
			m:    Job{UserID: nil},
			want: domain.Job{UserID: nil},
		},
		{
			name: "user_id empty string omitted in domain",
			m:    Job{UserID: &empty},
			want: domain.Job{UserID: nil},
		},
		{
			name: "user_id set",
			m:    Job{UserID: &uid},
			want: domain.Job{UserID: &uid},
		},
		{
			name: "full row",
			m: Job{
				ID: "id1", Source: "src", Title: "t", Company: "co", URL: "https://list",
				ApplyURL: &apply, Description: "desc", PostedAt: &posted, UserID: &uid,
			},
			want: domain.Job{
				ID: "id1", Source: "src", Title: "t", Company: "co", URL: "https://list",
				ApplyURL: apply, Description: "desc", PostedAt: posted, UserID: &uid,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.m.ToDomain()
			if !domainJobEqual(tc.want, got) {
				t.Fatalf("ToDomain() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestJobModel_roundTrip(t *testing.T) {
	posted := time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)
	uid := "alice"

	cases := []struct {
		name string
		in   domain.Job
		want domain.Job // normalized expectation after model round-trip
	}{
		{
			name: "minimal",
			in:   domain.Job{},
			want: domain.Job{},
		},
		{
			name: "all fields",
			in: domain.Job{
				ID: "j1", Source: "board", Title: "Eng", Company: "Co", URL: "https://job",
				ApplyURL: "https://ats", Description: "text", PostedAt: posted, UserID: &uid,
			},
			want: domain.Job{
				ID: "j1", Source: "board", Title: "Eng", Company: "Co", URL: "https://job",
				ApplyURL: "https://ats", Description: "text", PostedAt: posted, UserID: &uid,
			},
		},
		{
			name: "optional apply and times zero",
			in: domain.Job{
				ID: "x", Source: "s", Title: "t", Company: "c", URL: "u", Description: "d",
			},
			want: domain.Job{
				ID: "x", Source: "s", Title: "t", Company: "c", URL: "u", Description: "d",
			},
		},
		{
			name: "user_id empty pointer normalized away",
			in:   domain.Job{ID: "only", UserID: strPtr("")},
			want: domain.Job{ID: "only", UserID: nil},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewJobModel(tc.in)
			got := m.ToDomain()
			if !domainJobEqual(tc.want, got) {
				t.Fatalf("round-trip got %+v, want %+v", got, tc.want)
			}
		})
	}
}
