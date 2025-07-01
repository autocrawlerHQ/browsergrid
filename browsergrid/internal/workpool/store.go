package workpool

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

func (s *Store) RegisterWorker(ctx context.Context, w *Worker) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}

	now := time.Now()
	w.LastBeat = now
	w.StartedAt = now

	// Use stable identity based on pool_id + hostname
	// This prevents duplicate workers when the same physical worker restarts
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "pool_id"},
			{Name: "hostname"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"id", "name", "provider", "max_slots", "active", "last_beat", "started_at", "paused",
		}),
	}).Create(w).Error
}

func (s *Store) GetWorker(ctx context.Context, id uuid.UUID) (*Worker, error) {
	var worker Worker
	err := s.db.WithContext(ctx).Preload("Pool").First(&worker, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

func (s *Store) ListWorkers(ctx context.Context, poolID *uuid.UUID, online *bool) ([]Worker, error) {
	query := s.db.WithContext(ctx).Model(&Worker{}).Preload("Pool")

	if poolID != nil {
		query = query.Where("pool_id = ?", *poolID)
	}

	var workers []Worker
	err := query.Order("started_at DESC").Find(&workers).Error
	if err != nil {
		return nil, err
	}

	if online != nil {
		ttl := 5 * time.Minute
		filtered := make([]Worker, 0, len(workers))
		for _, w := range workers {
			if w.IsOnline(ttl) == *online {
				filtered = append(filtered, w)
			}
		}
		return filtered, nil
	}

	return workers, nil
}

func (s *Store) Heartbeat(ctx context.Context, id uuid.UUID, active int) error {
	return s.db.WithContext(ctx).
		Model(&Worker{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_beat": time.Now(),
			"active":    active,
		}).Error
}

func (s *Store) UpdateWorkerActive(ctx context.Context, id uuid.UUID, delta int) error {
	return s.db.WithContext(ctx).
		Model(&Worker{}).
		Where("id = ?", id).
		UpdateColumn("active", gorm.Expr("active + ?", delta)).Error
}

func (s *Store) PauseWorker(ctx context.Context, id uuid.UUID, paused bool) error {
	return s.db.WithContext(ctx).
		Model(&Worker{}).
		Where("id = ?", id).
		Update("paused", paused).Error
}

func (s *Store) DeleteWorker(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&Worker{}, "id = ?", id).Error
}

func (s *Store) DequeueSessions(
	ctx context.Context, poolID uuid.UUID, workerID uuid.UUID, limit int,
) ([]sessions.Session, error) {
	var out []sessions.Session

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.
			Where("status = ? AND work_pool_id = ?", sessions.StatusPending, poolID).
			Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Find(&out)

		if result.Error != nil {
			return result.Error
		}

		if len(out) == 0 {
			return nil
		}

		ids := make([]uuid.UUID, len(out))
		for i, s := range out {
			ids[i] = s.ID
		}

		err := tx.Model(&sessions.Session{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":     sessions.StatusStarting,
				"worker_id":  workerID,
				"updated_at": time.Now(),
			}).Error
		if err != nil {
			return err
		}

		for i := range out {
			out[i].Status = sessions.StatusStarting
			out[i].WorkerID = &workerID
			out[i].UpdatedAt = time.Now()
		}

		return nil
	})

	return out, err
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

func (s *Store) GetWorkerCapacity(ctx context.Context, poolID uuid.UUID) (int, int, error) {
	var workers []Worker
	err := s.db.WithContext(ctx).
		Where("pool_id = ? AND paused = false", poolID).
		Find(&workers).Error
	if err != nil {
		return 0, 0, err
	}

	totalSlots := 0
	totalActive := 0
	ttl := 5 * time.Minute

	for _, w := range workers {
		if w.IsOnline(ttl) {
			totalSlots += w.MaxSlots
			totalActive += w.Active
		}
	}

	return totalSlots, totalActive, nil
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(
		&WorkPool{},
		&Worker{},
	)
}
