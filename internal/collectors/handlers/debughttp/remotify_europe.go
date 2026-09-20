package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postRemotifyEurope(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postRemotifyEurope").Logger())
	runCollectorDebug(w, r, logH, h.remotifyEurope, nil, nil, nil, nil, h.remotifyEuropeConcrete, nil, nil, nil)
}
