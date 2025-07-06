package workpool

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateWorkPool(ctx context.Context, pool *WorkPool) error {
	if pool.ID == uuid.Nil {
		pool.ID = uuid.New()
	}
	return s.db.WithContext(ctx).Create(pool).Error
}

func (s *Store) GetWorkPool(ctx context.Context, id uuid.UUID) (*WorkPool, error) {
	var pool WorkPool
	err := s.db.WithContext(ctx).First(&pool, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (s *Store) GetWorkPoolByName(ctx context.Context, name string) (*WorkPool, error) {
	var pool WorkPool
	err := s.db.WithContext(ctx).First(&pool, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (s *Store) ListWorkPools(ctx context.Context, paused *bool) ([]WorkPool, error) {
	query := s.db.WithContext(ctx).Model(&WorkPool{})

	if paused != nil {
		query = query.Where("paused = ?", *paused)
	}

	var pools []WorkPool
	err := query.Order("created_at DESC").Find(&pools).Error
	return pools, err
}

func (s *Store) UpdateWorkPool(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return s.db.WithContext(ctx).Model(&WorkPool{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) DeleteWorkPool(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&WorkPool{}, "id = ?", id).Error
}

func (s *Store) DrainWorkPool(ctx context.Context, id uuid.UUID) error {
	updates := map[string]interface{}{
		"paused":     true,
		"auto_scale": false,
		"min_size":   0,
		"updated_at": time.Now(),
	}

	return s.db.WithContext(ctx).Model(&WorkPool{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) GetPoolCapacity(ctx context.Context, poolID uuid.UUID) (int, int, error) {
	var pool WorkPool
	err := s.db.WithContext(ctx).First(&pool, "id = ?", poolID).Error
	if err != nil {
		return 0, 0, err
	}

	var activeCount int64
	err = s.db.WithContext(ctx).Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status IN ?", poolID,
			[]sessions.SessionStatus{sessions.StatusStarting, sessions.StatusRunning, sessions.StatusIdle}).
		Count(&activeCount).Error
	if err != nil {
		return 0, 0, err
	}

	return pool.MaxConcurrency, int(activeCount), nil
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&WorkPool{})
}
