package sessions

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/datatypes"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	testContainer *postgres.PostgresContainer
	testConnStr   string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	testContainer, err = postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	testConnStr, err = testContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to get connection string: %v", err)
	}

	code := m.Run()

	if err := testContainer.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate PostgreSQL container: %v", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(postgresDriver.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.Migrator().DropTable(&Session{}, &SessionEvent{}, &SessionMetrics{}, &Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&Session{}, &SessionEvent{}, &SessionMetrics{}, &Pool{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestStore_CreateSession(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name: "create session with all fields",
			session: &Session{
				Browser:         BrowserChrome,
				Version:         VerLatest,
				OperatingSystem: OSLinux,
				Screen: ScreenConfig{
					Width:  1920,
					Height: 1080,
					DPI:    96,
					Scale:  1.0,
				},
				Status: StatusPending,
			},
			wantErr: false,
		},
		{
			name: "create session with existing UUID",
			session: &Session{
				ID:              uuid.New(),
				Browser:         BrowserFirefox,
				Version:         VerStable,
				OperatingSystem: OSWindows,
				Screen: ScreenConfig{
					Width:  1280,
					Height: 720,
					DPI:    96,
					Scale:  1.0,
				},
				Status: StatusPending,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			originalID := tt.session.ID

			err := store.CreateSession(ctx, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if originalID == uuid.Nil && tt.session.ID == uuid.Nil {
					t.Error("Expected UUID to be generated")
				}

				var found Session
				err := db.First(&found, "id = ?", tt.session.ID).Error
				if err != nil {
					t.Errorf("Session not found in database: %v", err)
				}
			}
		})
	}
}

func TestStore_GetSession(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testSession := &Session{
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status:          StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name:    "get existing session",
			id:      testSession.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent session",
			id:      uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := store.GetSession(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if session == nil {
					t.Error("Expected session to be returned")
				} else if session.ID != tt.id {
					t.Errorf("Expected session ID %v, got %v", tt.id, session.ID)
				}
			}
		})
	}
}

func TestStore_UpdateSessionStatus(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testSession := &Session{
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status:          StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	tests := []struct {
		name      string
		id        uuid.UUID
		newStatus SessionStatus
		wantErr   bool
	}{
		{
			name:      "update existing session status",
			id:        testSession.ID,
			newStatus: StatusRunning,
			wantErr:   false,
		},
		{
			name:      "update non-existent session",
			id:        uuid.New(),
			newStatus: StatusRunning,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.UpdateSessionStatus(ctx, tt.id, tt.newStatus)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSessionStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.id == testSession.ID {
				session, err := store.GetSession(ctx, tt.id)
				if err != nil {
					t.Errorf("Failed to get updated session: %v", err)
				} else if session.Status != tt.newStatus {
					t.Errorf("Expected status %v, got %v", tt.newStatus, session.Status)
				}
			}
		})
	}
}

func TestStore_ListSessions(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	now := time.Now()
	testSessions := []*Session{
		{
			Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
			Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status: StatusPending,
		},
		{
			Browser: BrowserFirefox, Version: VerStable, OperatingSystem: OSWindows,
			Screen: ScreenConfig{Width: 1280, Height: 720, DPI: 96, Scale: 1.0},
			Status: StatusRunning,
		},
		{
			Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSMacOS,
			Screen: ScreenConfig{Width: 1440, Height: 900, DPI: 96, Scale: 1.0},
			Status: StatusCompleted,
		},
	}

	for _, session := range testSessions {
		err := store.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}
	}
	tests := []struct {
		name        string
		status      *SessionStatus
		start       *time.Time
		end         *time.Time
		offset      int
		limit       int
		expectedMin int
		expectedMax int
	}{
		{
			name:        "list all sessions",
			status:      nil,
			start:       nil,
			end:         nil,
			offset:      0,
			limit:       100,
			expectedMin: 3,
			expectedMax: 3,
		},
		{
			name: "filter by status",
			status: func() *SessionStatus {
				s := StatusPending
				return &s
			}(),
			start:       nil,
			end:         nil,
			offset:      0,
			limit:       100,
			expectedMin: 1,
			expectedMax: 1,
		},
		{
			name:        "with pagination",
			status:      nil,
			start:       nil,
			end:         nil,
			offset:      1,
			limit:       2,
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "filter by time range",
			status:      nil,
			start:       &now,
			end:         nil,
			offset:      0,
			limit:       100,
			expectedMin: 0,
			expectedMax: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions, err := store.ListSessions(ctx, tt.status, tt.start, tt.end, tt.offset, tt.limit)
			if err != nil {
				t.Errorf("ListSessions() error = %v", err)
				return
			}

			if len(sessions) < tt.expectedMin || len(sessions) > tt.expectedMax {
				t.Errorf("Expected %d-%d sessions, got %d", tt.expectedMin, tt.expectedMax, len(sessions))
			}

			if tt.status != nil {
				for _, session := range sessions {
					if session.Status != *tt.status {
						t.Errorf("Expected status %v, got %v", *tt.status, session.Status)
					}
				}
			}
		})
	}
}

func TestStore_PoolOperations(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &Pool{
		ID:              "test-pool",
		Name:            "Test Pool",
		Description:     "A test pool",
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		MinSize:         2,
		MaxSize:         10,
		Enabled:         true,
	}

	err := store.CreatePool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test pool: %v", err)
	}

	pool, err := store.GetPool(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("Failed to get pool: %v", err)
	}
	if pool.ID != testPool.ID {
		t.Errorf("Expected pool ID %v, got %v", testPool.ID, pool.ID)
	}

	pools, err := store.ListPools(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list pools: %v", err)
	}
	if len(pools) != 1 {
		t.Errorf("Expected 1 pool, got %d", len(pools))
	}

	enabled := true
	enabledPools, err := store.ListPools(ctx, &enabled)
	if err != nil {
		t.Fatalf("Failed to list enabled pools: %v", err)
	}
	if len(enabledPools) != 1 {
		t.Errorf("Expected 1 enabled pool, got %d", len(enabledPools))
	}

	err = store.UpdatePoolSizes(ctx, testPool.ID, 5, 3)
	if err != nil {
		t.Fatalf("Failed to update pool sizes: %v", err)
	}

	updatedPool, err := store.GetPool(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("Failed to get updated pool: %v", err)
	}
	if updatedPool.CurrentSize != 5 {
		t.Errorf("Expected current size 5, got %d", updatedPool.CurrentSize)
	}
	if updatedPool.AvailableSize != 3 {
		t.Errorf("Expected available size 3, got %d", updatedPool.AvailableSize)
	}
}

func TestStore_PooledSessionOperations(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &Pool{
		ID:              "test-pool",
		Name:            "Test Pool",
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		MinSize:         2,
		MaxSize:         10,
		Enabled:         true,
	}
	err := store.CreatePool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test pool: %v", err)
	}

	testSession := &Session{
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status:          StatusPending,
	}

	err = store.CreatePooledSession(ctx, testPool.ID, testSession)
	if err != nil {
		t.Fatalf("Failed to create pooled session: %v", err)
	}

	if testSession.PoolID == nil || *testSession.PoolID != testPool.ID {
		t.Error("Expected session to be associated with pool")
	}
	if !testSession.IsPooled {
		t.Error("Expected session to be marked as pooled")
	}

	err = store.MarkSessionAvailable(ctx, testSession.ID)
	if err != nil {
		t.Fatalf("Failed to mark session as available: %v", err)
	}

	availableSessions, err := store.GetAvailableSessions(ctx, testPool.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get available sessions: %v", err)
	}
	if len(availableSessions) != 1 {
		t.Errorf("Expected 1 available session, got %d", len(availableSessions))
	}

	claimedBy := "user123"
	err = store.ClaimSession(ctx, testSession.ID, claimedBy)
	if err != nil {
		t.Fatalf("Failed to claim session: %v", err)
	}

	claimedSession, err := store.GetSession(ctx, testSession.ID)
	if err != nil {
		t.Fatalf("Failed to get claimed session: %v", err)
	}
	if claimedSession.Status != StatusClaimed {
		t.Errorf("Expected status %v, got %v", StatusClaimed, claimedSession.Status)
	}
	if claimedSession.ClaimedBy == nil || *claimedSession.ClaimedBy != claimedBy {
		t.Errorf("Expected claimed_by %v, got %v", claimedBy, claimedSession.ClaimedBy)
	}

	err = store.ReleaseSession(ctx, testSession.ID)
	if err != nil {
		t.Fatalf("Failed to release session: %v", err)
	}

	releasedSession, err := store.GetSession(ctx, testSession.ID)
	if err != nil {
		t.Fatalf("Failed to get released session: %v", err)
	}
	if releasedSession.Status != StatusAvailable {
		t.Errorf("Expected status %v, got %v", StatusAvailable, releasedSession.Status)
	}
	if releasedSession.ClaimedBy != nil {
		t.Errorf("Expected claimed_by to be nil, got %v", releasedSession.ClaimedBy)
	}
}

func TestStore_ClaimSession_EdgeCases(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &Pool{
		ID:              "test-pool",
		Name:            "Test Pool",
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		MinSize:         2,
		MaxSize:         10,
		CurrentSize:     1,
		AvailableSize:   1,
		Enabled:         true,
	}
	err := store.CreatePool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test pool: %v", err)
	}

	tests := []struct {
		name      string
		setupFunc func() uuid.UUID
		claimedBy string
		wantErr   bool
	}{
		{
			name: "claim non-existent session",
			setupFunc: func() uuid.UUID {
				return uuid.New()
			},
			claimedBy: "user123",
			wantErr:   true,
		},
		{
			name: "claim already claimed session",
			setupFunc: func() uuid.UUID {
				session := &Session{
					Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
					Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
				}
				store.CreatePooledSession(ctx, testPool.ID, session)
				store.MarkSessionAvailable(ctx, session.ID)
				store.ClaimSession(ctx, session.ID, "user456")
				return session.ID
			},
			claimedBy: "user123",
			wantErr:   true,
		},
		{
			name: "claim non-available session",
			setupFunc: func() uuid.UUID {
				session := &Session{
					Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
					Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
				}
				store.CreatePooledSession(ctx, testPool.ID, session)
				return session.ID
			},
			claimedBy: "user123",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := tt.setupFunc()
			err := store.ClaimSession(ctx, sessionID, tt.claimedBy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClaimSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStore_EventOperations(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testSession := &Session{
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status:          StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	testEvent := &SessionEvent{
		SessionID: testSession.ID,
		Event:     EvtSessionStarting,
		Data:      datatypes.JSON(`{"test": "data"}`),
	}

	err = store.CreateEvent(ctx, testEvent)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	if testEvent.ID == uuid.Nil {
		t.Error("Expected event ID to be generated")
	}

	events, err := store.ListEvents(ctx, nil, nil, nil, nil, 0, 100)
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	eventType := EvtSessionStarting
	filteredEvents, err := store.ListEvents(ctx, &testSession.ID, &eventType, nil, nil, 0, 100)
	if err != nil {
		t.Fatalf("Failed to list filtered events: %v", err)
	}
	if len(filteredEvents) != 1 {
		t.Errorf("Expected 1 filtered event, got %d", len(filteredEvents))
	}
}

func TestStore_MetricsOperations(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testSession := &Session{
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status:          StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	cpuPercent := 45.5
	memoryMB := 512.0
	testMetrics := &SessionMetrics{
		SessionID:  testSession.ID,
		CPUPercent: &cpuPercent,
		MemoryMB:   &memoryMB,
	}

	err = store.CreateMetrics(ctx, testMetrics)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	if testMetrics.ID == uuid.Nil {
		t.Error("Expected metrics ID to be generated")
	}

	var savedMetrics SessionMetrics
	err = db.First(&savedMetrics, "session_id = ?", testSession.ID).Error
	if err != nil {
		t.Fatalf("Failed to find saved metrics: %v", err)
	}

	if savedMetrics.CPUPercent == nil || *savedMetrics.CPUPercent != cpuPercent {
		t.Errorf("Expected CPU percent %v, got %v", cpuPercent, savedMetrics.CPUPercent)
	}
	if savedMetrics.MemoryMB == nil || *savedMetrics.MemoryMB != memoryMB {
		t.Errorf("Expected memory MB %v, got %v", memoryMB, savedMetrics.MemoryMB)
	}
}

func TestStore_CleanupExpiredSessions(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &Pool{
		ID:              "test-pool",
		Name:            "Test Pool",
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		MinSize:         2,
		MaxSize:         10,
		Enabled:         true,
	}
	err := store.CreatePool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test pool: %v", err)
	}

	oldSession := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
	}
	err = store.CreatePooledSession(ctx, testPool.ID, oldSession)
	if err != nil {
		t.Fatalf("Failed to create old session: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	err = db.Model(&Session{}).
		Where("id = ?", oldSession.ID).
		Updates(map[string]interface{}{
			"status":       StatusAvailable,
			"available_at": oldTime,
		}).Error
	if err != nil {
		t.Fatalf("Failed to update session timestamp: %v", err)
	}

	recentSession := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
	}
	err = store.CreatePooledSession(ctx, testPool.ID, recentSession)
	if err != nil {
		t.Fatalf("Failed to create recent session: %v", err)
	}
	err = store.MarkSessionAvailable(ctx, recentSession.ID)
	if err != nil {
		t.Fatalf("Failed to mark recent session as available: %v", err)
	}

	cleanedCount, err := store.CleanupExpiredSessions(ctx, testPool.ID, 3600)
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}

	if cleanedCount != 1 {
		t.Errorf("Expected 1 cleaned session, got %d", cleanedCount)
	}

	oldSessionUpdated, err := store.GetSession(ctx, oldSession.ID)
	if err != nil {
		t.Fatalf("Failed to get old session: %v", err)
	}
	if oldSessionUpdated.Status != StatusExpired {
		t.Errorf("Expected old session to be expired, got %v", oldSessionUpdated.Status)
	}

	recentSessionUpdated, err := store.GetSession(ctx, recentSession.ID)
	if err != nil {
		t.Fatalf("Failed to get recent session: %v", err)
	}
	if recentSessionUpdated.Status != StatusAvailable {
		t.Errorf("Expected recent session to be available, got %v", recentSessionUpdated.Status)
	}
}

func TestStore_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)

	store := NewStore(db)
	err := store.AutoMigrate()
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	tables := []string{"sessions", "session_events", "session_metrics", "session_pools"}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("Expected table %s to exist", table)
		}
	}
}
