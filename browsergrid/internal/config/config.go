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
}

func Load() Config {
	cfg := Config{
		Port:        8765,
		DatabaseURL: "postgres://user:password@localhost/browsergrid?sslmode=disable",
		RedisAddr:   "localhost:6379",
		RedisDB:     0,
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

	return cfg
}
