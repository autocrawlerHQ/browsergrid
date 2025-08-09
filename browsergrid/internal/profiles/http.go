package profiles

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

// Dependencies holds the dependencies for profile handlers
type Dependencies struct {
	DB           *gorm.DB
	Store        *Store
	ProfileStore ProfileStore
}

// RegisterRoutes registers profile management routes
func RegisterRoutes(rg *gin.RouterGroup, deps Dependencies) {
	rg.POST("/profiles", createProfile(deps))
	rg.GET("/profiles", listProfiles(deps))
	rg.GET("/profiles/:id", getProfile(deps))
	rg.PATCH("/profiles/:id", updateProfile(deps))
	rg.DELETE("/profiles/:id", deleteProfile(deps))
	rg.POST("/profiles/import", importProfile(deps))
	rg.GET("/profiles/:id/export", exportProfile(deps))
}

// createProfile creates a new profile
// @Summary Create a new profile
// @Description Create a new browser profile for saving and reusing browser state
// @Tags profiles
// @Accept json
// @Produce json
// @Param profile body CreateProfileRequest true "Profile configuration"
// @Success 201 {object} Profile "Profile created successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/profiles [post]
func createProfile(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if name is unique
		existing, err := deps.Store.GetProfileByName(c.Request.Context(), req.Name)
		if err == nil && existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "profile with this name already exists"})
			return
		}

		profile := &Profile{
			Name:        req.Name,
			Description: req.Description,
			Browser:     req.Browser,
		}

		// Create profile in database
		if err := deps.Store.CreateProfile(c.Request.Context(), profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Initialize profile storage
		if err := deps.ProfileStore.InitializeProfile(c.Request.Context(), profile.ID.String()); err != nil {
			// Rollback database creation
			deps.DB.Delete(profile)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize profile storage: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, profile)
	}
}

// listProfiles lists all profiles
// @Summary List profiles
// @Description Get a paginated list of browser profiles
// @Tags profiles
// @Accept json
// @Produce json
// @Param browser query string false "Filter by browser type" Enums(chrome,chromium,firefox,edge,webkit,safari)
// @Param offset query integer false "Number of profiles to skip" default(0) minimum(0)
// @Param limit query integer false "Maximum number of profiles to return" default(20) minimum(1) maximum(100)
// @Success 200 {object} ProfileListResponse "List of profiles"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/profiles [get]
func listProfiles(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var browser *sessions.Browser
		if b := c.Query("browser"); b != "" {
			browserType := sessions.Browser(b)
			browser = &browserType
		}

		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		if limit > 100 {
			limit = 100
		}

		profiles, total, err := deps.Store.ListProfiles(c.Request.Context(), browser, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, ProfileListResponse{
			Profiles: profiles,
			Total:    total,
			Offset:   offset,
			Limit:    limit,
		})
	}
}

// getProfile retrieves a specific profile
// @Summary Get a profile
// @Description Get detailed information about a specific profile
// @Tags profiles
// @Accept json
// @Produce json
// @Param id path string true "Profile ID (UUID)"
// @Success 200 {object} Profile "Profile details"
// @Failure 400 {object} ErrorResponse "Invalid profile ID"
// @Failure 404 {object} ErrorResponse "Profile not found"
// @Router /api/v1/profiles/{id} [get]
func getProfile(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile ID"})
			return
		}

		profile, err := deps.Store.GetProfile(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, profile)
	}
}

// updateProfile updates profile metadata
// @Summary Update a profile
// @Description Update profile metadata (name, description)
// @Tags profiles
// @Accept json
// @Produce json
// @Param id path string true "Profile ID (UUID)"
// @Param updates body UpdateProfileRequest true "Fields to update"
// @Success 200 {object} Profile "Updated profile"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Profile not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id} [patch]
func updateProfile(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile ID"})
			return
		}

		var req UpdateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if profile exists first
		_, err = deps.Store.GetProfile(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		// Build updates map
		updates := make(map[string]interface{})
		if req.Name != nil {
			// Check if new name is unique
			existing, _ := deps.Store.GetProfileByName(c.Request.Context(), *req.Name)
			if existing != nil && existing.ID != id {
				c.JSON(http.StatusConflict, gin.H{"error": "profile with this name already exists"})
				return
			}
			updates["name"] = *req.Name
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}

		if len(updates) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no updates provided"})
			return
		}

		if err := deps.Store.UpdateProfile(c.Request.Context(), id, updates); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Get updated profile
		profile, err := deps.Store.GetProfile(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, profile)
	}
}

// deleteProfile deletes a profile
// @Summary Delete a profile
// @Description Delete a profile and its associated data. Cannot delete profiles with active sessions.
// @Tags profiles
// @Accept json
// @Produce json
// @Param id path string true "Profile ID (UUID)"
// @Success 200 {object} MessageResponse "Profile deleted successfully"
// @Failure 400 {object} ErrorResponse "Invalid profile ID or profile has active sessions"
// @Failure 404 {object} ErrorResponse "Profile not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id} [delete]
func deleteProfile(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile ID"})
			return
		}

		// Check if profile exists
		profile, err := deps.Store.GetProfile(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		// Delete from database (will check for active sessions)
		if err := deps.Store.DeleteProfile(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Delete profile storage
		if err := deps.ProfileStore.DeleteProfile(c.Request.Context(), profile.ID.String()); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to delete profile storage: %v\n", err)
		}

		c.JSON(http.StatusOK, gin.H{"message": "profile deleted successfully"})
	}
}

// importProfile imports a profile from a ZIP file
// @Summary Import a profile
// @Description Import a browser profile from a ZIP archive
// @Tags profiles
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Profile ZIP archive"
// @Param name formData string true "Profile name"
// @Param description formData string false "Profile description"
// @Param browser formData string true "Browser type" Enums(chrome,chromium,firefox,edge,webkit,safari)
// @Success 201 {object} Profile "Profile imported successfully"
// @Failure 400 {object} ErrorResponse "Invalid request or file"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/profiles/import [post]
func importProfile(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ProfileImportRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get uploaded file
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
			return
		}
		defer file.Close()

		// Validate file size (max 1GB)
		if header.Size > maxProfileSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file size exceeds maximum of %d bytes", maxProfileSize)})
			return
		}

		// Check if name is unique
		existing, err := deps.Store.GetProfileByName(c.Request.Context(), req.Name)
		if err == nil && existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "profile with this name already exists"})
			return
		}

		profile := &Profile{
			Name:        req.Name,
			Description: req.Description,
			Browser:     req.Browser,
		}

		// Create profile in database
		if err := deps.Store.CreateProfile(c.Request.Context(), profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Import profile data
		if err := deps.ProfileStore.ImportProfile(c.Request.Context(), profile.ID.String(), file); err != nil {
			// Rollback database creation
			deps.DB.Delete(profile)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to import profile: " + err.Error()})
			return
		}

		// Update profile size
		size, _ := deps.ProfileStore.GetProfileSize(c.Request.Context(), profile.ID.String())
		deps.Store.UpdateProfileSize(c.Request.Context(), profile.ID, size)

		// Get updated profile
		profile, _ = deps.Store.GetProfile(c.Request.Context(), profile.ID)
		c.JSON(http.StatusCreated, profile)
	}
}

// exportProfile exports a profile as a ZIP file
// @Summary Export a profile
// @Description Export a browser profile as a ZIP archive
// @Tags profiles
// @Accept json
// @Produce application/zip
// @Param id path string true "Profile ID (UUID)"
// @Success 200 {file} binary "Profile ZIP archive"
// @Failure 400 {object} ErrorResponse "Invalid profile ID"
// @Failure 404 {object} ErrorResponse "Profile not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id}/export [get]
func exportProfile(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile ID"})
			return
		}

		// Check if profile exists
		profile, err := deps.Store.GetProfile(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		// Export profile data
		reader, err := deps.ProfileStore.ExportProfile(c.Request.Context(), profile.ID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export profile: " + err.Error()})
			return
		}
		defer reader.Close()

		// Set headers for file download
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", profile.Name))

		// Stream the file
		if _, err := io.Copy(c.Writer, reader); err != nil {
			// Can't send error response after headers are sent
			fmt.Printf("Failed to stream profile export: %v\n", err)
		}
	}
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// MessageResponse represents a success message response
type MessageResponse struct {
	Message string `json:"message"`
}
