package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postBuiltin(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postBuiltin").Logger())
	runCollectorDebug(w, r, logH, h.builtin, h.workingNomadsConcrete, h.europeRemotelyConcrete, h.douUaConcrete, h.himalayasConcrete, h.djinniConcrete, h.builtinConcrete)
}
