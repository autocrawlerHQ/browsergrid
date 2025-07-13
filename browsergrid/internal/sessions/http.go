package sessions

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
)

// ProfileService defines the interface for profile operations that sessions need
type ProfileService interface {
	// ValidateProfile checks if a profile exists and is valid for the given browser
	ValidateProfile(ctx context.Context, profileID uuid.UUID, browser Browser) error
}

// ProfileStore defines the interface for profile storage validation
type ProfileStore interface {
	// ValidateProfileForBrowser validates that a profile exists and is compatible with the browser type
	ValidateProfileForBrowser(ctx context.Context, profileID uuid.UUID, browser Browser) error
}

type Dependencies struct {
	DB           *gorm.DB
	PoolSvc      PoolService
	TaskClient   *asynq.Client
	ProfileSvc   ProfileService
	ProfileStore ProfileStore
}

func RegisterRoutes(rg *gin.RouterGroup, deps Dependencies) {
	store := NewStore(deps.DB)

	rg.POST("/sessions", createSession(store, deps))
	rg.GET("/sessions", listSessions(store))
	rg.GET("/sessions/:id", getSession(store))
	rg.DELETE("/sessions/:id", deleteSession(store, deps))
	rg.POST("/sessions/:id/events", createEvent(store))
	rg.GET("/sessions/:id/events", listEvents(store))
	rg.POST("/sessions/:id/metrics", createMetrics(store))

	rg.POST("/events", createEvent(store))
	rg.GET("/events", listEvents(store))
	rg.POST("/metrics", createMetrics(store))
}

// CreateSession creates a new browser session and enqueues a start task
// @Summary Create a new browser session
// @Description Create a new browser session with specified configuration. The session will be created in pending status and a start task will be enqueued.
// @Tags sessions
// @Accept json
// @Produce json
// @Param session body Session true "Session configuration"
// @Success 201 {object} Session "Session created successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sessions [post]
func createSession(store *Store, deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Session
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Browser == "" && req.Version == "" && req.OperatingSystem == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "browser is required"})
			return
		}

		if req.Browser == "" {
			req.Browser = BrowserChrome
		}
		if req.Version == "" {
			req.Version = VerLatest
		}
		if req.OperatingSystem == "" {
			req.OperatingSystem = OSLinux
		}

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

		if req.Provider == "" {
			req.Provider = "docker"
		}

		if req.Environment == nil {
			req.Environment = []byte("{}")
		}

		if req.ExpiresAt == nil {
			expires := time.Now().Add(1 * time.Hour)
			req.ExpiresAt = &expires
		}

		// Validate profile if provided
		if req.ProfileID != nil {
			if deps.ProfileStore != nil {
				if err := deps.ProfileStore.ValidateProfileForBrowser(c.Request.Context(), *req.ProfileID, req.Browser); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile: " + err.Error()})
					return
				}
			} else if deps.ProfileSvc != nil {
				if err := deps.ProfileSvc.ValidateProfile(c.Request.Context(), *req.ProfileID, req.Browser); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile: " + err.Error()})
					return
				}
			}
		}

		if req.WorkPoolID == nil {
			if err := assignToDefaultWorkPool(c.Request.Context(), deps.PoolSvc, &req); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign to default work pool: " + err.Error()})
				return
			}
		}

		req.Status = StatusPending
		if err := store.CreateSession(c.Request.Context(), &req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		pool, err := getWorkPool(ctx, deps.DB, *req.WorkPoolID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get work pool: " + err.Error()})
			return
		}

		payload := tasks.SessionStartPayload{
			SessionID:          req.ID,
			WorkPoolID:         *req.WorkPoolID,
			MaxSessionDuration: pool.MaxSessionDuration,
			RedisAddr:          os.Getenv("REDIS_ADDR"),
			QueueName:          getQueueName(pool.Provider),
		}

		// Skip task enqueueing in tests when TaskClient is nil
		if deps.TaskClient != nil {
			task, err := tasks.NewSessionStartTask(payload)
			if err != nil {
				store.UpdateSessionStatus(ctx, req.ID, StatusFailed)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task: " + err.Error()})
				return
			}

			info, err := deps.TaskClient.Enqueue(task,
				asynq.Queue(payload.QueueName),
				asynq.MaxRetry(3),
				asynq.Timeout(5*time.Minute),
			)
			if err != nil {
				store.UpdateSessionStatus(ctx, req.ID, StatusFailed)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue task: " + err.Error()})
				return
			}

			log.Printf("[API] Created session %s and enqueued task %s", req.ID, info.ID)
		} else {
			log.Printf("[API] Created session %s (task client disabled for testing)", req.ID)
		}

		event := SessionEvent{
			SessionID: req.ID,
			Event:     EvtSessionCreated,
			Timestamp: time.Now(),
		}
		store.CreateEvent(ctx, &event)

		c.JSON(http.StatusCreated, req)
	}
}

// DeleteSession stops a running session
// @Summary Delete a browser session
// @Description Stop and terminate a running browser session. If the session is already in a terminal state, it will return success immediately.
// @Tags sessions
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Success 200 {object} MessageResponse "Session termination initiated or already terminated"
// @Failure 400 {object} ErrorResponse "Invalid session ID"
// @Failure 404 {object} ErrorResponse "Session not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sessions/{id} [delete]
func deleteSession(store *Store, deps Dependencies) gin.HandlerFunc {
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

		if IsTerminalStatus(sess.Status) {
			c.JSON(http.StatusOK, gin.H{"message": "session already terminated"})
			return
		}

		payload := tasks.SessionStopPayload{
			SessionID: sess.ID,
			Reason:    "user_requested",
		}

		task, err := tasks.NewSessionStopTask(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create stop task: " + err.Error()})
			return
		}

		info, err := deps.TaskClient.Enqueue(task,
			asynq.Queue("critical"),
			asynq.MaxRetry(5),
			asynq.Timeout(2*time.Minute),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue stop task: " + err.Error()})
			return
		}

		log.Printf("[API] Enqueued stop task %s for session %s", info.ID, sess.ID)

		store.UpdateSessionStatus(c.Request.Context(), sess.ID, StatusTerminated)

		c.JSON(http.StatusOK, gin.H{"message": "session termination initiated"})
	}
}

// ListSessions lists all sessions with optional filtering
// @Summary List browser sessions
// @Description Get a list of browser sessions with optional filtering by status, time range, and pagination
// @Tags sessions
// @Accept json
// @Produce json
// @Param status query string false "Filter by session status" Enums(pending, starting, available, claimed, running, idle, completed, failed, expired, crashed, timed_out, terminated)
// @Param start_time query string false "Filter sessions created after this time (RFC3339 format)"
// @Param end_time query string false "Filter sessions created before this time (RFC3339 format)"
// @Param offset query integer false "Number of sessions to skip" default(0) minimum(0)
// @Param limit query integer false "Maximum number of sessions to return" default(100) minimum(1) maximum(1000)
// @Success 200 {object} SessionListResponse "List of sessions"
// @Failure 500 {object} ErrorResponse "Internal server error"
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
// @Description Get detailed information about a specific browser session by its ID
// @Tags sessions
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Success 200 {object} Session "Session details"
// @Failure 400 {object} ErrorResponse "Invalid session ID"
// @Failure 404 {object} ErrorResponse "Session not found"
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
// @Description Create a new event for a browser session. The session ID can be provided in the URL path or in the request body.
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param event body SessionEvent true "Event data"
// @Success 201 {object} SessionEvent "Event created successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data or missing session_id/event"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sessions/{id}/events [post]
// @Router /api/v1/events [post]
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

		if err := store.CreateEvent(c.Request.Context(), &ev); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update session status if event triggers a transition
		if newStatus, shouldUpdate := statusFromEvent(ev.Event); shouldUpdate {
			if err := store.UpdateSessionStatus(c.Request.Context(), ev.SessionID, newStatus); err != nil {
				// Log error but don't fail the request since event was created
				log.Printf("Failed to update session status for event %s: %v", ev.Event, err)
			}
		}

		c.JSON(http.StatusCreated, ev)
	}
}

// ListEvents lists session events with optional filtering
// @Summary List session events
// @Description Get a list of events for browser sessions with optional filtering by session ID, event type, time range, and pagination
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param session_id query string false "Session ID (UUID) - alternative to path parameter"
// @Param event_type query string false "Filter by event type" Enums(session_created, resource_allocated, session_starting, container_started, browser_started, session_available, session_claimed, session_assigned, session_ready, session_active, session_idle, heartbeat, pool_added, pool_removed, pool_drained, session_completed, session_expired, session_timed_out, session_terminated, startup_failed, browser_crashed, container_crashed, resource_exhausted, network_error, status_changed, config_updated, health_check)
// @Param start_time query string false "Filter events after this time (RFC3339 format)"
// @Param end_time query string false "Filter events before this time (RFC3339 format)"
// @Param offset query integer false "Number of events to skip" default(0) minimum(0)
// @Param limit query integer false "Maximum number of events to return" default(100) minimum(1) maximum(1000)
// @Success 200 {object} SessionEventListResponse "List of events"
// @Failure 400 {object} ErrorResponse "Invalid session ID or parameters"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sessions/{id}/events [get]
// @Router /api/v1/events [get]
func listEvents(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			sessionIDPtr *uuid.UUID
			eventType    *SessionEventType
			start, end   *time.Time
		)

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
// @Description Create performance metrics for a browser session. The session ID can be provided in the URL path or in the request body.
// @Tags metrics
// @Accept json
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param metrics body SessionMetrics true "Performance metrics data"
// @Success 201 {object} SessionMetrics "Metrics created successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data or missing session_id"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sessions/{id}/metrics [post]
// @Router /api/v1/metrics [post]
func createMetrics(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var metrics SessionMetrics
		if err := c.ShouldBindJSON(&metrics); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

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

func assignToDefaultWorkPool(ctx context.Context, poolSvc PoolService, session *Session) error {
	id, err := poolSvc.GetOrCreateDefault(ctx, session.Provider)
	if err != nil {
		return err
	}
	session.WorkPoolID = &id
	return nil
}

func getWorkPool(ctx context.Context, db *gorm.DB, id uuid.UUID) (*WorkPool, error) {
	var pool WorkPool
	err := db.WithContext(ctx).First(&pool, "id = ?", id).Error
	return &pool, err
}

func getQueueName(provider ProviderType) string {
	switch provider {
	case ProviderDocker:
		return "default"
	case ProviderACI:
		return "azure"
	case ProviderLocal:
		return "local"
	default:
		return "default"
	}
}

type WorkPool struct {
	ID                 uuid.UUID
	Provider           ProviderType
	MaxSessionDuration int
}

type ProviderType string

const (
	ProviderDocker ProviderType = "docker"
	ProviderACI    ProviderType = "azure_aci"
	ProviderLocal  ProviderType = "local"
)
