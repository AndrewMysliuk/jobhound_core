package handlers

import (
	"net/http"
	"strings"
)

func stringsTrimPathValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.PathValue(key))
}
