// cmd/browsermux/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"browsermux/internal/api"
	"browsermux/internal/browser"
	"browsermux/internal/config"
	"browsermux/internal/events"
	"browsermux/internal/webhook"
)

func main() {
	log.Println("Starting BrowserFleet CDP Proxy...")

	// Load configuration
	cfg := loadConfig()

	// Create event dispatcher
	dispatcher := events.NewDispatcher()

	// Set up webhook system
	webhookManager := webhook.NewManager(dispatcher)

	// Register webhook event handlers
	registerWebhookHandlers(dispatcher, webhookManager)

	// Create and configure CDP Proxy
	cdpProxyConfig := browser.CDPProxyConfig{
		BrowserURL:        cfg.BrowserURL,
		MaxMessageSize:    cfg.MaxMessageSize,
		ConnectionTimeout: time.Duration(cfg.ConnectionTimeoutSeconds) * time.Second,
	}

	cdpProxy, err := browser.NewCDPProxy(dispatcher, cdpProxyConfig)
	if err != nil {
		log.Fatalf("Failed to create CDP Proxy: %v", err)
	}

	// Create and start API server
	server := api.NewServer(cdpProxy, webhookManager, dispatcher, cfg.Port)

	// Run server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the API server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	// Shutdown the CDP Proxy
	if err := cdpProxy.Shutdown(); err != nil {
		log.Fatalf("CDP Proxy shutdown failed: %v", err)
	}

	log.Println("Server gracefully stopped")
}

// loadConfig loads the application configuration
func loadConfig() *config.Config {
	// Try to load from file or environment variables
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		return config.DefaultConfig()
	}
	return cfg
}

// registerWebhookHandlers registers event handlers for webhook triggers
func registerWebhookHandlers(dispatcher events.Dispatcher, manager webhook.Manager) {
	// Handle CDP commands for before_event webhooks
	dispatcher.Register(events.EventCDPCommand, func(event events.Event) {
		manager.TriggerWebhooks(
			event.Method,
			event.Params,
			event.SourceID, // This is now clientID instead of sessionID
			webhook.WebhookTimingBeforeEvent,
		)
	})

	// Handle CDP events for after_event webhooks
	dispatcher.Register(events.EventCDPEvent, func(event events.Event) {
		// For browser events, the clientID can be empty (broadcast to all)
		// Or we can extract the target client from the event parameters if needed
		clientID := ""
		if event.SourceType == "client" {
			clientID = event.SourceID
		}

		manager.TriggerWebhooks(
			event.Method,
			event.Params,
			clientID,
			webhook.WebhookTimingAfterEvent,
		)
	})

	// You can add more event handlers for client connection/disconnection events
	dispatcher.Register(events.EventClientConnected, func(event events.Event) {
		clientID := ""
		if id, ok := event.Params["client_id"].(string); ok {
			clientID = id
		}

		// You could trigger webhooks for client connection events if desired
		manager.TriggerWebhooks(
			"client.connected",
			event.Params,
			clientID,
			webhook.WebhookTimingAfterEvent,
		)
	})
}
