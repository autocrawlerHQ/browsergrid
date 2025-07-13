package profiles

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Profile{}, &sessions.Session{})
	require.NoError(t, err)

	return db
}

func TestStore_CreateProfile(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for unit testing",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, profile.ID)
	assert.NotZero(t, profile.CreatedAt)
	assert.NotZero(t, profile.UpdatedAt)
}

func TestStore_GetProfile(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for unit testing",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	retrieved, err := store.GetProfile(ctx, profile.ID)
	require.NoError(t, err)

	assert.Equal(t, profile.ID, retrieved.ID)
	assert.Equal(t, profile.Name, retrieved.Name)
	assert.Equal(t, profile.Description, retrieved.Description)
	assert.Equal(t, profile.Browser, retrieved.Browser)
}

func TestStore_GetProfileByName(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for unit testing",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	retrieved, err := store.GetProfileByName(ctx, "test-profile")
	require.NoError(t, err)

	assert.Equal(t, profile.ID, retrieved.ID)
	assert.Equal(t, profile.Name, retrieved.Name)
}

func TestStore_ListProfiles(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test profiles
	profiles := []*Profile{
		{Name: "chrome-profile", Browser: sessions.BrowserChrome},
		{Name: "firefox-profile", Browser: sessions.BrowserFirefox},
		{Name: "chrome-profile-2", Browser: sessions.BrowserChrome},
	}

	for _, p := range profiles {
		err := store.CreateProfile(ctx, p)
		require.NoError(t, err)
	}

	// Test listing all profiles
	allProfiles, total, err := store.ListProfiles(ctx, nil, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, allProfiles, 3)

	// Test filtering by browser
	browserChrome := sessions.BrowserChrome
	chromeProfiles, total, err := store.ListProfiles(ctx, &browserChrome, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, chromeProfiles, 2)

	for _, p := range chromeProfiles {
		assert.Equal(t, sessions.BrowserChrome, p.Browser)
	}

	// Test pagination
	paginatedProfiles, total, err := store.ListProfiles(ctx, nil, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, paginatedProfiles, 2)
}

func TestStore_UpdateProfile(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Original description",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	originalUpdatedAt := profile.UpdatedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	// Update profile
	updates := map[string]interface{}{
		"description": "Updated description",
		"size_bytes":  int64(1024),
	}

	err = store.UpdateProfile(ctx, profile.ID, updates)
	require.NoError(t, err)

	// Retrieve updated profile
	updated, err := store.GetProfile(ctx, profile.ID)
	require.NoError(t, err)

	assert.Equal(t, "Updated description", updated.Description)
	assert.Equal(t, int64(1024), updated.SizeBytes)
	assert.True(t, updated.UpdatedAt.After(originalUpdatedAt))
}

func TestStore_DeleteProfile(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for deletion",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	// Delete profile
	err = store.DeleteProfile(ctx, profile.ID)
	require.NoError(t, err)

	// Verify profile is deleted
	_, err = store.GetProfile(ctx, profile.ID)
	assert.Error(t, err)
}

func TestStore_UpdateProfileSize(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:    "test-profile",
		Browser: sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	// Update size
	err = store.UpdateProfileSize(ctx, profile.ID, 2048)
	require.NoError(t, err)

	// Verify size is updated
	updated, err := store.GetProfile(ctx, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2048), updated.SizeBytes)
}

func TestStore_UpdateProfileLastUsed(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	profile := &Profile{
		Name:    "test-profile",
		Browser: sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	// Get the actual created profile from database to ensure we have the right timestamp
	created, err := store.GetProfile(ctx, profile.ID)
	require.NoError(t, err)

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update last used
	err = store.UpdateProfileLastUsed(ctx, profile.ID)
	require.NoError(t, err)

	// Verify last used is updated
	updated, err := store.GetProfile(ctx, profile.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.LastUsedAt)
	assert.True(t, updated.LastUsedAt.After(created.CreatedAt))
}
