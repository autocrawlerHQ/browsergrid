package workpool

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

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
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

	err = db.Migrator().DropTable(&WorkPool{}, &sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&WorkPool{}, &sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestStore_CreateWorkPool(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	tests := []struct {
		name    string
		pool    *WorkPool
		wantErr bool
	}{
		{
			name: "create work pool with all fields",
			pool: &WorkPool{
				Name:            "test-pool",
				Description:     "A test work pool",
				Provider:        ProviderDocker,
				MinSize:         2,
				MaxConcurrency:  10,
				MaxIdleTime:     1800,
				AutoScale:       true,
				DefaultPriority: 5,
				QueueStrategy:   "fifo",
				DefaultEnv:      datatypes.JSON(`{"ENV": "test"}`),
			},
			wantErr: false,
		},
		{
			name: "create work pool with existing UUID",
			pool: &WorkPool{
				ID:             uuid.New(),
				Name:           "test-pool-2",
				Provider:       ProviderDocker,
				MaxConcurrency: 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			originalID := tt.pool.ID

			err := store.CreateWorkPool(ctx, tt.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateWorkPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if originalID == uuid.Nil && tt.pool.ID == uuid.Nil {
					t.Error("Expected UUID to be generated")
				}

				var found WorkPool
				err := db.First(&found, "id = ?", tt.pool.ID).Error
				if err != nil {
					t.Errorf("WorkPool not found in database: %v", err)
				}
			}
		})
	}
}

func TestStore_GetWorkPool(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name:    "get existing work pool",
			id:      testPool.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent work pool",
			id:      uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := store.GetWorkPool(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWorkPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if pool == nil {
					t.Error("Expected work pool to be returned")
				} else if pool.ID != tt.id {
					t.Errorf("Expected work pool ID %v, got %v", tt.id, pool.ID)
				}
			}
		})
	}
}

func TestStore_GetWorkPoolByName(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "unique-pool-name",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	tests := []struct {
		name     string
		poolName string
		wantErr  bool
	}{
		{
			name:     "get existing work pool by name",
			poolName: "unique-pool-name",
			wantErr:  false,
		},
		{
			name:     "get non-existent work pool by name",
			poolName: "non-existent-pool",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := store.GetWorkPoolByName(ctx, tt.poolName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWorkPoolByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if pool == nil {
					t.Error("Expected work pool to be returned")
				} else if pool.Name != tt.poolName {
					t.Errorf("Expected work pool name %v, got %v", tt.poolName, pool.Name)
				}
			}
		})
	}
}

func TestStore_ListWorkPools(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPools := []*WorkPool{
		{
			Name:           "active-pool",
			Provider:       ProviderDocker,
			MaxConcurrency: 10,
			Paused:         false,
		},
		{
			Name:           "paused-pool",
			Provider:       ProviderDocker,
			MaxConcurrency: 5,
			Paused:         true,
		},
	}

	for _, pool := range testPools {
		err := store.CreateWorkPool(ctx, pool)
		if err != nil {
			t.Fatalf("Failed to create test work pool: %v", err)
		}
	}

	tests := []struct {
		name        string
		paused      *bool
		expectedMin int
		expectedMax int
	}{
		{
			name:        "list all work pools",
			paused:      nil,
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name: "list active work pools",
			paused: func() *bool {
				p := false
				return &p
			}(),
			expectedMin: 1,
			expectedMax: 1,
		},
		{
			name: "list paused work pools",
			paused: func() *bool {
				p := true
				return &p
			}(),
			expectedMin: 1,
			expectedMax: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pools, err := store.ListWorkPools(ctx, tt.paused)
			if err != nil {
				t.Errorf("ListWorkPools() error = %v", err)
				return
			}

			if len(pools) < tt.expectedMin || len(pools) > tt.expectedMax {
				t.Errorf("Expected %d-%d work pools, got %d", tt.expectedMin, tt.expectedMax, len(pools))
			}

			if tt.paused != nil {
				for _, pool := range pools {
					if pool.Paused != *tt.paused {
						t.Errorf("Expected paused %v, got %v", *tt.paused, pool.Paused)
					}
				}
			}
		})
	}
}

func TestStore_UpdateWorkPool(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
		Paused:         false,
	}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	updates := map[string]interface{}{
		"max_concurrency": 20,
		"paused":          true,
	}

	err = store.UpdateWorkPool(ctx, testPool.ID, updates)
	if err != nil {
		t.Fatalf("UpdateWorkPool() error = %v", err)
	}

	updatedPool, err := store.GetWorkPool(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("Failed to get updated work pool: %v", err)
	}

	if updatedPool.MaxConcurrency != 20 {
		t.Errorf("Expected max_concurrency 20, got %d", updatedPool.MaxConcurrency)
	}
	if !updatedPool.Paused {
		t.Errorf("Expected paused true, got %v", updatedPool.Paused)
	}
}

func TestStore_DrainWorkPool(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
		MinSize:        3,
		AutoScale:      true,
		Paused:         false,
	}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	err = store.DrainWorkPool(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("DrainWorkPool() error = %v", err)
	}

	drainedPool, err := store.GetWorkPool(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("Failed to get drained work pool: %v", err)
	}

	if !drainedPool.Paused {
		t.Error("Expected pool to be paused after drain")
	}
	if drainedPool.AutoScale {
		t.Error("Expected auto_scale to be disabled after drain")
	}
	if drainedPool.MinSize != 0 {
		t.Errorf("Expected min_size 0 after drain, got %d", drainedPool.MinSize)
	}
}

func TestStore_DeleteWorkPool(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	err = store.DeleteWorkPool(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("DeleteWorkPool() error = %v", err)
	}

	_, err = store.GetWorkPool(ctx, testPool.ID)
	if err == nil {
		t.Error("Expected work pool to be deleted")
	}
}

func TestStore_GetPoolCapacity(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	testSessions := []*sessions.Session{
		{
			Browser: sessions.BrowserChrome, Version: sessions.VerLatest, OperatingSystem: sessions.OSLinux,
			Screen: sessions.ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status: sessions.StatusStarting, WorkPoolID: &testPool.ID,
		},
		{
			Browser: sessions.BrowserChrome, Version: sessions.VerLatest, OperatingSystem: sessions.OSLinux,
			Screen: sessions.ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status: sessions.StatusRunning, WorkPoolID: &testPool.ID,
		},
		{
			Browser: sessions.BrowserChrome, Version: sessions.VerLatest, OperatingSystem: sessions.OSLinux,
			Screen: sessions.ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status: sessions.StatusCompleted, WorkPoolID: &testPool.ID,
		},
	}

	for _, session := range testSessions {
		err := db.Create(session).Error
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}
	}

	maxCapacity, activeCount, err := store.GetPoolCapacity(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("GetPoolCapacity() error = %v", err)
	}

	if maxCapacity != 10 {
		t.Errorf("Expected max capacity 10, got %d", maxCapacity)
	}
	if activeCount != 2 {
		t.Errorf("Expected active count 2, got %d", activeCount)
	}
}

func TestStore_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)

	store := NewStore(db)
	err := store.AutoMigrate()
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	tables := []string{"work_pools"}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("Expected table %s to exist", table)
		}
	}
}

func TestWorkPool_Methods(t *testing.T) {
	tests := []struct {
		name     string
		pool     *WorkPool
		testFunc func(*testing.T, *WorkPool)
	}{
		{
			name: "NeedsScaling returns true when below minimum",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: true,
				Paused:    false,
			},
			testFunc: func(t *testing.T, pool *WorkPool) {
				if !pool.NeedsScaling(2, 1) {
					t.Error("Expected pool to need scaling")
				}
			},
		},
		{
			name: "NeedsScaling returns false when paused",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: true,
				Paused:    true,
			},
			testFunc: func(t *testing.T, pool *WorkPool) {
				if pool.NeedsScaling(2, 1) {
					t.Error("Expected paused pool to not need scaling")
				}
			},
		},
		{
			name: "NeedsScaling returns false when autoscale disabled",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: false,
				Paused:    false,
			},
			testFunc: func(t *testing.T, pool *WorkPool) {
				if pool.NeedsScaling(2, 1) {
					t.Error("Expected pool with autoscale disabled to not need scaling")
				}
			},
		},
		{
			name: "CanAcceptMore returns true when under capacity",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         false,
			},
			testFunc: func(t *testing.T, pool *WorkPool) {
				if !pool.CanAcceptMore(5) {
					t.Error("Expected pool to accept more sessions")
				}
			},
		},
		{
			name: "CanAcceptMore returns false when paused",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         true,
			},
			testFunc: func(t *testing.T, pool *WorkPool) {
				if pool.CanAcceptMore(5) {
					t.Error("Expected paused pool to not accept more sessions")
				}
			},
		},
		{
			name: "SessionsToCreate calculates correct amount",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         false,
			},
			testFunc: func(t *testing.T, pool *WorkPool) {
				result := pool.SessionsToCreate(3, 2)
				expected := 5
				if result != expected {
					t.Errorf("Expected %d sessions to create, got %d", expected, result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t, tt.pool)
		})
	}
}
