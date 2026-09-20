package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postVueJobs(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postVueJobs").Logger())
	runCollectorDebug(w, r, logH, h.vueJobs, nil, nil, nil, nil, nil, nil, h.vueJobsConcrete, nil)
}
