package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalListingURL(t *testing.T) {
	got, err := CanonicalListingURL("HTTPS://Example.COM/job/1/")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.com/job/1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStableJobIDForListing(t *testing.T) {
	id, err := StableJobIDForListing("europe_remotely", "https://euremotejobs.com/x/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(id, "europe_remotely") || !strings.Contains(id, "https://euremotejobs.com/x") {
		t.Fatalf("unexpected id %q", id)
	}
}

func TestCountryResolver_Alpha2ForName(t *testing.T) {
	r, err := LoadCountryResolver(strings.NewReader(`[
		{"alpha_2":"DE","name":"Germany"},
		{"alpha_2":"US","name":"United States of America"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Alpha2ForName("germany"); got != "DE" {
		t.Fatalf("germany: got %q", got)
	}
	if got := r.Alpha2ForName("UK"); got != "GB" {
		t.Fatalf("UK alias: got %q want GB", got)
	}
	if got := r.Alpha2ForName("Remote"); got != "" {
		t.Fatalf("remote token: got %q want empty", got)
	}
}

func TestLoadCountryResolver_repoData(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	p := filepath.Join(root, "data", "countries.json")
	f, err := os.Open(p)
	if err != nil {
		t.Skip("data/countries.json not reachable from test cwd:", err)
	}
	defer f.Close()
	r, err := LoadCountryResolver(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Alpha2ForName("United States of America"); got != "US" {
		t.Fatalf("United States of America: got %q want US", got)
	}
}

func TestNewHTTPClient(t *testing.T) {
	c := NewHTTPClient()
	if c == nil || c.Timeout != DefaultHTTPTimeout {
		t.Fatalf("client: %+v", c)
	}
}

func TestStripHTMLToPlainText(t *testing.T) {
	got := StripHTMLToPlainText("<p>Ship features end-to-end.</p>")
	if got != "Ship features end-to-end." {
		t.Fatalf("got %q", got)
	}
}

func TestInferPosition_fullStackInTitle(t *testing.T) {
	p := InferPosition("Senior Full Stack Developer", "x", []string{"go"})
	if p == nil || *p != "full-stack" {
		t.Fatalf("got %v", p)
	}
}
