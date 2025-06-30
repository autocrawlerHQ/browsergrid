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
	log.Println("Starting Browsergrid CDP Proxy...")

	cfg := loadConfig()

	dispatcher := events.NewDispatcher()

	webhookManager := webhook.NewManager(dispatcher)

	registerWebhookHandlers(dispatcher, webhookManager)

	cdpProxyConfig := browser.CDPProxyConfig{
		BrowserURL:        cfg.BrowserURL,
		MaxMessageSize:    cfg.MaxMessageSize,
		ConnectionTimeout: time.Duration(cfg.ConnectionTimeoutSeconds) * time.Second,
	}

	cdpProxy, err := browser.NewCDPProxy(dispatcher, cdpProxyConfig)
	if err != nil {
		log.Fatalf("Failed to create CDP Proxy: %v", err)
	}

	server := api.NewServer(cdpProxy, webhookManager, dispatcher, cfg.Port, cfg)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	if err := cdpProxy.Shutdown(); err != nil {
		log.Fatalf("CDP Proxy shutdown failed: %v", err)
	}

	log.Println("Server gracefully stopped")
}

func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		return config.DefaultConfig()
	}
	return cfg
}

func registerWebhookHandlers(dispatcher events.Dispatcher, manager webhook.Manager) {
	dispatcher.Register(events.EventCDPCommand, func(event events.Event) {
		manager.TriggerWebhooks(
			event.Method,
			event.Params,
			event.SourceID,
			webhook.WebhookTimingBeforeEvent,
		)
	})

	dispatcher.Register(events.EventCDPEvent, func(event events.Event) {
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

	dispatcher.Register(events.EventClientConnected, func(event events.Event) {
		clientID := ""
		if id, ok := event.Params["client_id"].(string); ok {
			clientID = id
		}

		manager.TriggerWebhooks(
			"client.connected",
			event.Params,
			clientID,
			webhook.WebhookTimingAfterEvent,
		)
	})
}
