package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// GetDB returns the underlying database connection for advanced operations
func (s *Store) GetDB() *gorm.DB {
	return s.db
}

func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}
	return s.db.WithContext(ctx).Create(sess).Error
}

func (s *Store) GetSession(ctx context.Context, id uuid.UUID) (*Session, error) {
	var sess Session
	err := s.db.WithContext(ctx).First(&sess, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) UpdateSessionStatus(ctx context.Context, id uuid.UUID, status SessionStatus) error {
	return s.db.WithContext(ctx).Model(&Session{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (s *Store) UpdateSessionEndpoints(ctx context.Context, id uuid.UUID, wsEndpoint, liveURL string, status SessionStatus) error {
	updates := map[string]interface{}{
		"status":      status,
		"ws_endpoint": wsEndpoint,
		"live_url":    liveURL,
		"updated_at":  time.Now(),
	}
	return s.db.WithContext(ctx).Model(&Session{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) ListSessions(ctx context.Context,
	status *SessionStatus, start, end *time.Time,
	offset, limit int) ([]Session, error) {

	query := s.db.WithContext(ctx).Model(&Session{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if end != nil {
		query = query.Where("created_at <= ?", *end)
	}

	var sessions []Session
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&sessions).Error

	return sessions, err
}

func (s *Store) CreatePool(ctx context.Context, pool *Pool) error {
	return s.db.WithContext(ctx).Create(pool).Error
}

func (s *Store) GetPool(ctx context.Context, id string) (*Pool, error) {
	var pool Pool
	err := s.db.WithContext(ctx).First(&pool, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (s *Store) ListPools(ctx context.Context, enabled *bool) ([]Pool, error) {
	query := s.db.WithContext(ctx).Model(&Pool{})

	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	var pools []Pool
	err := query.Order("created_at DESC").Find(&pools).Error
	return pools, err
}

func (s *Store) UpdatePoolSizes(ctx context.Context, poolID string, currentSize, availableSize int) error {
	return s.db.WithContext(ctx).Model(&Pool{}).
		Where("id = ?", poolID).
		Updates(map[string]interface{}{
			"current_size":   currentSize,
			"available_size": availableSize,
			"updated_at":     time.Now(),
		}).Error
}

func (s *Store) CreatePooledSession(ctx context.Context, poolID string, sess *Session) error {
	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}

	sess.PoolID = &poolID
	sess.IsPooled = true
	sess.Status = StatusPending

	return s.db.WithContext(ctx).Create(sess).Error
}

func (s *Store) GetAvailableSessions(ctx context.Context, poolID string, limit int) ([]Session, error) {
	var sessions []Session

	query := s.db.WithContext(ctx).
		Where("pool_id = ? AND status = ? AND claimed_by IS NULL", poolID, StatusAvailable).
		Order("available_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&sessions).Error
	return sessions, err
}

func (s *Store) ClaimSession(ctx context.Context, sessionID uuid.UUID, claimedBy string) error {
	now := time.Now()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session Session
		err := tx.Where("id = ? AND status = ? AND claimed_by IS NULL",
			sessionID, StatusAvailable).First(&session).Error
		if err != nil {
			return fmt.Errorf("session not available for claiming: %w", err)
		}

		err = tx.Model(&Session{}).
			Where("id = ?", sessionID).
			Updates(map[string]interface{}{
				"status":     StatusClaimed,
				"claimed_at": &now,
				"claimed_by": claimedBy,
				"updated_at": now,
			}).Error
		if err != nil {
			return err
		}

		if session.PoolID != nil {
			err = tx.Model(&Pool{}).
				Where("id = ?", *session.PoolID).
				UpdateColumn("available_size", gorm.Expr("available_size - 1")).Error
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Store) ReleaseSession(ctx context.Context, sessionID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session Session
		err := tx.Where("id = ?", sessionID).First(&session).Error
		if err != nil {
			return err
		}

		if session.IsPooled && session.PoolID != nil {
			now := time.Now()
			err = tx.Model(&Session{}).
				Where("id = ?", sessionID).
				Updates(map[string]interface{}{
					"status":       StatusAvailable,
					"claimed_at":   nil,
					"claimed_by":   nil,
					"available_at": &now,
					"updated_at":   now,
				}).Error
			if err == nil {
				err = tx.Model(&Pool{}).
					Where("id = ?", *session.PoolID).
					UpdateColumn("available_size", gorm.Expr("available_size + 1")).Error
			}
		} else {
			err = tx.Model(&Session{}).
				Where("id = ?", sessionID).
				Update("status", StatusTerminated).Error
		}

		return err
	})
}

func (s *Store) MarkSessionAvailable(ctx context.Context, sessionID uuid.UUID) error {
	now := time.Now()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session Session
		err := tx.Where("id = ?", sessionID).First(&session).Error
		if err != nil {
			return err
		}

		err = tx.Model(&Session{}).
			Where("id = ?", sessionID).
			Updates(map[string]interface{}{
				"status":       StatusAvailable,
				"available_at": &now,
				"updated_at":   now,
			}).Error
		if err != nil {
			return err
		}

		if session.IsPooled && session.PoolID != nil {
			err = tx.Model(&Pool{}).
				Where("id = ?", *session.PoolID).
				UpdateColumn("available_size", gorm.Expr("available_size + 1")).Error
		}

		return err
	})
}

func (s *Store) GetPoolSessions(ctx context.Context, poolID string, status *SessionStatus) ([]Session, error) {
	query := s.db.WithContext(ctx).Where("pool_id = ?", poolID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var sessions []Session
	err := query.Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

func (s *Store) CleanupExpiredSessions(ctx context.Context, poolID string, maxIdleSeconds int) (int, error) {
	cutoff := time.Now().Add(-time.Duration(maxIdleSeconds) * time.Second)

	result := s.db.WithContext(ctx).
		Model(&Session{}).
		Where("pool_id = ? AND status = ? AND available_at < ?", poolID, StatusAvailable, cutoff).
		Update("status", StatusExpired)

	return int(result.RowsAffected), result.Error
}

func (s *Store) CreateEvent(ctx context.Context, ev *SessionEvent) error {
	if ev.ID == uuid.Nil {
		ev.ID = uuid.New()
	}
	return s.db.WithContext(ctx).Create(ev).Error
}

func (s *Store) ListEvents(ctx context.Context,
	sessionID *uuid.UUID, eventType *SessionEventType,
	start, end *time.Time, offset, limit int) ([]SessionEvent, error) {

	query := s.db.WithContext(ctx).Model(&SessionEvent{})

	if sessionID != nil {
		query = query.Where("session_id = ?", *sessionID)
	}
	if eventType != nil {
		query = query.Where("event = ?", *eventType)
	}
	if start != nil {
		query = query.Where("timestamp >= ?", *start)
	}
	if end != nil {
		query = query.Where("timestamp <= ?", *end)
	}

	var events []SessionEvent
	err := query.Order("timestamp DESC").
		Offset(offset).
		Limit(limit).
		Find(&events).Error

	return events, err
}

func (s *Store) CreateMetrics(ctx context.Context, metrics *SessionMetrics) error {
	if metrics.ID == uuid.Nil {
		metrics.ID = uuid.New()
	}
	return s.db.WithContext(ctx).Create(metrics).Error
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(
		&Session{},
		&SessionEvent{},
		&SessionMetrics{},
		&Pool{},
	)
}

// MarkWorkerSessionsFailed marks all non-terminal sessions assigned to a worker as failed
// This is used during worker shutdown to prevent orphaned sessions
func (s *Store) MarkWorkerSessionsFailed(ctx context.Context, workerID uuid.UUID) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&Session{}).
		Where("worker_id = ? AND status IN (?)", workerID, []SessionStatus{
			StatusStarting,
			StatusRunning,
			StatusIdle,
		}).
		Updates(map[string]interface{}{
			"status":     StatusFailed,
			"updated_at": time.Now(),
		})

	return result.RowsAffected, result.Error
}
