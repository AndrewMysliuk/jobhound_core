package schema

import "time"

// ProfileResponse is GET/PUT /api/v1/profile 200 body.
type ProfileResponse struct {
	Text      string    `json:"text"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProfilePutRequest is PUT /api/v1/profile body.
type ProfilePutRequest struct {
	Text string `json:"text"`
}
