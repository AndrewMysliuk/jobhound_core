package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postHimalayas(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postHimalayas").Logger())
	runCollectorDebug(w, r, logH, h.himalayas, h.workingNomadsConcrete, h.europeRemotelyConcrete, h.douUaConcrete, h.himalayasConcrete, nil)
}
