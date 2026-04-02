package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/google/uuid"
)

var testSlotIDA = uuid.MustParse("11111111-1111-4111-8111-111111111111")

func TestCanonicalBroadFilterKeyJSON_keyOrderAndNormalization(t *testing.T) {
	from := time.Date(2026, 1, 2, 15, 4, 5, 123456789, time.UTC)
	to := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	p := BroadFilterKeyParts{
		SlotID:      testSlotIDA,
		Role:        "  Go  ",
		TimeFromUTC: &from,
		TimeToUTC:   &to,
		Sources:     []string{"  B ", "a"},
		Keywords:    []string{" Z ", "a"},
	}
	got, err := CanonicalBroadFilterKeyJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"slot_id":"11111111-1111-4111-8111-111111111111","role":"go","time_window":{"from":"2026-01-02T15:04:05.123456789Z","to":"2026-01-09T00:00:00Z"},"sources":["a","b"],"keywords":["a","z"]}`
	if got != want {
		t.Fatalf("canonical JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestCanonicalBroadFilterKeyJSON_optionalUserID(t *testing.T) {
	from := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	u := "  User_ABC  "
	p := BroadFilterKeyParts{
		SlotID:      testSlotIDA,
		UserID:      &u,
		TimeFromUTC: &from,
		TimeToUTC:   &to,
		Sources:     []string{"s1"},
	}
	got, err := CanonicalBroadFilterKeyJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"slot_id":"11111111-1111-4111-8111-111111111111","user_id":"user_abc","time_window":{"from":"2026-01-02T00:00:00Z","to":"2026-01-09T00:00:00Z"},"sources":["s1"]}`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBroadFilterKeyHashHex_deterministic(t *testing.T) {
	from := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	to := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	p := BroadFilterKeyParts{SlotID: testSlotIDA, Role: "x", TimeFromUTC: &from, TimeToUTC: &to, Sources: []string{"s"}}
	j, err := CanonicalBroadFilterKeyJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(j))
	want := hex.EncodeToString(sum[:])

	got, err := BroadFilterKeyHashHex(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || len(got) != 64 {
		t.Fatalf("hash: got %q (%d) want %q", got, len(got), want)
	}
}

func TestBroadFilterKeyHashFromRules_equivalentSourcesOrder(t *testing.T) {
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	rules := pipeline.BroadFilterRules{
		From:             &from,
		To:               &to,
		RoleSynonyms:     []string{"Go", "Rust"},
		RemoteOnly:       true,
		CountryAllowlist: []string{"DE", "pl"},
	}
	slot := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	h1, err := BroadFilterKeyHashFromRules(rules, []string{"b", "a"}, slot, nil)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := BroadFilterKeyHashFromRules(rules, []string{"a", "b"}, slot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("same rules different source order: %q vs %q", h1, h2)
	}
}

func TestBroadFilterKeyHashFromRules_differentSlots(t *testing.T) {
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	rules := pipeline.BroadFilterRules{From: &from, To: &to, RoleSynonyms: []string{"go"}}
	sa := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	sb := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	h1, err := BroadFilterKeyHashFromRules(rules, []string{"src"}, sa, nil)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := BroadFilterKeyHashFromRules(rules, []string{"src"}, sb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatal("expected different hashes for different slot_id")
	}
}
