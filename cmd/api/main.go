package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gsarmaonline/kyc/internal/config"
	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	svc := service.New(db)
	corsOrigin := cfg.CORSOrigin
	if corsOrigin == "" {
		corsOrigin = os.Getenv("CORS_ORIGIN")
	}
	if corsOrigin == "" {
		corsOrigin = "http://localhost:8080"
	}

	srv := httpserver.New(db, httpserver.Options{
		Service:              svc,
		CORSOrigin:           corsOrigin,
		APITokens:            cfg.APITokens,
		CheckRateLimitPerMin: cfg.CheckRateLimitPerMin,
	})
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if len(cfg.APITokens) > 0 {
			log.Printf("listening on %s (API auth enabled)", cfg.HTTPAddr)
		} else {
			log.Printf("listening on %s (API auth disabled)", cfg.HTTPAddr)
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
