package deployments

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetDB() *gorm.DB {
	return s.db
}

// Deployment CRUD operations

func (s *Store) CreateDeployment(ctx context.Context, deployment *Deployment) error {
	if deployment.ID == uuid.Nil {
		deployment.ID = uuid.New()
	}
	return s.db.WithContext(ctx).Create(deployment).Error
}

func (s *Store) GetDeployment(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	var deployment Deployment
	err := s.db.WithContext(ctx).
		Preload("Runs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10)
		}).
		First(&deployment, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	// Calculate computed fields
	s.calculateDeploymentStats(ctx, &deployment)

	return &deployment, nil
}

func (s *Store) GetDeploymentByName(ctx context.Context, name, version string) (*Deployment, error) {
	var deployment Deployment
	err := s.db.WithContext(ctx).
		Where("name = ? AND version = ?", name, version).
		First(&deployment).Error
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

func (s *Store) ListDeployments(ctx context.Context, status *DeploymentStatus, runtime *Runtime, offset, limit int) ([]Deployment, int64, error) {
	query := s.db.WithContext(ctx).Model(&Deployment{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if runtime != nil {
		query = query.Where("runtime = ?", *runtime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var deployments []Deployment
	err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&deployments).Error
	if err != nil {
		return nil, 0, err
	}

	// Calculate computed fields for each deployment
	for i := range deployments {
		s.calculateDeploymentStats(ctx, &deployments[i])
	}

	return deployments, total, nil
}

func (s *Store) UpdateDeployment(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	result := s.db.WithContext(ctx).Model(&Deployment{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status DeploymentStatus) error {
	return s.db.WithContext(ctx).Model(&Deployment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

func (s *Store) DeleteDeployment(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Delete(&Deployment{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeploymentRun CRUD operations

func (s *Store) CreateDeploymentRun(ctx context.Context, run *DeploymentRun) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	return s.db.WithContext(ctx).Create(run).Error
}

func (s *Store) GetDeploymentRun(ctx context.Context, id uuid.UUID) (*DeploymentRun, error) {
	var run DeploymentRun
	err := s.db.WithContext(ctx).
		Preload("Deployment").
		First(&run, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) ListDeploymentRuns(ctx context.Context, deploymentID *uuid.UUID, status *RunStatus, offset, limit int) ([]DeploymentRun, int64, error) {
	query := s.db.WithContext(ctx).Model(&DeploymentRun{})

	if deploymentID != nil {
		query = query.Where("deployment_id = ?", *deploymentID)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var runs []DeploymentRun
	err := query.
		Preload("Deployment").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&runs).Error
	if err != nil {
		return nil, 0, err
	}

	return runs, total, nil
}

func (s *Store) UpdateDeploymentRun(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return s.db.WithContext(ctx).Model(&DeploymentRun{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) UpdateDeploymentRunStatus(ctx context.Context, id uuid.UUID, status RunStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// Set completed_at if the run is transitioning to a terminal state
	if status == RunStatusCompleted || status == RunStatusFailed || status == RunStatusCancelled {
		updates["completed_at"] = time.Now()
	}

	return s.db.WithContext(ctx).Model(&DeploymentRun{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) CompleteDeploymentRun(ctx context.Context, id uuid.UUID, status RunStatus, output map[string]interface{}, errorMsg *string) error {
	updates := map[string]interface{}{
		"status":       status,
		"completed_at": time.Now(),
		"updated_at":   time.Now(),
	}

	if output != nil {
		updates["output"] = output
	}
	if errorMsg != nil {
		updates["error"] = *errorMsg
	}

	return s.db.WithContext(ctx).Model(&DeploymentRun{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) DeleteDeploymentRun(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&DeploymentRun{}, "id = ?", id).Error
}

// Utility methods

func (s *Store) GetActiveDeployments(ctx context.Context) ([]Deployment, error) {
	var deployments []Deployment
	err := s.db.WithContext(ctx).
		Where("status = ?", StatusActive).
		Find(&deployments).Error
	return deployments, err
}

func (s *Store) GetRunningDeploymentRuns(ctx context.Context) ([]DeploymentRun, error) {
	var runs []DeploymentRun
	err := s.db.WithContext(ctx).
		Where("status = ?", RunStatusRunning).
		Find(&runs).Error
	return runs, err
}

func (s *Store) GetDeploymentRunsForSession(ctx context.Context, sessionID uuid.UUID) ([]DeploymentRun, error) {
	var runs []DeploymentRun
	err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Find(&runs).Error
	return runs, err
}

func (s *Store) CleanupOldRuns(ctx context.Context, maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	return s.db.WithContext(ctx).
		Where("created_at < ? AND status IN ?", cutoff, []RunStatus{RunStatusCompleted, RunStatusFailed, RunStatusCancelled}).
		Delete(&DeploymentRun{}).Error
}

// Helper methods

func (s *Store) calculateDeploymentStats(ctx context.Context, deployment *Deployment) {
	var stats struct {
		TotalRuns      int64
		SuccessfulRuns int64
		FailedRuns     int64
		LastRunAt      *time.Time
	}

	s.db.WithContext(ctx).
		Model(&DeploymentRun{}).
		Where("deployment_id = ?", deployment.ID).
		Count(&stats.TotalRuns)

	s.db.WithContext(ctx).
		Model(&DeploymentRun{}).
		Where("deployment_id = ? AND status = ?", deployment.ID, RunStatusCompleted).
		Count(&stats.SuccessfulRuns)

	s.db.WithContext(ctx).
		Model(&DeploymentRun{}).
		Where("deployment_id = ? AND status = ?", deployment.ID, RunStatusFailed).
		Count(&stats.FailedRuns)

	s.db.WithContext(ctx).
		Model(&DeploymentRun{}).
		Where("deployment_id = ?", deployment.ID).
		Select("MAX(created_at)").
		Scan(&stats.LastRunAt)

	deployment.TotalRuns = int(stats.TotalRuns)
	deployment.SuccessfulRuns = int(stats.SuccessfulRuns)
	deployment.FailedRuns = int(stats.FailedRuns)
	deployment.LastRunAt = stats.LastRunAt
}

func (s *Store) GetDeploymentStats(ctx context.Context, deploymentID uuid.UUID) (map[string]interface{}, error) {
	// First check if deployment exists
	var deployment Deployment
	err := s.db.WithContext(ctx).First(&deployment, "id = ?", deploymentID).Error
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}

	// Get run statistics
	var runStats []struct {
		Status RunStatus `json:"status"`
		Count  int       `json:"count"`
	}

	err = s.db.WithContext(ctx).
		Model(&DeploymentRun{}).
		Where("deployment_id = ?", deploymentID).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&runStats).Error
	if err != nil {
		return nil, err
	}

	// Get recent runs
	var recentRuns []DeploymentRun
	err = s.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("created_at DESC").
		Limit(10).
		Find(&recentRuns).Error
	if err != nil {
		return nil, err
	}

	// Calculate average runtime
	var avgDuration *float64
	err = s.db.WithContext(ctx).
		Model(&DeploymentRun{}).
		Where("deployment_id = ? AND completed_at IS NOT NULL", deploymentID).
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - started_at)))").
		Scan(&avgDuration).Error
	if err != nil {
		return nil, err
	}

	stats = map[string]interface{}{
		"run_stats":    runStats,
		"recent_runs":  recentRuns,
		"avg_duration": avgDuration,
	}

	return stats, nil
}
