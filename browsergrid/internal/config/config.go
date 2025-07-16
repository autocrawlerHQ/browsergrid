package config

import (
	"os"
	"strconv"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
)

type Config struct {
	Port          int
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Storage       storage.Config // Add storage configuration
}

func Load() Config {
	cfg := Config{
		Port:        8765,
		DatabaseURL: "postgres://user:password@localhost/browsergrid?sslmode=disable",
		RedisAddr:   "localhost:6379",
		RedisDB:     0,
		Storage: storage.Config{
			Provider: "local",
			Settings: map[string]interface{}{
				"bucket_url": "file:///var/lib/browsergrid/storage",
			},
		},
	}

	if v := os.Getenv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}

	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.RedisPassword = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if db, err := strconv.Atoi(v); err == nil {
			cfg.RedisDB = db
		}
	}

	// Load storage configuration
	if v := os.Getenv("STORAGE_PROVIDER"); v != "" {
		cfg.Storage.Provider = v
	}

	if v := os.Getenv("STORAGE_BUCKET_URL"); v != "" {
		cfg.Storage.Settings["bucket_url"] = v
	}

	// Provider-specific settings
	switch cfg.Storage.Provider {
	case "s3":
		if v := os.Getenv("AWS_REGION"); v != "" {
			cfg.Storage.Settings["region"] = v
		}
	case "azure":
		if v := os.Getenv("AZURE_STORAGE_ACCOUNT"); v != "" {
			cfg.Storage.Settings["account_name"] = v
		}
	case "gcs":
		if v := os.Getenv("GCP_PROJECT_ID"); v != "" {
			cfg.Storage.Settings["project_id"] = v
		}
	}

	return cfg
}
