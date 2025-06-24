package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	Port        int
	Host        string
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost/browsergrid?sslmode=disable"),
		Port:        getEnvInt("PORT", 8080),
		Host:        getEnv("HOST", "0.0.0.0"),
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
