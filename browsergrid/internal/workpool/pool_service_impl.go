package workpool

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PoolServiceImpl implements the sessions.PoolService interface
type PoolServiceImpl struct{ db *gorm.DB }

func NewPoolService(db *gorm.DB) *PoolServiceImpl {
	return &PoolServiceImpl{db: db}
}

func (p *PoolServiceImpl) GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error) {
	// Auto-create table if it doesn't exist
	if err := p.db.WithContext(ctx).AutoMigrate(&WorkPool{}); err != nil {
		return uuid.Nil, err
	}

	poolName := fmt.Sprintf("default-%s", provider)

	var pool WorkPool
	err := p.db.WithContext(ctx).Where("name = ?", poolName).First(&pool).Error
	if err == nil {
		return pool.ID, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.Nil, err
	}

	pool = WorkPool{
		ID:             uuid.New(),
		Name:           poolName,
		Provider:       ProviderType(provider),
		MaxConcurrency: 10,
		AutoScale:      true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return pool.ID, p.db.WithContext(ctx).Create(&pool).Error
}
