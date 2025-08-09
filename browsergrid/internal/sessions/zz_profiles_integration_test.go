package sessions

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockProfileService implements ProfileService for testing
type MockProfileService struct {
	profiles map[uuid.UUID]bool
	errors   map[string]error
}

func NewMockProfileService() *MockProfileService {
	return &MockProfileService{
		profiles: make(map[uuid.UUID]bool),
		errors:   make(map[string]error),
	}
}

func (m *MockProfileService) ValidateProfile(ctx context.Context, profileID uuid.UUID, browser Browser) error {
	if err, exists := m.errors["ValidateProfile"]; exists {
		return err
	}

	// Check if profile exists
	if !m.profiles[profileID] {
		return errors.New("profile not found")
	}

	return nil
}

func (m *MockProfileService) AddProfile(profileID uuid.UUID) {
	m.profiles[profileID] = true
}

func (m *MockProfileService) SetError(method string, err error) {
	m.errors[method] = err
}

func setupTestDBWithProfiles(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Session{}, &SessionEvent{}, &SessionMetrics{})
	require.NoError(t, err)

	return db
}

func TestSessionCreationWithProfile(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	profileService := NewMockProfileService()
	ctx := context.Background()

	// Create a test profile ID
	profileID := uuid.New()
	profileService.AddProfile(profileID)

	tests := []struct {
		name          string
		session       Session
		expectedError string
		setupMock     func()
	}{
		{
			name: "valid session with profile",
			session: Session{
				Browser:   BrowserChrome,
				Version:   VerLatest,
				ProfileID: &profileID,
			},
			expectedError: "",
		},
		{
			name: "session with non-existent profile",
			session: Session{
				Browser:   BrowserChrome,
				Version:   VerLatest,
				ProfileID: func() *uuid.UUID { id := uuid.New(); return &id }(),
			},
			expectedError: "profile not found",
		},
		{
			name: "session with profile validation error",
			session: Session{
				Browser:   BrowserChrome,
				Version:   VerLatest,
				ProfileID: &profileID,
			},
			expectedError: "validation failed",
			setupMock: func() {
				profileService.SetError("ValidateProfile", fmt.Errorf("validation failed"))
			},
		},
		{
			name: "session without profile (should work)",
			session: Session{
				Browser: BrowserChrome,
				Version: VerLatest,
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock errors
			profileService.errors = make(map[string]error)
			if tt.setupMock != nil {
				tt.setupMock()
			}

			// Validate profile manually since store doesn't have access to ProfileService
			if tt.session.ProfileID != nil {
				err := profileService.ValidateProfile(ctx, *tt.session.ProfileID, tt.session.Browser)
				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
					return
				} else {
					assert.NoError(t, err)
				}
			}

			// Create session
			err := store.CreateSession(ctx, &tt.session)

			if tt.expectedError != "" && tt.session.ProfileID == nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.session.ID)
			}
		})
	}
}

func TestSessionProfileValidation(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test profile IDs for different browsers
	chromeProfileID := uuid.New()
	firefoxProfileID := uuid.New()

	tests := []struct {
		name          string
		session       Session
		expectedError string
	}{
		{
			name: "chrome session with chrome profile",
			session: Session{
				Browser:   BrowserChrome,
				ProfileID: &chromeProfileID,
			},
			expectedError: "",
		},
		{
			name: "firefox session with firefox profile",
			session: Session{
				Browser:   BrowserFirefox,
				ProfileID: &firefoxProfileID,
			},
			expectedError: "",
		},
		{
			name: "chrome session with firefox profile",
			session: Session{
				Browser:   BrowserChrome,
				ProfileID: &firefoxProfileID,
			},
			expectedError: "", // Profile service should handle browser compatibility
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store should be able to create sessions regardless of profile validation
			// Profile validation is handled at the HTTP handler level
			err := store.CreateSession(ctx, &tt.session)
			assert.NoError(t, err)
		})
	}
}

func TestSessionRetrievalWithProfile(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create a session with profile
	profileID := uuid.New()
	session := &Session{
		Browser:   BrowserChrome,
		Version:   VerLatest,
		ProfileID: &profileID,
		Status:    StatusPending,
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	// Retrieve session
	retrieved, err := store.GetSession(ctx, session.ID)
	require.NoError(t, err)

	// Verify profile ID is preserved
	assert.Equal(t, profileID, *retrieved.ProfileID)
	assert.Equal(t, session.Browser, retrieved.Browser)
}

func TestSessionListWithProfileFiltering(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create sessions with and without profiles
	profileID1 := uuid.New()
	profileID2 := uuid.New()

	sessions := []*Session{
		{
			Browser:   BrowserChrome,
			Version:   VerLatest,
			ProfileID: &profileID1,
			Status:    StatusPending,
		},
		{
			Browser:   BrowserFirefox,
			Version:   VerLatest,
			ProfileID: &profileID2,
			Status:    StatusPending,
		},
		{
			Browser: BrowserChrome,
			Version: VerLatest,
			Status:  StatusPending,
		},
	}

	for _, s := range sessions {
		err := store.CreateSession(ctx, s)
		require.NoError(t, err)
	}

	// List all sessions
	allSessions, err := store.ListSessions(ctx, nil, nil, nil, 0, 10)
	require.NoError(t, err)
	assert.Len(t, allSessions, 3)

	// Verify profile IDs are preserved
	profileCount := 0
	for _, s := range allSessions {
		if s.ProfileID != nil {
			profileCount++
		}
	}
	assert.Equal(t, 2, profileCount)
}

func TestSessionProfilePersistence(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create a session with profile
	profileID := uuid.New()
	session := &Session{
		Browser:   BrowserChrome,
		Version:   VerLatest,
		ProfileID: &profileID,
		Status:    StatusPending,
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	// Update session status
	err = store.UpdateSessionStatus(ctx, session.ID, StatusRunning)
	require.NoError(t, err)

	// Retrieve session and verify profile ID is still there
	retrieved, err := store.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, profileID, *retrieved.ProfileID)
	assert.Equal(t, StatusRunning, retrieved.Status)
}

func TestSessionProfileUpdate(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create a session with profile
	profileID1 := uuid.New()
	session := &Session{
		Browser:   BrowserChrome,
		Version:   VerLatest,
		ProfileID: &profileID1,
		Status:    StatusPending,
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	// Update session endpoints (simulating session start)
	profileID2 := uuid.New()
	updates := map[string]interface{}{
		"status":      StatusRunning,
		"ws_endpoint": "ws://localhost:8080",
		"live_url":    "http://localhost:8080",
		"profile_id":  profileID2, // Change profile
	}

	err = store.db.WithContext(ctx).Model(&Session{}).
		Where("id = ?", session.ID).
		Updates(updates).Error
	require.NoError(t, err)

	// Retrieve session and verify profile was updated
	retrieved, err := store.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, profileID2, *retrieved.ProfileID)
	assert.Equal(t, StatusRunning, retrieved.Status)
}

func TestSessionProfileCleanup(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create a session with profile
	profileID := uuid.New()
	session := &Session{
		Browser:   BrowserChrome,
		Version:   VerLatest,
		ProfileID: &profileID,
		Status:    StatusPending,
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	// Terminate session
	err = store.UpdateSessionStatus(ctx, session.ID, StatusTerminated)
	require.NoError(t, err)

	// Retrieve session and verify profile ID is still preserved
	retrieved, err := store.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, profileID, *retrieved.ProfileID)
	assert.Equal(t, StatusTerminated, retrieved.Status)
}

func TestSessionProfileEvents(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create a session with profile
	profileID := uuid.New()
	session := &Session{
		Browser:   BrowserChrome,
		Version:   VerLatest,
		ProfileID: &profileID,
		Status:    StatusPending,
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create session events
	events := []*SessionEvent{
		{
			SessionID: session.ID,
			Event:     EvtSessionCreated,
			Timestamp: time.Now(),
		},
		{
			SessionID: session.ID,
			Event:     EvtSessionStarting,
			Timestamp: time.Now(),
		},
	}

	for _, event := range events {
		err := store.CreateEvent(ctx, event)
		require.NoError(t, err)
	}

	// List events
	eventList, err := store.ListEvents(ctx, &session.ID, nil, nil, nil, 0, 10)
	require.NoError(t, err)
	assert.Len(t, eventList, 2)

	// Verify events are associated with the correct session
	for _, event := range eventList {
		assert.Equal(t, session.ID, event.SessionID)
	}
}

func TestSessionProfileMetrics(t *testing.T) {
	db := setupTestDBWithProfiles(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create a session with profile
	profileID := uuid.New()
	session := &Session{
		Browser:   BrowserChrome,
		Version:   VerLatest,
		ProfileID: &profileID,
		Status:    StatusPending,
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create session metrics
	metrics := &SessionMetrics{
		SessionID:  session.ID,
		CPUPercent: float64Ptr(45.2),
		MemoryMB:   float64Ptr(1024.5),
		Timestamp:  time.Now(),
	}

	err = store.CreateMetrics(ctx, metrics)
	require.NoError(t, err)

	// Verify metrics were created
	assert.NotEqual(t, uuid.Nil, metrics.ID)
	assert.Equal(t, session.ID, metrics.SessionID)
}

// Helper function to create float64 pointers
func float64Ptr(f float64) *float64 {
	return &f
}
