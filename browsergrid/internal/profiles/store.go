package profiles

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

// Store provides database operations for profiles
type Store struct {
	db *gorm.DB
}

// NewStore creates a new profile store
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// CreateProfile creates a new profile in the database
func (s *Store) CreateProfile(ctx context.Context, profile *Profile) error {
	return s.db.WithContext(ctx).Create(profile).Error
}

// GetProfile retrieves a profile by ID
func (s *Store) GetProfile(ctx context.Context, id uuid.UUID) (*Profile, error) {
	var profile Profile
	err := s.db.WithContext(ctx).First(&profile, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetProfileByName retrieves a profile by name
func (s *Store) GetProfileByName(ctx context.Context, name string) (*Profile, error) {
	var profile Profile
	err := s.db.WithContext(ctx).First(&profile, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// ListProfiles retrieves a paginated list of profiles with optional filtering
func (s *Store) ListProfiles(ctx context.Context, browser *sessions.Browser, offset, limit int) ([]Profile, int, error) {
	query := s.db.WithContext(ctx).Model(&Profile{})

	if browser != nil {
		query = query.Where("browser = ?", *browser)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var profiles []Profile
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&profiles).Error

	return profiles, int(total), err
}

// UpdateProfile updates profile fields
func (s *Store) UpdateProfile(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return s.db.WithContext(ctx).Model(&Profile{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteProfile deletes a profile from the database
func (s *Store) DeleteProfile(ctx context.Context, id uuid.UUID) error {
	// Check if profile has active sessions
	var count int64
	err := s.db.WithContext(ctx).Model(&sessions.Session{}).
		Where("profile_id = ? AND status NOT IN (?)", id, []sessions.SessionStatus{
			sessions.StatusCompleted,
			sessions.StatusFailed,
			sessions.StatusExpired,
			sessions.StatusCrashed,
			sessions.StatusTimedOut,
			sessions.StatusTerminated,
		}).
		Count(&count).Error

	if err != nil {
		return err
	}

	if count > 0 {
		return gorm.ErrInvalidData
	}

	return s.db.WithContext(ctx).Delete(&Profile{}, "id = ?", id).Error
}

// UpdateProfileSize updates the size of a profile
func (s *Store) UpdateProfileSize(ctx context.Context, id uuid.UUID, sizeBytes int64) error {
	return s.db.WithContext(ctx).Model(&Profile{}).
		Where("id = ?", id).
		Update("size_bytes", sizeBytes).Error
}

// UpdateProfileLastUsed updates the last used timestamp
func (s *Store) UpdateProfileLastUsed(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&Profile{}).
		Where("id = ?", id).
		Update("last_used_at", time.Now()).Error
}

// AutoMigrate runs database migrations for profiles
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Profile{})
}

// ValidateProfileForBrowser validates that a profile exists and is compatible with the browser type
func (s *Store) ValidateProfileForBrowser(ctx context.Context, profileID uuid.UUID, browser sessions.Browser) error {
	profile, err := s.GetProfile(ctx, profileID)
	if err != nil {
		return err
	}

	if profile.Browser != browser {
		return fmt.Errorf("profile browser %s does not match session browser %s", profile.Browser, browser)
	}

	return nil
}
