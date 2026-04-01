package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

func TestCanonicalBroadFilterKeyJSON_keyOrderAndNormalization(t *testing.T) {
	from := time.Date(2026, 1, 2, 15, 4, 5, 123456789, time.UTC)
	to := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	p := BroadFilterKeyParts{
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
	want := `{"role":"go","time_window":{"from":"2026-01-02T15:04:05.123456789Z","to":"2026-01-09T00:00:00Z"},"sources":["a","b"],"keywords":["a","z"]}`
	if got != want {
		t.Fatalf("canonical JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestBroadFilterKeyHashHex_deterministic(t *testing.T) {
	from := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	to := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	p := BroadFilterKeyParts{Role: "x", TimeFromUTC: &from, TimeToUTC: &to, Sources: []string{"s"}}
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
	h1, err := BroadFilterKeyHashFromRules(rules, []string{"b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := BroadFilterKeyHashFromRules(rules, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("same rules different source order: %q vs %q", h1, h2)
	}
}
