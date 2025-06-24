package sessions

import (
	"context"

	"github.com/google/uuid"
)

// PoolService defines the interface for work pool operations that sessions need.
// This follows the dependency inversion principle - the high-level sessions domain
// depends on this abstraction rather than concrete implementations.
type PoolService interface {
	// GetOrCreateDefault returns the ID of a pool named "default-{provider}",
	// creating it if necessary.
	GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error)
}
