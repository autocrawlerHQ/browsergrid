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

	// Profiles storage backend configuration
	ProfileStorage  string // local | blobfs | s3 | azure
	ProfilesPath    string // local base path for profiles
	ProfileBlobDir  string // blobfs: where ZIPs are stored
	ProfileCacheDir string // blobfs: where ZIPs are extracted for mounting
}

func Load() Config {
	cfg := Config{
		Port:            8765,
		DatabaseURL:     "postgres://user:password@localhost/browsergrid?sslmode=disable",
		RedisAddr:       "localhost:6379",
		RedisDB:         0,
		ProfileStorage:  getenv("BROWSERGRID_PROFILE_STORAGE", "local"),
		ProfilesPath:    getenv("BROWSERGRID_PROFILES_PATH", "/var/lib/browsergrid/profiles"),
		ProfileBlobDir:  getenv("BROWSERGRID_PROFILE_BLOB_PATH", "/var/lib/browsergrid/profile-blobs"),
		ProfileCacheDir: getenv("BROWSERGRID_PROFILE_CACHE_PATH", "/var/lib/browsergrid/profile-cache"),
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

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
