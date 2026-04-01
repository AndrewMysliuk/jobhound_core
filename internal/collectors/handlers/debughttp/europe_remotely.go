package debughttp

import "net/http"

func (h *HTTPHandler) postEuropeRemotely(w http.ResponseWriter, r *http.Request) {
	runCollectorDebug(w, r, h.europeRemotely, nil, h.europeRemotelyConcrete)
}
