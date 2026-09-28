package utils

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
)

// StringsTrimPathValue returns r.PathValue(key) with surrounding spaces trimmed.
func StringsTrimPathValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.PathValue(key))
}

// ParseStageDigit parses a single-character stage path value ("1", "2", or "3").
// Returns the digit and true on success; 0 and false on any invalid input.
func ParseStageDigit(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 1 || s[0] < '1' || s[0] > '3' {
		return 0, false
	}
	return int(s[0] - '0'), true
}

// ParseJobListQuery extracts page, limit, optional status filter, and debug=1 from query params, applying defaults.
// status is non-empty when ?status= is present (stages 2–3 job lists).
// debug is true only when ?debug=1 exactly; any other non-empty debug value yields ok=false.
// Returns ok=false if any present value is malformed or out of range.
func ParseJobListQuery(q map[string][]string) (page, limit int, status string, debug bool, ok bool) {
	page = 1
	if vs := q["page"]; len(vs) > 0 && strings.TrimSpace(vs[0]) != "" {
		p, err := strconv.Atoi(strings.TrimSpace(vs[0]))
		if err != nil || p < 1 {
			return 0, 0, "", false, false
		}
		page = p
	}
	limit = schema.DefaultJobListLimit
	if vs := q["limit"]; len(vs) > 0 && strings.TrimSpace(vs[0]) != "" {
		l, err := strconv.Atoi(strings.TrimSpace(vs[0]))
		if err != nil || l < 1 || l > schema.MaxJobListLimit {
			return 0, 0, "", false, false
		}
		limit = l
	}
	if vs := q["status"]; len(vs) > 0 {
		status = strings.TrimSpace(vs[0])
	}
	if vs := q["debug"]; len(vs) > 0 {
		d := strings.TrimSpace(vs[0])
		if d == "" {
			return page, limit, status, false, true
		}
		if d != "1" {
			return 0, 0, "", false, false
		}
		debug = true
	}
	return page, limit, status, debug, true
}
