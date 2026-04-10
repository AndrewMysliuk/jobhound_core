package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postWorkingNomads(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postWorkingNomads").Logger())
	runCollectorDebug(w, r, logH, h.workingNomads, h.workingNomadsConcrete, nil, nil, h.himalayasConcrete)
}
