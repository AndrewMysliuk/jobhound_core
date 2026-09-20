package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postWellfound(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postWellfound").Logger())
	runCollectorDebug(w, r, logH, h.wellfound, nil, nil, nil, nil, nil, h.wellfoundConcrete, nil, nil)
}
