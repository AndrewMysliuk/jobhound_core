package schema

import "time"

// SlotListItem is one row in GET /api/v1/slots (contracts/http-public-api.md §4.1).
type SlotListItem struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	CreatedAt time.Time    `json:"created_at"`
	Stage1    StageCompact `json:"stage_1"`
	Stage2    StageCompact `json:"stage_2"`
	Stage3    StageCompact `json:"stage_3"`
}

// SlotsListResponse is GET /api/v1/slots 200 body.
type SlotsListResponse struct {
	Slots []SlotListItem `json:"slots"`
}

// SlotCard is GET /api/v1/slots/{id} and POST /api/v1/slots 201 (full stages).
type SlotCard struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Stage1    StageFull `json:"stage_1"`
	Stage2    StageFull `json:"stage_2"`
	Stage3    StageFull `json:"stage_3"`
}

// CreateSlotRequest is POST /api/v1/slots body.
type CreateSlotRequest struct {
	Name string `json:"name"`
}
