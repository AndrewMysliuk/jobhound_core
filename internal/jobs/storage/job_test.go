package storage

import (
	"bytes"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func strPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

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

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func floatSliceEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func domainJobEqual(a, b schema.Job) bool {
	if a.ID != b.ID || a.Source != b.Source || a.Title != b.Title || a.Company != b.Company ||
		a.URL != b.URL || a.ApplyURL != b.ApplyURL || a.Description != b.Description ||
		a.SalaryRaw != b.SalaryRaw {
		return false
	}
	if !stringSliceEqual(a.Tags, b.Tags) {
		return false
	}
	if !floatSliceEqual(a.TimezoneOffsets, b.TimezoneOffsets) {
		return false
	}
	if !strPtrEqual(a.Position, b.Position) {
		return false
	}
	if !a.PostedAt.Equal(b.PostedAt) {
		return false
	}
	if !boolPtrEqual(a.Remote, b.Remote) {
		return false
	}
	if a.CountryCode != b.CountryCode {
		return false
	}
	switch {
	case a.UserID == nil && b.UserID == nil:
	case a.UserID == nil || b.UserID == nil:
		return false
	default:
		if *a.UserID != *b.UserID {
			return false
		}
	}
	switch {
	case a.Stage1Status == nil && b.Stage1Status == nil:
		return true
	case a.Stage1Status == nil || b.Stage1Status == nil:
		return false
	default:
		return *a.Stage1Status == *b.Stage1Status
	}
}

func TestNewJobModel(t *testing.T) {
	posted := time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC)
	uid := "user-42"

	cases := []struct {
		name         string
		in           schema.Job
		wantApplyURL *string
		wantPostedAt *time.Time
		wantIsRemote *bool
		wantCountry  string
		wantUserID   *string
	}{
		{
			name:         "empty optional fields stay nil pointers",
			in:           schema.Job{},
			wantApplyURL: nil,
			wantPostedAt: nil,
			wantIsRemote: nil,
			wantCountry:  "",
			wantUserID:   nil,
		},
		{
			name:         "apply_url set when non-empty",
			in:           schema.Job{ApplyURL: "https://apply.example/1"},
			wantApplyURL: strPtr("https://apply.example/1"),
			wantPostedAt: nil,
			wantIsRemote: nil,
			wantCountry:  "",
			wantUserID:   nil,
		},
		{
			name:         "apply_url empty leaves nil",
			in:           schema.Job{ApplyURL: ""},
			wantApplyURL: nil,
		},
		{
			name:         "posted_at zero leaves nil",
			in:           schema.Job{PostedAt: time.Time{}},
			wantPostedAt: nil,
		},
		{
			name:         "posted_at non-zero is copied",
			in:           schema.Job{PostedAt: posted},
			wantPostedAt: &posted,
		},
		{
			name:         "remote true",
			in:           schema.Job{Remote: boolPtr(true)},
			wantIsRemote: boolPtr(true),
		},
		{
			name:         "remote false",
			in:           schema.Job{Remote: boolPtr(false)},
			wantIsRemote: boolPtr(false),
		},
		{
			name:        "country code",
			in:          schema.Job{CountryCode: "de"},
			wantCountry: "de",
		},
		{
			name:        "salary tags position",
			in:          schema.Job{SalaryRaw: "50-70k", Tags: []string{"rust"}, Position: strPtr("backend")},
			wantCountry: "",
		},
		{
			name:        "timezone offsets json",
			in:          schema.Job{TimezoneOffsets: []float64{5.5, -3.5}},
			wantCountry: "",
		},
		{
			name:       "user_id nil stays nil",
			in:         schema.Job{UserID: nil},
			wantUserID: nil,
		},
		{
			name:       "user_id empty string pointer omitted in row",
			in:         schema.Job{UserID: strPtr("")},
			wantUserID: nil,
		},
		{
			name:       "user_id non-empty is copied",
			in:         schema.Job{UserID: &uid},
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
			if !boolPtrEqual(got.IsRemote, tc.wantIsRemote) {
				t.Fatalf("IsRemote: got %v want %v", got.IsRemote, tc.wantIsRemote)
			}
			if got.CountryCode != tc.wantCountry {
				t.Fatalf("CountryCode: got %q want %q", got.CountryCode, tc.wantCountry)
			}
			if got.SalaryRaw != tc.in.SalaryRaw {
				t.Fatalf("SalaryRaw: got %q want %q", got.SalaryRaw, tc.in.SalaryRaw)
			}
			if !bytes.Equal(got.Tags, encodeJobTags(tc.in.Tags)) {
				t.Fatalf("Tags: got %s want %s", got.Tags, encodeJobTags(tc.in.Tags))
			}
			if !bytes.Equal(got.TimezoneOffsets, encodeTimezoneOffsets(tc.in.TimezoneOffsets)) {
				t.Fatalf("TimezoneOffsets: got %s want %s", got.TimezoneOffsets, encodeTimezoneOffsets(tc.in.TimezoneOffsets))
			}
			if !strPtrEqual(got.Position, tc.in.Position) {
				t.Fatalf("Position: got %v want %v", got.Position, tc.in.Position)
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
		want schema.Job
	}{
		{
			name: "posted_at nil maps to zero time",
			m:    Job{PostedAt: nil},
			want: schema.Job{PostedAt: time.Time{}},
		},
		{
			name: "posted_at set",
			m:    Job{PostedAt: &posted},
			want: schema.Job{PostedAt: posted},
		},
		{
			name: "apply_url nil is empty string",
			m:    Job{ApplyURL: nil},
			want: schema.Job{ApplyURL: ""},
		},
		{
			name: "apply_url non-nil including empty",
			m:    Job{ApplyURL: &empty},
			want: schema.Job{ApplyURL: ""},
		},
		{
			name: "apply_url value",
			m:    Job{ApplyURL: &apply},
			want: schema.Job{ApplyURL: apply},
		},
		{
			name: "user_id nil",
			m:    Job{UserID: nil},
			want: schema.Job{UserID: nil},
		},
		{
			name: "user_id empty string omitted in domain",
			m:    Job{UserID: &empty},
			want: schema.Job{UserID: nil},
		},
		{
			name: "user_id set",
			m:    Job{UserID: &uid},
			want: schema.Job{UserID: &uid},
		},
		{
			name: "full row",
			m: Job{
				ID: "id1", Source: "src", Title: "t", Company: "co", URL: "https://list",
				ApplyURL: &apply, Description: "desc", PostedAt: &posted, UserID: &uid,
				TimezoneOffsets: encodeTimezoneOffsets([]float64{8}),
			},
			want: schema.Job{
				ID: "id1", Source: "src", Title: "t", Company: "co", URL: "https://list",
				ApplyURL: apply, Description: "desc", PostedAt: posted, UserID: &uid,
				TimezoneOffsets: []float64{8},
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
		in   schema.Job
		want schema.Job // normalized expectation after model round-trip
	}{
		{
			name: "minimal",
			in:   schema.Job{},
			want: schema.Job{},
		},
		{
			name: "all fields",
			in: schema.Job{
				ID: "j1", Source: "board", Title: "Eng", Company: "Co", URL: "https://job",
				ApplyURL: "https://ats", Description: "text", PostedAt: posted, UserID: &uid,
			},
			want: schema.Job{
				ID: "j1", Source: "board", Title: "Eng", Company: "Co", URL: "https://job",
				ApplyURL: "https://ats", Description: "text", PostedAt: posted, UserID: &uid,
			},
		},
		{
			name: "optional apply and times zero",
			in: schema.Job{
				ID: "x", Source: "s", Title: "t", Company: "c", URL: "u", Description: "d",
			},
			want: schema.Job{
				ID: "x", Source: "s", Title: "t", Company: "c", URL: "u", Description: "d",
			},
		},
		{
			name: "user_id empty pointer normalized away",
			in:   schema.Job{ID: "only", UserID: strPtr("")},
			want: schema.Job{ID: "only", UserID: nil},
		},
		{
			name: "remote and country",
			in: schema.Job{
				ID: "j2", Remote: boolPtr(true), CountryCode: "US",
			},
			want: schema.Job{
				ID: "j2", Remote: boolPtr(true), CountryCode: "US",
			},
		},
		{
			name: "salary tags position",
			in: schema.Job{
				ID: "j3", SalaryRaw: "€80k", Tags: []string{"go", "backend"}, Position: strPtr("backend"),
			},
			want: schema.Job{
				ID: "j3", SalaryRaw: "€80k", Tags: []string{"go", "backend"}, Position: strPtr("backend"),
			},
		},
		{
			name: "timezone offsets",
			in: schema.Job{
				ID: "jtz", TimezoneOffsets: []float64{5.5},
			},
			want: schema.Job{
				ID: "jtz", TimezoneOffsets: []float64{5.5},
			},
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
