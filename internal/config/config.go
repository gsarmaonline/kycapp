package config

import (
	"fmt"
	"os"
)

// Config holds runtime configuration loaded from the environment.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

// Load reads configuration from environment variables.
//
//	HTTP_ADDR     listen address (default :8080)
//	DATABASE_URL  Postgres connection string (required)
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    envOr("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
