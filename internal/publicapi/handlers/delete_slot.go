package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) deleteSlot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	id := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), id)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "deleteSlot").Logger())
	err := h.deps.Slots.Delete(ctx, id)
	if errors.Is(err, slots.ErrNotFound) {
		apputils.WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("delete slot")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
