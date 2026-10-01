package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) deleteSlot(w http.ResponseWriter, r *http.Request) {
	id := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), id)
	err := h.deps.Slots.Delete(ctx, slotschema.DeleteSlotParams{SlotID: id})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
