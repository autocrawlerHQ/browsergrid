package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          int
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	Storage StorageConfig
}

// StorageConfig holds configuration for artifact storage backends.
type StorageConfig struct {
	Backend   string
	LocalPath string
	S3Bucket  string
	S3Region  string
	S3Prefix  string
}

func Load() Config {
	cfg := Config{
		Port:        8765,
		DatabaseURL: "postgres://user:password@localhost/browsergrid?sslmode=disable",
		RedisAddr:   "localhost:6379",
		RedisDB:     0,
		Storage: StorageConfig{
			Backend:   "local",
			LocalPath: "./data",
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

	if v := os.Getenv("STORAGE_BACKEND"); v != "" {
		cfg.Storage.Backend = v
	}
	if v := os.Getenv("STORAGE_PATH"); v != "" {
		cfg.Storage.LocalPath = v
	}
	if v := os.Getenv("STORAGE_S3_BUCKET"); v != "" {
		cfg.Storage.S3Bucket = v
	}
	if v := os.Getenv("STORAGE_S3_REGION"); v != "" {
		cfg.Storage.S3Region = v
	}
	if v := os.Getenv("STORAGE_S3_PREFIX"); v != "" {
		cfg.Storage.S3Prefix = v
	}

	return cfg
}
