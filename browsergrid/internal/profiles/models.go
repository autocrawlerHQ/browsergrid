package profiles

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

// Constants
const (
	maxProfileSize = 1 << 30 // 1GB
)

// Profile represents a reusable browser profile with saved state
type Profile struct {
	ID          uuid.UUID        `json:"id" gorm:"type:uuid;primary_key"`
	Name        string           `json:"name" gorm:"not null;index"`
	Description string           `json:"description"`
	Browser     sessions.Browser `json:"browser" gorm:"not null"`
	SizeBytes   int64            `json:"size_bytes" gorm:"default:0"`
	Metadata    datatypes.JSON   `json:"metadata"`
	CreatedAt   time.Time        `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time        `json:"updated_at" gorm:"not null"`
	LastUsedAt  *time.Time       `json:"last_used_at"`

	// Computed fields
	ActiveSessions int `json:"active_sessions" gorm:"-"`
	TotalSessions  int `json:"total_sessions" gorm:"-"`
}

func (Profile) TableName() string {
	return "profiles"
}

// BeforeCreate hook to ensure UUID is set and defaults are applied
func (p *Profile) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}

	if p.Metadata == nil {
		p.Metadata = datatypes.JSON("{}")
	}

	return nil
}

// BeforeUpdate hook to update the UpdatedAt timestamp
func (p *Profile) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	return nil
}

// CreateProfileRequest represents a request to create a new profile
type CreateProfileRequest struct {
	Name        string           `json:"name" binding:"required,min=1,max=255"`
	Description string           `json:"description" binding:"max=1000"`
	Browser     sessions.Browser `json:"browser" binding:"required,oneof=chrome chromium firefox edge webkit safari"`
}

// UpdateProfileRequest represents a request to update profile metadata
type UpdateProfileRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=1000"`
}

// ProfileListResponse represents a paginated list of profiles
type ProfileListResponse struct {
	Profiles []Profile `json:"profiles"`
	Total    int       `json:"total"`
	Offset   int       `json:"offset"`
	Limit    int       `json:"limit"`
}

// ProfileImportRequest represents a request to import a profile
type ProfileImportRequest struct {
	Name        string           `form:"name" binding:"required,min=1,max=255"`
	Description string           `form:"description" binding:"max=1000"`
	Browser     sessions.Browser `form:"browser" binding:"required,oneof=chrome chromium firefox edge webkit safari"`
}
