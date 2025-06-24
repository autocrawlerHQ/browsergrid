package sessions

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test implementations of PoolService for testing

// successPoolService always returns a successful pool ID
type successPoolService struct {
	poolID uuid.UUID
}

func (s *successPoolService) GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error) {
	if s.poolID == uuid.Nil {
		s.poolID = uuid.New()
	}
	return s.poolID, nil
}

// errorPoolService always returns an error
type errorPoolService struct {
	err error
}

func (e *errorPoolService) GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error) {
	if e.err == nil {
		e.err = errors.New("pool service error")
	}
	return uuid.Nil, e.err
}

// trackingPoolService tracks calls made to it
type trackingPoolService struct {
	calls []struct {
		ctx      context.Context
		provider string
	}
	poolID uuid.UUID
}

func (t *trackingPoolService) GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error) {
	t.calls = append(t.calls, struct {
		ctx      context.Context
		provider string
	}{ctx, provider})

	if t.poolID == uuid.Nil {
		t.poolID = uuid.New()
	}
	return t.poolID, nil
}

func TestAssignToDefaultWorkPool_Success(t *testing.T) {
	ctx := context.Background()
	poolService := &successPoolService{}

	session := &Session{
		Provider: "docker",
	}

	err := assignToDefaultWorkPool(ctx, poolService, session)
	require.NoError(t, err)

	assert.NotNil(t, session.WorkPoolID)
	assert.Equal(t, poolService.poolID, *session.WorkPoolID)
}

func TestAssignToDefaultWorkPool_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("database connection failed")
	poolService := &errorPoolService{err: expectedErr}

	session := &Session{
		Provider: "docker",
	}

	err := assignToDefaultWorkPool(ctx, poolService, session)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, session.WorkPoolID, "WorkPoolID should remain nil on error")
}

func TestAssignToDefaultWorkPool_ProviderPassing(t *testing.T) {
	ctx := context.Background()
	poolService := &trackingPoolService{}

	tests := []struct {
		name     string
		provider string
	}{
		{"docker provider", "docker"},
		{"local provider", "local"},
		{"azure_aci provider", "azure_aci"},
		{"empty provider", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Provider: tt.provider,
			}

			err := assignToDefaultWorkPool(ctx, poolService, session)
			require.NoError(t, err)

			// Find the call for this provider
			var foundCall bool
			for _, call := range poolService.calls {
				if call.provider == tt.provider {
					foundCall = true
					assert.Equal(t, ctx, call.ctx, "Context should be passed through")
					break
				}
			}
			assert.True(t, foundCall, "Provider should be passed to pool service")
		})
	}
}

func TestAssignToDefaultWorkPool_ContextPropagation(t *testing.T) {
	// Create a context with a value to test propagation
	type contextKey string
	key := contextKey("test-key")
	value := "test-value"
	ctx := context.WithValue(context.Background(), key, value)

	poolService := &trackingPoolService{}
	session := &Session{Provider: "docker"}

	err := assignToDefaultWorkPool(ctx, poolService, session)
	require.NoError(t, err)

	// Verify context was propagated
	require.Len(t, poolService.calls, 1)
	receivedValue := poolService.calls[0].ctx.Value(key)
	assert.Equal(t, value, receivedValue, "Context should be propagated to pool service")
}

func TestAssignToDefaultWorkPool_AlreadyAssigned(t *testing.T) {
	ctx := context.Background()
	poolService := &trackingPoolService{}

	existingPoolID := uuid.New()
	session := &Session{
		Provider:   "docker",
		WorkPoolID: &existingPoolID,
	}

	err := assignToDefaultWorkPool(ctx, poolService, session)
	require.NoError(t, err)

	// Pool service should still be called and should update the WorkPoolID
	assert.Len(t, poolService.calls, 1)
	assert.Equal(t, poolService.poolID, *session.WorkPoolID)
	assert.NotEqual(t, existingPoolID, *session.WorkPoolID, "WorkPoolID should be updated")
}

func TestPoolServiceInterface_Compliance(t *testing.T) {
	// Test that our test implementations comply with the interface
	var _ PoolService = &successPoolService{}
	var _ PoolService = &errorPoolService{}
	var _ PoolService = &trackingPoolService{}
	var _ PoolService = &mockPoolService{}
}

// Benchmark the assign function
func BenchmarkAssignToDefaultWorkPool(b *testing.B) {
	ctx := context.Background()
	poolService := &successPoolService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session := &Session{Provider: "docker"}
		err := assignToDefaultWorkPool(ctx, poolService, session)
		if err != nil {
			b.Fatal(err)
		}
	}
}
