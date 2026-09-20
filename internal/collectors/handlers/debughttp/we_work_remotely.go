package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postWeWorkRemotely(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postWeWorkRemotely").Logger())
	runCollectorDebug(w, r, logH, h.weWorkRemotely, nil, nil, nil, nil, nil, nil, nil, nil)
}
