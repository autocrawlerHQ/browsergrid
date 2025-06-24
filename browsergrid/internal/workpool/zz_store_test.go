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

	err = db.Migrator().DropTable(&WorkPool{}, &Worker{}, &sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&WorkPool{}, &Worker{}, &sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{})
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
				Provider:       ProviderLocal,
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
			Provider:       ProviderLocal,
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

func TestStore_RegisterWorker(t *testing.T) {
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
		worker  *Worker
		wantErr bool
	}{
		{
			name: "register new worker",
			worker: &Worker{
				PoolID:   testPool.ID,
				Name:     "test-worker",
				Hostname: "localhost",
				Provider: ProviderDocker,
				MaxSlots: 5,
			},
			wantErr: false,
		},
		{
			name: "register worker with existing UUID",
			worker: &Worker{
				ID:       uuid.New(),
				PoolID:   testPool.ID,
				Name:     "test-worker-2",
				Provider: ProviderLocal,
				MaxSlots: 3,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalID := tt.worker.ID

			err := store.RegisterWorker(ctx, tt.worker)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterWorker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if originalID == uuid.Nil && tt.worker.ID == uuid.Nil {
					t.Error("Expected UUID to be generated")
				}

				var found Worker
				err := db.First(&found, "id = ?", tt.worker.ID).Error
				if err != nil {
					t.Errorf("Worker not found in database: %v", err)
				}

				if tt.worker.LastBeat.IsZero() {
					t.Error("Expected LastBeat to be set")
				}
				if tt.worker.StartedAt.IsZero() {
					t.Error("Expected StartedAt to be set")
				}
			}
		})
	}
}

func TestStore_GetWorker(t *testing.T) {
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

	testWorker := &Worker{
		PoolID:   testPool.ID,
		Name:     "test-worker",
		Provider: ProviderDocker,
		MaxSlots: 5,
	}
	err = store.RegisterWorker(ctx, testWorker)
	if err != nil {
		t.Fatalf("Failed to register test worker: %v", err)
	}

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name:    "get existing worker",
			id:      testWorker.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent worker",
			id:      uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker, err := store.GetWorker(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWorker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if worker == nil {
					t.Error("Expected worker to be returned")
				} else if worker.ID != tt.id {
					t.Errorf("Expected worker ID %v, got %v", tt.id, worker.ID)
				}
				if worker.Pool.ID != testPool.ID {
					t.Errorf("Expected pool ID %v, got %v", testPool.ID, worker.Pool.ID)
				}
			}
		})
	}
}

func TestStore_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)

	store := NewStore(db)
	err := store.AutoMigrate()
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	tables := []string{"work_pools", "workers"}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("Expected table %s to exist", table)
		}
	}
}

func TestStore_ListWorkers(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	pool1 := &WorkPool{Name: "pool-1", Provider: ProviderDocker, MaxConcurrency: 10}
	pool2 := &WorkPool{Name: "pool-2", Provider: ProviderLocal, MaxConcurrency: 5}

	err := store.CreateWorkPool(ctx, pool1)
	if err != nil {
		t.Fatalf("Failed to create pool1: %v", err)
	}
	err = store.CreateWorkPool(ctx, pool2)
	if err != nil {
		t.Fatalf("Failed to create pool2: %v", err)
	}

	workers := []*Worker{
		{PoolID: pool1.ID, Name: "worker-1-1", Provider: ProviderDocker, MaxSlots: 3},
		{PoolID: pool1.ID, Name: "worker-1-2", Provider: ProviderDocker, MaxSlots: 2},
		{PoolID: pool2.ID, Name: "worker-2-1", Provider: ProviderLocal, MaxSlots: 1},
	}

	for _, worker := range workers {
		err := store.RegisterWorker(ctx, worker)
		if err != nil {
			t.Fatalf("Failed to register worker: %v", err)
		}
	}

	tests := []struct {
		name        string
		poolID      *uuid.UUID
		online      *bool
		expectedMin int
		expectedMax int
	}{
		{
			name:        "list all workers",
			poolID:      nil,
			online:      nil,
			expectedMin: 3,
			expectedMax: 3,
		},
		{
			name:        "list workers for pool1",
			poolID:      &pool1.ID,
			online:      nil,
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "list workers for pool2",
			poolID:      &pool2.ID,
			online:      nil,
			expectedMin: 1,
			expectedMax: 1,
		},
		{
			name:   "list online workers",
			poolID: nil,
			online: func() *bool {
				o := true
				return &o
			}(),
			expectedMin: 3,
			expectedMax: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workers, err := store.ListWorkers(ctx, tt.poolID, tt.online)
			if err != nil {
				t.Errorf("ListWorkers() error = %v", err)
				return
			}

			if len(workers) < tt.expectedMin || len(workers) > tt.expectedMax {
				t.Errorf("Expected %d-%d workers, got %d", tt.expectedMin, tt.expectedMax, len(workers))
			}

			if tt.poolID != nil {
				for _, worker := range workers {
					if worker.PoolID != *tt.poolID {
						t.Errorf("Expected pool ID %v, got %v", *tt.poolID, worker.PoolID)
					}
				}
			}
		})
	}
}

func TestStore_Heartbeat(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{Name: "test-pool", Provider: ProviderDocker, MaxConcurrency: 10}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	testWorker := &Worker{
		PoolID:   testPool.ID,
		Name:     "test-worker",
		Provider: ProviderDocker,
		MaxSlots: 5,
		Active:   2,
	}
	err = store.RegisterWorker(ctx, testWorker)
	if err != nil {
		t.Fatalf("Failed to register test worker: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	originalLastBeat := testWorker.LastBeat

	newActive := 3
	err = store.Heartbeat(ctx, testWorker.ID, newActive)
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}

	updatedWorker, err := store.GetWorker(ctx, testWorker.ID)
	if err != nil {
		t.Fatalf("Failed to get updated worker: %v", err)
	}

	if updatedWorker.Active != newActive {
		t.Errorf("Expected active %d, got %d", newActive, updatedWorker.Active)
	}
	if !updatedWorker.LastBeat.After(originalLastBeat) {
		t.Error("Expected LastBeat to be updated")
	}
}

func TestStore_UpdateWorkerActive(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{Name: "test-pool", Provider: ProviderDocker, MaxConcurrency: 10}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	testWorker := &Worker{
		PoolID:   testPool.ID,
		Name:     "test-worker",
		Provider: ProviderDocker,
		MaxSlots: 5,
		Active:   2,
	}
	err = store.RegisterWorker(ctx, testWorker)
	if err != nil {
		t.Fatalf("Failed to register test worker: %v", err)
	}

	tests := []struct {
		name     string
		delta    int
		expected int
	}{
		{
			name:     "increment active count",
			delta:    2,
			expected: 4,
		},
		{
			name:     "decrement active count",
			delta:    -1,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.UpdateWorkerActive(ctx, testWorker.ID, tt.delta)
			if err != nil {
				t.Errorf("UpdateWorkerActive() error = %v", err)
				return
			}

			updatedWorker, err := store.GetWorker(ctx, testWorker.ID)
			if err != nil {
				t.Fatalf("Failed to get updated worker: %v", err)
			}

			if updatedWorker.Active != tt.expected {
				t.Errorf("Expected active %d, got %d", tt.expected, updatedWorker.Active)
			}
		})
	}
}

func TestStore_PauseWorker(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{Name: "test-pool", Provider: ProviderDocker, MaxConcurrency: 10}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	testWorker := &Worker{
		PoolID:   testPool.ID,
		Name:     "test-worker",
		Provider: ProviderDocker,
		MaxSlots: 5,
		Paused:   false,
	}
	err = store.RegisterWorker(ctx, testWorker)
	if err != nil {
		t.Fatalf("Failed to register test worker: %v", err)
	}

	err = store.PauseWorker(ctx, testWorker.ID, true)
	if err != nil {
		t.Fatalf("PauseWorker() error = %v", err)
	}

	pausedWorker, err := store.GetWorker(ctx, testWorker.ID)
	if err != nil {
		t.Fatalf("Failed to get paused worker: %v", err)
	}

	if !pausedWorker.Paused {
		t.Error("Expected worker to be paused")
	}

	err = store.PauseWorker(ctx, testWorker.ID, false)
	if err != nil {
		t.Fatalf("PauseWorker() error = %v", err)
	}

	unpausedWorker, err := store.GetWorker(ctx, testWorker.ID)
	if err != nil {
		t.Fatalf("Failed to get unpaused worker: %v", err)
	}

	if unpausedWorker.Paused {
		t.Error("Expected worker to be unpaused")
	}
}

func TestStore_DeleteWorker(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{Name: "test-pool", Provider: ProviderDocker, MaxConcurrency: 10}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	testWorker := &Worker{
		PoolID:   testPool.ID,
		Name:     "test-worker",
		Provider: ProviderDocker,
		MaxSlots: 5,
	}
	err = store.RegisterWorker(ctx, testWorker)
	if err != nil {
		t.Fatalf("Failed to register test worker: %v", err)
	}

	err = store.DeleteWorker(ctx, testWorker.ID)
	if err != nil {
		t.Fatalf("DeleteWorker() error = %v", err)
	}

	_, err = store.GetWorker(ctx, testWorker.ID)
	if err == nil {
		t.Error("Expected worker to be deleted")
	}
}

func TestStore_DequeueSessions(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{Name: "test-pool", Provider: ProviderDocker, MaxConcurrency: 10}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	testWorker := &Worker{
		PoolID:   testPool.ID,
		Name:     "test-worker",
		Provider: ProviderDocker,
		MaxSlots: 5,
	}
	err = store.RegisterWorker(ctx, testWorker)
	if err != nil {
		t.Fatalf("Failed to register test worker: %v", err)
	}

	testSessions := []*sessions.Session{
		{
			Browser:         sessions.BrowserChrome,
			Version:         sessions.VerLatest,
			OperatingSystem: sessions.OSLinux,
			Screen:          sessions.ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status:          sessions.StatusPending,
			WorkPoolID:      &testPool.ID,
		},
		{
			Browser:         sessions.BrowserFirefox,
			Version:         sessions.VerStable,
			OperatingSystem: sessions.OSLinux,
			Screen:          sessions.ScreenConfig{Width: 1280, Height: 720, DPI: 96, Scale: 1.0},
			Status:          sessions.StatusPending,
			WorkPoolID:      &testPool.ID,
		},
		{
			Browser:         sessions.BrowserChrome,
			Version:         sessions.VerLatest,
			OperatingSystem: sessions.OSLinux,
			Screen:          sessions.ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status:          sessions.StatusRunning,
			WorkPoolID:      &testPool.ID,
		},
	}

	for _, session := range testSessions {
		err := db.Create(session).Error
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}
	}

	tests := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{
			name:          "dequeue with limit 1",
			limit:         1,
			expectedCount: 1,
		},
		{
			name:          "dequeue with limit 5",
			limit:         5,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dequeuedSessions, err := store.DequeueSessions(ctx, testPool.ID, testWorker.ID, tt.limit)
			if err != nil {
				t.Errorf("DequeueSessions() error = %v", err)
				return
			}

			if len(dequeuedSessions) != tt.expectedCount {
				t.Errorf("Expected %d sessions, got %d", tt.expectedCount, len(dequeuedSessions))
			}

			for _, session := range dequeuedSessions {
				if session.WorkerID == nil || *session.WorkerID != testWorker.ID {
					t.Errorf("Expected session to be assigned to worker %v", testWorker.ID)
				}
				if session.Status != sessions.StatusStarting {
					t.Errorf("Expected session status %v, got %v", sessions.StatusStarting, session.Status)
				}
			}
		})
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

func TestStore_GetWorkerCapacity(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	testPool := &WorkPool{Name: "test-pool", Provider: ProviderDocker, MaxConcurrency: 10}
	err := store.CreateWorkPool(ctx, testPool)
	if err != nil {
		t.Fatalf("Failed to create test work pool: %v", err)
	}

	workers := []*Worker{
		{PoolID: testPool.ID, Name: "worker-1", Provider: ProviderDocker, MaxSlots: 5, Active: 2, Paused: false},
		{PoolID: testPool.ID, Name: "worker-2", Provider: ProviderDocker, MaxSlots: 3, Active: 1, Paused: false},
		{PoolID: testPool.ID, Name: "worker-3", Provider: ProviderDocker, MaxSlots: 2, Active: 0, Paused: true},
	}

	for _, worker := range workers {
		err := store.RegisterWorker(ctx, worker)
		if err != nil {
			t.Fatalf("Failed to register worker: %v", err)
		}
	}

	totalSlots, totalActive, err := store.GetWorkerCapacity(ctx, testPool.ID)
	if err != nil {
		t.Fatalf("GetWorkerCapacity() error = %v", err)
	}

	expectedSlots := 8
	expectedActive := 3

	if totalSlots != expectedSlots {
		t.Errorf("Expected total slots %d, got %d", expectedSlots, totalSlots)
	}
	if totalActive != expectedActive {
		t.Errorf("Expected total active %d, got %d", expectedActive, totalActive)
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
				result := pool.SessionsToCreate(3, 2, 8)
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

func TestWorker_Methods(t *testing.T) {
	tests := []struct {
		name     string
		worker   *Worker
		testFunc func(*testing.T, *Worker)
	}{
		{
			name: "IsOnline returns true for recent heartbeat",
			worker: &Worker{
				LastBeat: time.Now().Add(-1 * time.Minute),
			},
			testFunc: func(t *testing.T, worker *Worker) {
				if !worker.IsOnline(5 * time.Minute) {
					t.Error("Expected worker to be online")
				}
			},
		},
		{
			name: "IsOnline returns false for old heartbeat",
			worker: &Worker{
				LastBeat: time.Now().Add(-10 * time.Minute),
			},
			testFunc: func(t *testing.T, worker *Worker) {
				if worker.IsOnline(5 * time.Minute) {
					t.Error("Expected worker to be offline")
				}
			},
		},
		{
			name: "HasCapacity returns true when under max slots",
			worker: &Worker{
				MaxSlots: 5,
				Active:   3,
				Paused:   false,
			},
			testFunc: func(t *testing.T, worker *Worker) {
				if !worker.HasCapacity() {
					t.Error("Expected worker to have capacity")
				}
			},
		},
		{
			name: "HasCapacity returns false when paused",
			worker: &Worker{
				MaxSlots: 5,
				Active:   3,
				Paused:   true,
			},
			testFunc: func(t *testing.T, worker *Worker) {
				if worker.HasCapacity() {
					t.Error("Expected paused worker to have no capacity")
				}
			},
		},
		{
			name: "AvailableSlots calculates correctly",
			worker: &Worker{
				MaxSlots: 5,
				Active:   2,
				Paused:   false,
			},
			testFunc: func(t *testing.T, worker *Worker) {
				expected := 3
				if worker.AvailableSlots() != expected {
					t.Errorf("Expected %d available slots, got %d", expected, worker.AvailableSlots())
				}
			},
		},
		{
			name: "AvailableSlots returns 0 when paused",
			worker: &Worker{
				MaxSlots: 5,
				Active:   2,
				Paused:   true,
			},
			testFunc: func(t *testing.T, worker *Worker) {
				if worker.AvailableSlots() != 0 {
					t.Error("Expected paused worker to have 0 available slots")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t, tt.worker)
		})
	}
}
