package debughttp

import "net/http"

func (h *HTTPHandler) postWorkingNomads(w http.ResponseWriter, r *http.Request) {
	runCollectorDebug(w, r, h.workingNomads, h.workingNomadsConcrete, nil)
}
