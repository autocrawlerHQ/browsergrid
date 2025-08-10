package workpool

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPoolServiceImpl_GetOrCreateDefault(t *testing.T) {
	db := setupPoolServiceTestDB(t)
	defer cleanupPoolServiceTestDB(db)

	service := NewPoolService(db)
	ctx := context.Background()

	tests := []struct {
		name     string
		provider string
		setup    func()
		verify   func(t *testing.T, poolID uuid.UUID, err error)
	}{
		{
			name:     "create new default pool",
			provider: "docker",
			setup:    func() {},
			verify: func(t *testing.T, poolID uuid.UUID, err error) {
				require.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, poolID)

				var pool WorkPool
				err = db.Where("id = ?", poolID).First(&pool).Error
				require.NoError(t, err)
				assert.Equal(t, "default-docker", pool.Name)
				assert.Equal(t, ProviderDocker, pool.Provider)
				assert.Equal(t, 10, pool.MaxConcurrency)
				assert.True(t, pool.AutoScale)
			},
		},
		{
			name:     "get existing default pool",
			provider: "azure_aci",
			setup: func() {
				existingPool := WorkPool{
					ID:             uuid.New(),
					Name:           "default-azure_aci",
					Provider:       ProviderDocker,
					MaxConcurrency: 15,
					AutoScale:      false,
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				}
				err := db.Create(&existingPool).Error
				require.NoError(t, err)
			},
			verify: func(t *testing.T, poolID uuid.UUID, err error) {
				require.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, poolID)

				var pool WorkPool
				err = db.Where("id = ?", poolID).First(&pool).Error
				require.NoError(t, err)
				assert.Equal(t, "default-azure_aci", pool.Name)
				assert.Equal(t, ProviderDocker, pool.Provider)
				assert.Equal(t, 15, pool.MaxConcurrency)
				assert.False(t, pool.AutoScale)

				var count int64
				err = db.Model(&WorkPool{}).Where("name = ?", "default-azure_aci").Count(&count).Error
				require.NoError(t, err)
				assert.Equal(t, int64(1), count)
			},
		},
		{
			name:     "create pool with local provider",
			provider: "local",
			setup:    func() {},
			verify: func(t *testing.T, poolID uuid.UUID, err error) {
				require.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, poolID)

				var pool WorkPool
				err = db.Where("id = ?", poolID).First(&pool).Error
				require.NoError(t, err)
				assert.Equal(t, "default-local", pool.Name)
				assert.Equal(t, ProviderDocker, pool.Provider)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.Exec("DELETE FROM work_pools")

			tt.setup()
			poolID, err := service.GetOrCreateDefault(ctx, tt.provider)
			tt.verify(t, poolID, err)
		})
	}
}

func TestPoolServiceImpl_GetOrCreateDefault_Concurrent(t *testing.T) {
	db := setupPoolServiceTestDB(t)
	defer cleanupPoolServiceTestDB(db)

	service := NewPoolService(db)
	ctx := context.Background()
	provider := "docker"

	db.Exec("DELETE FROM work_pools")

	const numGoroutines = 10
	results := make(chan struct {
		id  uuid.UUID
		err error
	}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			id, err := service.GetOrCreateDefault(ctx, provider)
			results <- struct {
				id  uuid.UUID
				err error
			}{id, err}
		}()
	}

	var poolIDs []uuid.UUID
	for i := 0; i < numGoroutines; i++ {
		result := <-results
		require.NoError(t, result.err)
		poolIDs = append(poolIDs, result.id)
	}

	firstID := poolIDs[0]
	for _, id := range poolIDs {
		assert.Equal(t, firstID, id, "All concurrent calls should return the same pool ID")
	}

	var count int64
	err := db.Model(&WorkPool{}).Where("name = ?", "default-docker").Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Only one pool should be created despite concurrent calls")
}

func TestPoolServiceImpl_GetOrCreateDefault_TableAutoMigration(t *testing.T) {
	db, err := gorm.Open(postgres.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	db.Exec("DROP TABLE IF EXISTS work_pools")

	service := NewPoolService(db)
	ctx := context.Background()

	poolID, err := service.GetOrCreateDefault(ctx, "docker")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, poolID)

	var pool WorkPool
	err = db.Where("id = ?", poolID).First(&pool).Error
	require.NoError(t, err)
	assert.Equal(t, "default-docker", pool.Name)
}

func TestPoolServiceImpl_GetOrCreateDefault_InvalidProvider(t *testing.T) {
	db := setupPoolServiceTestDB(t)
	defer cleanupPoolServiceTestDB(db)

	service := NewPoolService(db)
	ctx := context.Background()

	poolID, err := service.GetOrCreateDefault(ctx, "")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, poolID)

	var pool WorkPool
	err = db.Where("id = ?", poolID).First(&pool).Error
	require.NoError(t, err)
	assert.Equal(t, "default-", pool.Name)
}

func setupPoolServiceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(postgres.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.Migrator().DropTable(&WorkPool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&WorkPool{})
	require.NoError(t, err)

	return db
}

func cleanupPoolServiceTestDB(db *gorm.DB) {
	db.Exec("DELETE FROM work_pools")
}
