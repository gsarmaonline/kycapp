package config

import (
	"testing"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is empty")
	}
}

func TestLoadDefaultsHTTPAddr(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/kyc?sslmode=disable")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("API_TOKENS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.CheckRateLimitPerMin != 120 {
		t.Fatalf("CheckRateLimitPerMin = %d", cfg.CheckRateLimitPerMin)
	}
	if cfg.AuthRateLimitPerMin != 20 {
		t.Fatalf("AuthRateLimitPerMin = %d", cfg.AuthRateLimitPerMin)
	}
	if cfg.UploadDir != "data/uploads" {
		t.Fatalf("UploadDir = %q", cfg.UploadDir)
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("PublicBaseURL = %q", cfg.PublicBaseURL)
	}
}

func TestLoadCustomHTTPAddr(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/kyc?sslmode=disable")
	t.Setenv("HTTP_ADDR", ":9090")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
}

func TestLoadAPITokens(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/kyc?sslmode=disable")
	t.Setenv("API_TOKENS", " a, b , ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.APITokens) != 2 || cfg.APITokens[0] != "a" || cfg.APITokens[1] != "b" {
		t.Fatalf("APITokens = %#v", cfg.APITokens)
	}
}
