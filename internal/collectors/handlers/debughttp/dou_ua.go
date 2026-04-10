package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postDouUA(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postDouUA").Logger())
	runCollectorDebug(w, r, logH, h.douUa, nil, nil, h.douUaConcrete, h.himalayasConcrete, nil)
}
