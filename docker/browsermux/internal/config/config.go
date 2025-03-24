package config

import (
	"encoding/json"
	"os"
	"strconv"
)

type Config struct {
	// Server settings
	Port string `json:"port"`

	// Browser settings
	BrowserURL               string `json:"browser_url"` // Base URL for browser, WebSocket URL will be fetched from /json/version
	MaxMessageSize           int    `json:"max_message_size"`
	ConnectionTimeoutSeconds int    `json:"connection_timeout_seconds"`
}

// Load loads configuration from environment variables or a file
func Load() (*Config, error) {
	config := &Config{}

	// Try to load from file first
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		file, err := os.Open(configPath)
		if err == nil {
			defer file.Close()

			decoder := json.NewDecoder(file)
			if err := decoder.Decode(config); err == nil {
				return config, nil
			}
		}
	}

	// Fall back to environment variables
	if port := os.Getenv("PORT"); port != "" {
		config.Port = port
	} else {
		config.Port = "8080" // Default
	}

	if browserURL := os.Getenv("BROWSER_URL"); browserURL != "" {
		config.BrowserURL = browserURL
	} else {
		config.BrowserURL = "http://localhost:9222" // Default (base URL, not WebSocket URL)
	}

	if maxSize := os.Getenv("MAX_MESSAGE_SIZE"); maxSize != "" {
		if size, err := strconv.Atoi(maxSize); err == nil {
			config.MaxMessageSize = size
		} else {
			config.MaxMessageSize = 1024 * 1024 // Default 1MB
		}
	} else {
		config.MaxMessageSize = 1024 * 1024 // Default 1MB
	}

	if timeout := os.Getenv("CONNECTION_TIMEOUT_SECONDS"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.ConnectionTimeoutSeconds = t
		} else {
			config.ConnectionTimeoutSeconds = 10 // Default 10 seconds
		}
	} else {
		config.ConnectionTimeoutSeconds = 10 // Default 10 seconds
	}

	return config, nil
}

func DefaultConfig() *Config {
	return &Config{
		Port:                     "8080",
		BrowserURL:               "http://localhost:9222", // Base URL, not WebSocket URL
		MaxMessageSize:           1024 * 1024,             // 1MB
		ConnectionTimeoutSeconds: 10,
	}
}
