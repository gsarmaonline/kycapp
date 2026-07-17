package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration loaded from the environment.
type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	CORSOrigin           string
	APITokens            []string
	CheckRateLimitPerMin int
}

// Load reads configuration from environment variables.
//
//	HTTP_ADDR                  listen address (default :8080)
//	DATABASE_URL               Postgres connection string (required)
//	CORS_ORIGIN                CORS allow origin (optional)
//	API_TOKENS                 comma-separated Bearer tokens; empty disables auth
//	CHECK_RATE_LIMIT_PER_MIN   max check requests per actor per minute (default 120; 0 disables)
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:             envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		CORSOrigin:           os.Getenv("CORS_ORIGIN"),
		APITokens:            splitCSV(os.Getenv("API_TOKENS")),
		CheckRateLimitPerMin: envInt("CHECK_RATE_LIMIT_PER_MIN", 120),
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

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
