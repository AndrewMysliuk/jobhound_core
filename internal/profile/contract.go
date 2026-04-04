package profile

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
)

// API is the application surface for GET/PUT /api/v1/profile.
type API interface {
	Get(ctx context.Context) (schema.ProfileResponse, error)
	Put(ctx context.Context, text string) (schema.ProfileResponse, error)
}
