package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type Service struct {
	db       *gorm.DB
	server   *asynq.Server
	client   *asynq.Client
	mux      *asynq.ServeMux
	redisOpt asynq.RedisClientOpt
}

func New(db *gorm.DB, redisOpt asynq.RedisClientOpt) *Service {
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"scheduler": 10,
				"low":       1,
			},
		},
	)

	client := asynq.NewClient(redisOpt)
	mux := asynq.NewServeMux()

	s := &Service{
		db:       db,
		server:   srv,
		client:   client,
		mux:      mux,
		redisOpt: redisOpt,
	}

	mux.HandleFunc(tasks.TypePoolScale, s.handlePoolScale)
	mux.HandleFunc(tasks.TypeCleanupExpired, s.handleCleanupExpired)

	return s
}

func (s *Service) Start() error {
	log.Println("[SCHEDULER] Starting scheduler service...")
	return s.server.Run(s.mux)
}

func (s *Service) Stop() {
	log.Println("[SCHEDULER] Stopping scheduler service...")
	s.server.Shutdown()
	s.client.Close()
}

func (s *Service) handlePoolScale(ctx context.Context, t *asynq.Task) error {
	var payload tasks.PoolScalePayload
	if err := payload.Unmarshal(t.Payload()); err != nil {
		return err
	}

	log.Printf("[SCHEDULER] Processing pool scale task for pool %s (desired: %d sessions)",
		payload.WorkPoolID, payload.DesiredSessions)

	var pool workpool.WorkPool
	if err := s.db.WithContext(ctx).First(&pool, "id = ?", payload.WorkPoolID).Error; err != nil {
		return err
	}

	sessStore := sessions.NewStore(s.db)
	created := 0

	for i := 0; i < payload.DesiredSessions; i++ {
		sess := s.createSessionFromPool(&pool)
		if err := sessStore.CreateSession(ctx, sess); err != nil {
			log.Printf("[SCHEDULER] Failed to create session: %v", err)
			continue
		}

		startPayload := tasks.SessionStartPayload{
			SessionID:          sess.ID,
			WorkPoolID:         pool.ID,
			MaxSessionDuration: pool.MaxSessionDuration,
			RedisAddr:          s.redisOpt.Addr,
			QueueName:          getQueueName(pool.Provider),
		}

		startTask, err := tasks.NewSessionStartTask(startPayload)
		if err != nil {
			log.Printf("[SCHEDULER] Failed to create start task: %v", err)
			continue
		}

		info, err := s.client.Enqueue(startTask,
			asynq.Queue(startPayload.QueueName),
			asynq.MaxRetry(3),
		)
		if err != nil {
			log.Printf("[SCHEDULER] Failed to enqueue start task: %v", err)
			sessStore.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
			continue
		}

		log.Printf("[SCHEDULER] Created session %s and enqueued task %s", sess.ID, info.ID)
		created++
	}

	log.Printf("[SCHEDULER] Pool scale completed: %d/%d sessions created for pool %s",
		created, payload.DesiredSessions, pool.Name)

	return nil
}

func (s *Service) handleCleanupExpired(ctx context.Context, t *asynq.Task) error {
	var payload tasks.CleanupExpiredPayload
	if err := payload.Unmarshal(t.Payload()); err != nil {
		return err
	}

	log.Printf("[SCHEDULER] Cleaning up sessions older than %d hours", payload.MaxAge)

	cutoff := time.Now().Add(-time.Duration(payload.MaxAge) * time.Hour)

	terminalStatuses := []sessions.SessionStatus{
		sessions.StatusCompleted,
		sessions.StatusFailed,
		sessions.StatusExpired,
		sessions.StatusCrashed,
		sessions.StatusTimedOut,
		sessions.StatusTerminated,
	}

	result := s.db.WithContext(ctx).
		Where("status IN ? AND updated_at < ?", terminalStatuses, cutoff).
		Delete(&sessions.Session{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("[SCHEDULER] Cleaned up %d expired sessions", result.RowsAffected)
	}

	s.db.Exec(`DELETE FROM session_events WHERE session_id NOT IN (SELECT id FROM sessions)`)
	s.db.Exec(`DELETE FROM session_metrics WHERE session_id NOT IN (SELECT id FROM sessions)`)

	return nil
}

func (s *Service) createSessionFromPool(pool *workpool.WorkPool) *sessions.Session {
	env := pool.DefaultEnv
	if env == nil {
		envData, _ := json.Marshal(map[string]string{})
		env = datatypes.JSON(envData)
	}

	sess := &sessions.Session{
		ID:              uuid.New(),
		Browser:         sessions.BrowserChrome,
		Version:         sessions.VerLatest,
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Environment: env,
		Status:      sessions.StatusPending,
		Provider:    string(pool.Provider),
		WorkPoolID:  &pool.ID,
	}

	if pool.DefaultImage != nil {
		var envMap map[string]string
		if err := json.Unmarshal(sess.Environment, &envMap); err != nil {
			envMap = make(map[string]string)
		}
		envMap["BROWSER_IMAGE"] = *pool.DefaultImage

		envData, _ := json.Marshal(envMap)
		sess.Environment = datatypes.JSON(envData)
	}

	return sess
}

func getQueueName(provider workpool.ProviderType) string {
	switch provider {
	case workpool.ProviderDocker:
		return "default"
	case workpool.ProviderACI:
		return "azure"
	case workpool.ProviderLocal:
		return "local"
	default:
		return "default"
	}
}
