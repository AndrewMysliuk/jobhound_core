package impl

import (
	"strings"
	"testing"
)

func TestDecodeModelReplySchemaInvalidNotUnknown(t *testing.T) {
	t.Parallel()

	// Missing required top-level fields: must fail closed, not coerce to all-unknown.
	body := []byte(`{"office":{"countries":[],"regions":[],"quote":null,"unknown":true}}`)
	got, err := decodeModelReply(body)
	if err == nil {
		t.Fatalf("expected schema error, got reply %#v (must not store all-unknown on first failure)", got)
	}
	if !strings.Contains(err.Error(), "model reply schema") {
		t.Fatalf("error = %v, want schema validation failure", err)
	}
	// Zero value on error — not a coerced all-unknown abstention.
	if got.Office.Unknown || got.HiringScope.Value == unknownAnswer {
		t.Fatalf("schema failure must not look like unknown abstention, got %#v", got)
	}
}

func TestDecodeModelReplyValidRoundTrip(t *testing.T) {
	t.Parallel()

	body := []byte(`{
  "office": {"countries": [], "regions": [], "quote": null, "unknown": true},
  "hiring_scope": {"value": "unknown", "countries": [], "regions": [], "quote": null},
  "work_authorization": {"value": "unknown", "countries": [], "quote": null},
  "timezone": {"value": "unknown", "zones": [], "quote": null},
  "compensation": {"currency": "", "min": null, "max": null, "period": "", "quote": null, "unknown": true},
  "work_mode": {"value": "unknown", "quote": null},
  "employment_load": {"value": "unknown", "quote": null},
  "engagement": {"value": "unknown", "quote": null}
}`)
	got, err := decodeModelReply(body)
	if err != nil {
		t.Fatalf("decodeModelReply: %v", err)
	}
	if !got.Office.Unknown || got.HiringScope.Value != unknownAnswer {
		t.Fatalf("unexpected reply %#v", got)
	}
}
