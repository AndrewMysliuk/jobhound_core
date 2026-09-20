package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postGolangCafe(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postGolangCafe").Logger())
	runCollectorDebug(w, r, logH, h.golangCafe, nil, nil, nil, nil, nil, nil, nil, h.golangCafeConcrete)
}
