package utils

import (
	"net/http"
	"strings"
)

// StringsTrimPathValue returns r.PathValue(key) with surrounding spaces trimmed.
func StringsTrimPathValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.PathValue(key))
}
