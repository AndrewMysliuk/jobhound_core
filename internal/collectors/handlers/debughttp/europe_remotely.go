package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postEuropeRemotely(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postEuropeRemotely").Logger())
	runCollectorDebug(w, r, logH, h.europeRemotely, nil, h.europeRemotelyConcrete, nil, h.himalayasConcrete, nil)
}
