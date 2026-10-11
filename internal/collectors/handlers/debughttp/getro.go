package debughttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func (h *HTTPHandler) postGetro(w http.ResponseWriter, r *http.Request) {
	logH := logging.EnrichWithContext(r.Context(), h.log.With().Str(logging.FieldHandler, "postGetro").Logger())

	raw, err := readDebugRequestBody(w, r)
	if err != nil {
		logH.Error().Err(err).Msg("read request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := parseCollectorsPOSTBody(raw)
	if err != nil {
		logH.Error().Err(err).Msg("parse json body")
		http.Error(w, fmt.Sprintf("JSON body: %v", err), http.StatusBadRequest)
		return
	}

	sourceID := ""
	if req.SourceID != nil {
		sourceID = strings.TrimSpace(*req.SourceID)
	}
	if sourceID == "" {
		logH.Error().Msg("source_id is required")
		http.Error(w, "source_id is required", http.StatusBadRequest)
		return
	}
	board, ok := lookupGetroBoard(h.getroBoards, sourceID)
	if !ok {
		logH.Error().Str(logging.FieldSourceID, sourceID).Msg("unknown source_id")
		http.Error(w, "unknown source_id", http.StatusNotFound)
		return
	}

	copied := *board
	if req.MaxPages != nil {
		copied.MaxPages = *req.MaxPages
	}
	query := "vue"
	if req.Q != nil {
		query = *req.Q
	}

	logH.Debug().Str(logging.FieldSourceID, sourceID).Msg("debug collector fetch")
	ctx := r.Context()
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	jobs, fetchErr := copied.FetchWithQuery(ctx, query)
	w.Header().Set("Content-Type", "application/json")
	if fetchErr != nil {
		logH.Error().Err(fetchErr).Str(logging.FieldSourceID, sourceID).Msg("collector fetch failed")
		_ = json.NewEncoder(w).Encode(runCollectorResponse{
			OK:        false,
			Collector: copied.Name(),
			Error:     fetchErr.Error(),
		})
		return
	}

	resp := runCollectorResponse{
		OK:        true,
		Collector: copied.Name(),
		Count:     len(jobs),
		Jobs:      make([]jobDebugJSON, 0, len(jobs)),
	}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, jobToDebugJSON(j))
	}
	_ = json.NewEncoder(w).Encode(resp)
}
