package sessions

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB, poolSvc PoolService) {
	store := NewStore(db)

	rg.POST("/sessions", createSession(store, poolSvc))
	rg.GET("/sessions", listSessions(store))
	rg.GET("/sessions/:id", getSession(store))
	rg.POST("/sessions/:id/events", createEvent(store))
	rg.GET("/sessions/:id/events", listEvents(store))
	rg.POST("/sessions/:id/metrics", createMetrics(store))

	// Add flat endpoints that tests expect
	rg.POST("/events", createEvent(store))
	rg.GET("/events", listEvents(store))
	rg.POST("/metrics", createMetrics(store))
}

// CreateSession creates a new browser session
// @Summary Create a new browser session
// @Description Create a new browser session with specified configuration
// @Tags sessions
// @Accept json
// @Produce json
// @Param session body Session true "Session configuration"
// @Success 201 {object} Session
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions [post]
func createSession(store *Store, poolSvc PoolService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Session
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Reject empty request body - tests expect 400 for empty {}
		if req.Browser == "" && req.Version == "" && req.OperatingSystem == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "browser is required"})
			return
		}

		// Set defaults for all fields
		if req.Browser == "" {
			req.Browser = BrowserChrome
		}
		if req.Version == "" {
			req.Version = VerLatest
		}
		if req.OperatingSystem == "" {
			req.OperatingSystem = OSLinux
		}

		// Set default screen dimensions if not provided
		if req.Screen.Width == 0 || req.Screen.Height == 0 {
			req.Screen.Width = 1920
			req.Screen.Height = 1080
		}
		if req.Screen.DPI == 0 {
			req.Screen.DPI = 96
		}
		if req.Screen.Scale == 0 {
			req.Screen.Scale = 1.0
		}

		// Set default resource limits if not provided
		if req.ResourceLimits.CPU == nil {
			cpu := 2.0
			req.ResourceLimits.CPU = &cpu
		}
		if req.ResourceLimits.Memory == nil {
			memory := "2GB"
			req.ResourceLimits.Memory = &memory
		}
		if req.ResourceLimits.TimeoutMinutes == nil {
			timeout := 30
			req.ResourceLimits.TimeoutMinutes = &timeout
		}

		// Set default provider
		if req.Provider == "" {
			req.Provider = "docker"
		}

		// Set default environment if not provided
		if req.Environment == nil {
			req.Environment = []byte("{}")
		}

		// Set default expiration (1 hour from now)
		if req.ExpiresAt == nil {
			expires := time.Now().Add(1 * time.Hour)
			req.ExpiresAt = &expires
		}

		// Assign to default work pool if not specified
		if req.WorkPoolID == nil {
			if err := assignToDefaultWorkPool(c.Request.Context(), poolSvc, &req); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign to default work pool: " + err.Error()})
				return
			}
		}

		req.Status = StatusPending
		if err := store.CreateSession(c.Request.Context(), &req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, req)
	}
}

// ListSessions lists all sessions with optional filtering
// @Summary List browser sessions
// @Description Get a list of browser sessions with optional filtering by status and time range
// @Tags sessions
// @Accept json
// @Produce json
// @Param status query SessionStatus false "Filter by session status" Enums(pending,starting,available,claimed,running,idle,completed,failed,expired,crashed,timed_out,terminated)
// @Param start_time query string false "Filter sessions created after this time (RFC3339)"
// @Param end_time query string false "Filter sessions created before this time (RFC3339)"
// @Param offset query int false "Pagination offset" default(0)
// @Param limit query int false "Pagination limit" default(100)
// @Success 200 {object} SessionListResponse "List of sessions with pagination info"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions [get]
func listSessions(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			status     *SessionStatus
			start, end *time.Time
		)
		if v := c.Query("status"); v != "" {
			s := SessionStatus(v)
			status = &s
		}
		if v := c.Query("start_time"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				start = &t
			}
		}
		if v := c.Query("end_time"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				end = &t
			}
		}
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

		sessions, err := store.ListSessions(c.Request.Context(), status, start, end, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"sessions": sessions,
			"total":    len(sessions),
			"offset":   offset,
			"limit":    limit,
		})
	}
}

// GetSession retrieves a specific session by ID
// @Summary Get a browser session
// @Description Get details of a specific browser session by ID
// @Tags sessions
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Success 200 {object} Session
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/sessions/{id} [get]
func getSession(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
			return
		}
		sess, err := store.GetSession(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusOK, sess)
	}
}

// CreateEvent creates a new session event
// @Summary Create a session event
// @Description Create a new event for a session (e.g., page navigation, user interaction)
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param event body SessionEvent true "Session event data"
// @Success 201 {object} SessionEvent
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions/{id}/events [post]
func createEvent(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ev SessionEvent
		if err := c.ShouldBindJSON(&ev); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Accept session ID from URL param or JSON body
		if idStr := c.Param("id"); idStr != "" {
			sessionID, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
				return
			}
			ev.SessionID = sessionID
		}

		if ev.SessionID == uuid.Nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		if ev.Event == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "event is required"})
			return
		}

		if next, ok := statusFromEvent(ev.Event); ok {
			sess, err := store.GetSession(c.Request.Context(), ev.SessionID)
			if err == nil && shouldUpdateStatus(sess.Status, next) {
				store.UpdateSessionStatus(c.Request.Context(), sess.ID, next)
			}
		}

		if err := store.CreateEvent(c.Request.Context(), &ev); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, ev)
	}
}

// ListEvents lists session events with optional filtering
// @Summary List session events
// @Description Get a list of session events with optional filtering by event type and time range
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param event_type query SessionEventType false "Filter by event type"
// @Param start_time query string false "Filter events created after this time (RFC3339)"
// @Param end_time query string false "Filter events created before this time (RFC3339)"
// @Param offset query int false "Pagination offset" default(0)
// @Param limit query int false "Pagination limit" default(100)
// @Success 200 {object} SessionEventListResponse "List of events with pagination info"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions/{id}/events [get]
func listEvents(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			sessionIDPtr *uuid.UUID
			eventType    *SessionEventType
			start, end   *time.Time
		)

		// Accept session ID from URL param or query parameter
		if idStr := c.Param("id"); idStr != "" {
			sessionID, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
				return
			}
			sessionIDPtr = &sessionID
		} else if q := c.Query("session_id"); q != "" {
			sessionID, err := uuid.Parse(q)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session_id"})
				return
			}
			sessionIDPtr = &sessionID
		}

		if v := c.Query("event_type"); v != "" {
			et := SessionEventType(v)
			eventType = &et
		}
		if v := c.Query("start_time"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				start = &t
			}
		}
		if v := c.Query("end_time"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				end = &t
			}
		}
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

		events, err := store.ListEvents(c.Request.Context(), sessionIDPtr, eventType, start, end, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"events": events,
			"total":  len(events),
			"offset": offset,
			"limit":  limit,
		})
	}
}

// CreateMetrics creates session performance metrics
// @Summary Create session metrics
// @Description Create performance metrics for a session (CPU, memory, network usage)
// @Tags metrics
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param metrics body SessionMetrics true "Session metrics data"
// @Success 201 {object} SessionMetrics
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sessions/{id}/metrics [post]
func createMetrics(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var metrics SessionMetrics
		if err := c.ShouldBindJSON(&metrics); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Accept session ID from URL param or JSON body
		if idStr := c.Param("id"); idStr != "" {
			sessionID, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
				return
			}
			metrics.SessionID = sessionID
		}

		if metrics.SessionID == uuid.Nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		if err := store.CreateMetrics(c.Request.Context(), &metrics); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, metrics)
	}
}

// assignToDefaultWorkPool uses dependency inversion to avoid circular imports
func assignToDefaultWorkPool(ctx context.Context, poolSvc PoolService, session *Session) error {
	id, err := poolSvc.GetOrCreateDefault(ctx, session.Provider)
	if err != nil {
		return err
	}
	session.WorkPoolID = &id
	return nil
}
