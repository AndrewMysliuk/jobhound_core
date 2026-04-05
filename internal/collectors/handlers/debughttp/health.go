package debughttp

import (
	"encoding/json"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) health(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "health").Logger())
	logH.Debug().Msg("health")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
