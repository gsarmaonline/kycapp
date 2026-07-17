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
	PlatformAdminEmails  []string
	CheckRateLimitPerMin int
	AuthRateLimitPerMin  int

	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
	AppOrigin          string
	OAuthStateSecret   string
	AuthDevLogin       bool

	UploadDir     string
	PublicBaseURL string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:             envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		CORSOrigin:           os.Getenv("CORS_ORIGIN"),
		APITokens:            splitCSV(os.Getenv("API_TOKENS")),
		PlatformAdminEmails:  splitCSV(os.Getenv("PLATFORM_ADMIN_EMAILS")),
		CheckRateLimitPerMin: envInt("CHECK_RATE_LIMIT_PER_MIN", 120),
		AuthRateLimitPerMin:  envInt("AUTH_RATE_LIMIT_PER_MIN", 20),
		GoogleClientID:       os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		OAuthRedirectURL:     envOr("OAUTH_REDIRECT_URL", "http://localhost:8080/v1/auth/google/callback"),
		AppOrigin:            envOr("APP_ORIGIN", "http://localhost:8080"),
		OAuthStateSecret:     envOr("OAUTH_STATE_SECRET", os.Getenv("API_TOKENS")),
		AuthDevLogin:         envBool("AUTH_DEV_LOGIN", false),
		UploadDir:            envOr("UPLOAD_DIR", "data/uploads"),
		PublicBaseURL:        envOr("PUBLIC_BASE_URL", envOr("APP_ORIGIN", "http://localhost:8080")),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.OAuthStateSecret == "" {
		cfg.OAuthStateSecret = "dev-insecure-oauth-state"
	}
	return cfg, nil
}

func (c Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
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

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
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
