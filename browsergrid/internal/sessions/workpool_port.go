package sessions

import (
	"context"

	"github.com/google/uuid"
)

// WorkPoolPort is a "hexagonal" port exposing the single capability
// that the sessions domain needs from the work-pool domain.
type WorkPoolPort interface {
	// GetOrCreateDefault returns the ID of a pool named "default-{provider}",
	// creating it if necessary.
	GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error)
}
