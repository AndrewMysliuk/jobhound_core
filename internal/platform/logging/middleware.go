package logging

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// HeaderRequestID is the HTTP header used for request correlation (specs/010-observability/contracts/logging.md).
const HeaderRequestID = "X-Request-ID"

const maxRequestIDLen = 128

// RequestIDMiddleware ensures each request has a request_id on the context and echoes X-Request-ID on the response.
// Outermost placement (before CORS) keeps OPTIONS and API routes consistent.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(HeaderRequestID))
		if id == "" {
			id = uuid.New().String()
		} else if len(id) > maxRequestIDLen {
			id = id[:maxRequestIDLen]
		}
		ctx := WithRequestID(r.Context(), id)
		r = r.WithContext(ctx)
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r)
	})
}
