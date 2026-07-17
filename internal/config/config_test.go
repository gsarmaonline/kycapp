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
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://localhost/kyc?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
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
