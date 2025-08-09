package deployments

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func setupStoreTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(postgresDriver.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Drop and recreate tables
	err = db.Migrator().DropTable(&Deployment{}, &DeploymentRun{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&Deployment{}, &DeploymentRun{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestStore_CreateDeployment(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	tests := []struct {
		name       string
		deployment *Deployment
		wantErr    bool
	}{
		{
			name: "valid deployment",
			deployment: &Deployment{
				Name:        "test-deployment",
				Description: "Test deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
				Config:      datatypes.JSON(`{"concurrency": 1}`),
				Status:      StatusActive,
			},
			wantErr: false,
		},
		{
			name: "deployment with nil config",
			deployment: &Deployment{
				Name:        "test-deployment-2",
				Version:     "1.0.0",
				Runtime:     RuntimePython,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash456",
			},
			wantErr: false,
		},
		{
			name: "deployment with empty name",
			deployment: &Deployment{
				Name:        "",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash789",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.CreateDeployment(ctx, tt.deployment)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.deployment.ID)
				assert.NotZero(t, tt.deployment.CreatedAt)
				assert.NotZero(t, tt.deployment.UpdatedAt)
			}
		})
	}
}

func TestStore_GetDeployment(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Description: "Test deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test runs
	runs := []*DeploymentRun{
		{
			DeploymentID: deployment.ID,
			Status:       RunStatusCompleted,
		},
		{
			DeploymentID: deployment.ID,
			Status:       RunStatusFailed,
		},
	}
	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name:    "existing deployment",
			id:      deployment.ID,
			wantErr: false,
		},
		{
			name:    "non-existent deployment",
			id:      uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.GetDeployment(ctx, tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.id, result.ID)
				assert.Equal(t, deployment.Name, result.Name)
				// Check computed fields
				assert.Equal(t, 2, result.TotalRuns)
				assert.Equal(t, 1, result.SuccessfulRuns)
				assert.Equal(t, 1, result.FailedRuns)
				assert.NotNil(t, result.LastRunAt)
			}
		})
	}
}

func TestStore_GetDeploymentByName(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	tests := []struct {
		name    string
		depName string
		version string
		wantErr bool
	}{
		{
			name:    "existing deployment",
			depName: "test-deployment",
			version: "1.0.0",
			wantErr: false,
		},
		{
			name:    "non-existent deployment",
			depName: "non-existent",
			version: "1.0.0",
			wantErr: true,
		},
		{
			name:    "existing name, wrong version",
			depName: "test-deployment",
			version: "2.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.GetDeploymentByName(ctx, tt.depName, tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.depName, result.Name)
				assert.Equal(t, tt.version, result.Version)
			}
		})
	}
}

func TestStore_ListDeployments(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployments
	deployments := []*Deployment{
		{
			Name:        "node-deployment",
			Version:     "1.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/node.zip",
			PackageHash: "hash1",
			Status:      StatusActive,
		},
		{
			Name:        "python-deployment",
			Version:     "1.0.0",
			Runtime:     RuntimePython,
			PackageURL:  "https://example.com/python.zip",
			PackageHash: "hash2",
			Status:      StatusActive,
		},
		{
			Name:        "inactive-deployment",
			Version:     "1.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/inactive.zip",
			PackageHash: "hash3",
			Status:      StatusInactive,
		},
	}

	for _, dep := range deployments {
		err := store.CreateDeployment(ctx, dep)
		require.NoError(t, err)
	}

	tests := []struct {
		name        string
		status      *DeploymentStatus
		runtime     *Runtime
		offset      int
		limit       int
		expectedLen int
		expectedMin int
	}{
		{
			name:        "all deployments",
			offset:      0,
			limit:       10,
			expectedLen: 3,
			expectedMin: 3,
		},
		{
			name:        "active deployments only",
			status:      &[]DeploymentStatus{StatusActive}[0],
			offset:      0,
			limit:       10,
			expectedLen: 2,
			expectedMin: 2,
		},
		{
			name:        "inactive deployments only",
			status:      &[]DeploymentStatus{StatusInactive}[0],
			offset:      0,
			limit:       10,
			expectedLen: 1,
			expectedMin: 1,
		},
		{
			name:        "node runtime only",
			runtime:     &[]Runtime{RuntimeNode}[0],
			offset:      0,
			limit:       10,
			expectedLen: 2,
			expectedMin: 2,
		},
		{
			name:        "python runtime only",
			runtime:     &[]Runtime{RuntimePython}[0],
			offset:      0,
			limit:       10,
			expectedLen: 1,
			expectedMin: 1,
		},
		{
			name:        "pagination - first page",
			offset:      0,
			limit:       2,
			expectedLen: 2,
			expectedMin: 2,
		},
		{
			name:        "pagination - second page",
			offset:      2,
			limit:       2,
			expectedLen: 1,
			expectedMin: 1,
		},
		{
			name:        "combined filters",
			status:      &[]DeploymentStatus{StatusActive}[0],
			runtime:     &[]Runtime{RuntimeNode}[0],
			offset:      0,
			limit:       10,
			expectedLen: 1,
			expectedMin: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, total, err := store.ListDeployments(ctx, tt.status, tt.runtime, tt.offset, tt.limit)
			assert.NoError(t, err)
			assert.Len(t, result, tt.expectedLen)
			assert.GreaterOrEqual(t, int(total), tt.expectedMin)

			// Verify filtering
			if tt.status != nil {
				for _, dep := range result {
					assert.Equal(t, *tt.status, dep.Status)
				}
			}
			if tt.runtime != nil {
				for _, dep := range result {
					assert.Equal(t, *tt.runtime, dep.Runtime)
				}
			}
		})
	}
}

func TestStore_UpdateDeployment(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	originalUpdatedAt := deployment.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	tests := []struct {
		name    string
		id      uuid.UUID
		updates map[string]interface{}
		wantErr bool
	}{
		{
			name: "update description",
			id:   deployment.ID,
			updates: map[string]interface{}{
				"description": "Updated description",
			},
			wantErr: false,
		},
		{
			name: "update status",
			id:   deployment.ID,
			updates: map[string]interface{}{
				"status": StatusInactive,
			},
			wantErr: false,
		},
		{
			name: "update multiple fields",
			id:   deployment.ID,
			updates: map[string]interface{}{
				"description": "Final description",
				"status":      StatusActive,
			},
			wantErr: false,
		},
		{
			name: "update non-existent deployment",
			id:   uuid.New(),
			updates: map[string]interface{}{
				"description": "Should fail",
			},
			wantErr: true, // Should return error for non-existent deployment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.UpdateDeployment(ctx, tt.id, tt.updates)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// If updating the original deployment, verify changes
				if tt.id == deployment.ID {
					updated, err := store.GetDeployment(ctx, tt.id)
					require.NoError(t, err)
					assert.True(t, updated.UpdatedAt.After(originalUpdatedAt))

					for key, value := range tt.updates {
						switch key {
						case "description":
							assert.Equal(t, value, updated.Description)
						case "status":
							assert.Equal(t, value, updated.Status)
						}
					}
				}
			}
		})
	}
}

func TestStore_UpdateDeploymentStatus(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	originalUpdatedAt := deployment.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	err = store.UpdateDeploymentStatus(ctx, deployment.ID, StatusInactive)
	assert.NoError(t, err)

	// Verify update
	updated, err := store.GetDeployment(ctx, deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusInactive, updated.Status)
	assert.True(t, updated.UpdatedAt.After(originalUpdatedAt))
}

func TestStore_DeleteDeployment(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test run
	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
	}
	err = store.CreateDeploymentRun(ctx, run)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name:    "delete existing deployment",
			id:      deployment.ID,
			wantErr: false,
		},
		{
			name:    "delete non-existent deployment",
			id:      uuid.New(),
			wantErr: true, // Should return error for non-existent deployment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.DeleteDeployment(ctx, tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify deletion
				if tt.id == deployment.ID {
					_, err := store.GetDeployment(ctx, tt.id)
					assert.Error(t, err)

					// Verify cascade deletion of runs
					_, err = store.GetDeploymentRun(ctx, run.ID)
					assert.Error(t, err)
				}
			}
		})
	}
}

func TestStore_DeploymentRun_CRUD(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Test Create
	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusPending,
		Output:       datatypes.JSON(`{"message": "starting"}`),
	}
	err = store.CreateDeploymentRun(ctx, run)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, run.ID)

	// Test Get
	retrieved, err := store.GetDeploymentRun(ctx, run.ID)
	assert.NoError(t, err)
	assert.Equal(t, run.ID, retrieved.ID)
	assert.Equal(t, run.DeploymentID, retrieved.DeploymentID)
	assert.Equal(t, run.Status, retrieved.Status)
	assert.NotNil(t, retrieved.Deployment)

	// Test Update
	updates := map[string]interface{}{
		"status": RunStatusRunning,
		"output": datatypes.JSON(`{"message": "running"}`),
	}
	err = store.UpdateDeploymentRun(ctx, run.ID, updates)
	assert.NoError(t, err)

	// Verify update
	updated, err := store.GetDeploymentRun(ctx, run.ID)
	assert.NoError(t, err)
	assert.Equal(t, RunStatusRunning, updated.Status)

	// Test UpdateStatus
	err = store.UpdateDeploymentRunStatus(ctx, run.ID, RunStatusCompleted)
	assert.NoError(t, err)

	// Verify status update
	completed, err := store.GetDeploymentRun(ctx, run.ID)
	assert.NoError(t, err)
	assert.Equal(t, RunStatusCompleted, completed.Status)
	assert.NotNil(t, completed.CompletedAt)

	// Test CompleteDeploymentRun
	output := map[string]interface{}{
		"result": "success",
		"data":   "test output",
	}
	err = store.CompleteDeploymentRun(ctx, run.ID, RunStatusCompleted, output, nil)
	assert.NoError(t, err)

	// Test Delete
	err = store.DeleteDeploymentRun(ctx, run.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = store.GetDeploymentRun(ctx, run.ID)
	assert.Error(t, err)
}

func TestStore_ListDeploymentRuns(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployments
	deployment1 := &Deployment{
		Name:        "test-deployment-1",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package1.zip",
		PackageHash: "hash1",
		Status:      StatusActive,
	}
	deployment2 := &Deployment{
		Name:        "test-deployment-2",
		Version:     "1.0.0",
		Runtime:     RuntimePython,
		PackageURL:  "https://example.com/package2.zip",
		PackageHash: "hash2",
		Status:      StatusActive,
	}

	err := store.CreateDeployment(ctx, deployment1)
	require.NoError(t, err)
	err = store.CreateDeployment(ctx, deployment2)
	require.NoError(t, err)

	// Create test runs
	runs := []*DeploymentRun{
		{DeploymentID: deployment1.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment1.ID, Status: RunStatusFailed},
		{DeploymentID: deployment2.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment2.ID, Status: RunStatusRunning},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	tests := []struct {
		name         string
		deploymentID *uuid.UUID
		status       *RunStatus
		offset       int
		limit        int
		expectedLen  int
		expectedMin  int
	}{
		{
			name:        "all runs",
			offset:      0,
			limit:       10,
			expectedLen: 4,
			expectedMin: 4,
		},
		{
			name:         "runs for deployment1",
			deploymentID: &deployment1.ID,
			offset:       0,
			limit:        10,
			expectedLen:  2,
			expectedMin:  2,
		},
		{
			name:         "runs for deployment2",
			deploymentID: &deployment2.ID,
			offset:       0,
			limit:        10,
			expectedLen:  2,
			expectedMin:  2,
		},
		{
			name:        "completed runs only",
			status:      &[]RunStatus{RunStatusCompleted}[0],
			offset:      0,
			limit:       10,
			expectedLen: 2,
			expectedMin: 2,
		},
		{
			name:        "failed runs only",
			status:      &[]RunStatus{RunStatusFailed}[0],
			offset:      0,
			limit:       10,
			expectedLen: 1,
			expectedMin: 1,
		},
		{
			name:        "pagination - first page",
			offset:      0,
			limit:       2,
			expectedLen: 2,
			expectedMin: 2,
		},
		{
			name:        "pagination - second page",
			offset:      2,
			limit:       2,
			expectedLen: 2,
			expectedMin: 2,
		},
		{
			name:         "combined filters",
			deploymentID: &deployment1.ID,
			status:       &[]RunStatus{RunStatusCompleted}[0],
			offset:       0,
			limit:        10,
			expectedLen:  1,
			expectedMin:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, total, err := store.ListDeploymentRuns(ctx, tt.deploymentID, tt.status, tt.offset, tt.limit)
			assert.NoError(t, err)
			assert.Len(t, result, tt.expectedLen)
			assert.GreaterOrEqual(t, int(total), tt.expectedMin)

			// Verify filtering
			if tt.deploymentID != nil {
				for _, run := range result {
					assert.Equal(t, *tt.deploymentID, run.DeploymentID)
				}
			}
			if tt.status != nil {
				for _, run := range result {
					assert.Equal(t, *tt.status, run.Status)
				}
			}
		})
	}
}

func TestStore_GetActiveDeployments(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployments
	deployments := []*Deployment{
		{Name: "active-1", Version: "1.0.0", Runtime: RuntimeNode, PackageURL: "url1", PackageHash: "hash1", Status: StatusActive},
		{Name: "active-2", Version: "1.0.0", Runtime: RuntimeNode, PackageURL: "url2", PackageHash: "hash2", Status: StatusActive},
		{Name: "inactive-1", Version: "1.0.0", Runtime: RuntimeNode, PackageURL: "url3", PackageHash: "hash3", Status: StatusInactive},
	}

	for _, dep := range deployments {
		err := store.CreateDeployment(ctx, dep)
		require.NoError(t, err)
	}

	active, err := store.GetActiveDeployments(ctx)
	assert.NoError(t, err)
	assert.Len(t, active, 2)

	for _, dep := range active {
		assert.Equal(t, StatusActive, dep.Status)
	}
}

func TestStore_GetRunningDeploymentRuns(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test runs
	runs := []*DeploymentRun{
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	running, err := store.GetRunningDeploymentRuns(ctx)
	assert.NoError(t, err)
	assert.Len(t, running, 2)

	for _, run := range running {
		assert.Equal(t, RunStatusRunning, run.Status)
	}
}

func TestStore_CleanupOldRuns(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create old completed run
	oldRun := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
	}
	err = store.CreateDeploymentRun(ctx, oldRun)
	require.NoError(t, err)

	// Manually set old created_at time
	err = db.Model(oldRun).Update("created_at", time.Now().Add(-25*time.Hour)).Error
	require.NoError(t, err)

	// Create recent run
	recentRun := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
	}
	err = store.CreateDeploymentRun(ctx, recentRun)
	require.NoError(t, err)

	// Create old running run (should not be deleted)
	oldRunningRun := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusRunning,
	}
	err = store.CreateDeploymentRun(ctx, oldRunningRun)
	require.NoError(t, err)

	// Manually set old created_at time
	err = db.Model(oldRunningRun).Update("created_at", time.Now().Add(-25*time.Hour)).Error
	require.NoError(t, err)

	// Cleanup runs older than 24 hours
	err = store.CleanupOldRuns(ctx, 24*time.Hour)
	assert.NoError(t, err)

	// Verify old completed run was deleted
	_, err = store.GetDeploymentRun(ctx, oldRun.ID)
	assert.Error(t, err)

	// Verify recent run still exists
	_, err = store.GetDeploymentRun(ctx, recentRun.ID)
	assert.NoError(t, err)

	// Verify old running run still exists (not terminal)
	_, err = store.GetDeploymentRun(ctx, oldRunningRun.ID)
	assert.NoError(t, err)
}

func TestStore_GetDeploymentStats(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test runs with different statuses
	runs := []*DeploymentRun{
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment.ID, Status: RunStatusFailed},
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	stats, err := store.GetDeploymentStats(ctx, deployment.ID)
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// Verify stats structure
	assert.Contains(t, stats, "run_stats")
	assert.Contains(t, stats, "recent_runs")
	assert.Contains(t, stats, "avg_duration")

	// Verify recent runs
	recentRuns, ok := stats["recent_runs"].([]DeploymentRun)
	assert.True(t, ok)
	assert.Len(t, recentRuns, 4)
}

func TestStore_GetDeploymentRunsForSession(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	sessionID := uuid.New()

	// Create test runs
	runs := []*DeploymentRun{
		{DeploymentID: deployment.ID, Status: RunStatusCompleted, SessionID: &sessionID},
		{DeploymentID: deployment.ID, Status: RunStatusCompleted, SessionID: &sessionID},
		{DeploymentID: deployment.ID, Status: RunStatusCompleted}, // No session ID
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	// Get runs for session
	sessionRuns, err := store.GetDeploymentRunsForSession(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, sessionRuns, 2)

	for _, run := range sessionRuns {
		assert.Equal(t, sessionID, *run.SessionID)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	db := setupStoreTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "concurrent-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Test concurrent run creation
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			run := &DeploymentRun{
				DeploymentID: deployment.ID,
				Status:       RunStatusPending,
			}
			results <- store.CreateDeploymentRun(ctx, run)
		}(i)
	}

	// Check results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-results == nil {
			successCount++
		}
	}

	assert.Equal(t, numGoroutines, successCount, "All concurrent operations should succeed")

	// Verify all runs were created
	runs, total, err := store.ListDeploymentRuns(ctx, &deployment.ID, nil, 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, int64(numGoroutines), total)
	assert.Len(t, runs, numGoroutines)
}
