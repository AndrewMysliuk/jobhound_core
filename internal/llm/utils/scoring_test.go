package utils

import "testing"

func TestParseScoringJSON_happy(t *testing.T) {
	score, r, err := ParseScoringJSON([]byte(`{"score":42,"rationale":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 42 || r != "ok" {
		t.Fatalf("got score=%d rationale=%q", score, r)
	}
}

func TestParseScoringJSON_invalidJSON(t *testing.T) {
	_, _, err := ParseScoringJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseScoringJSON_missingScore(t *testing.T) {
	_, _, err := ParseScoringJSON([]byte(`{"rationale":"only"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseScoringJSON_missingRationale(t *testing.T) {
	_, _, err := ParseScoringJSON([]byte(`{"score":1}`))
	if err == nil {
		t.Fatal("expected error")
	}
}
