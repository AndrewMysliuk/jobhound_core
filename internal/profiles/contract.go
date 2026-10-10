// Package profiles is the search-profile module: contracts at the root, data model under schema/.
package profiles

import (
	"context"
	"errors"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
)

// Store loads search profiles.
type Store interface {
	List(ctx context.Context) ([]schema.Profile, error)
	Get(ctx context.Context, id string) (schema.Profile, error)
}

var (
	ErrProfileNotFound = errors.New("profiles: profile not found")
	ErrProfileInvalid  = errors.New("profiles: profile file invalid")
)
