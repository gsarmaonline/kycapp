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
	"github.com/gsarmaonline/kyc/internal/jobs"
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
	svc.ConfigureAssets(cfg.UploadDir, cfg.PublicBaseURL)
	if riverClient, err := jobs.NewInsertClient(db.Pool()); err != nil {
		log.Fatalf("river: %v", err)
	} else {
		svc.SetEnqueuer(riverClient)
	}
	corsOrigin := cfg.CORSOrigin
	if corsOrigin == "" {
		corsOrigin = os.Getenv("CORS_ORIGIN")
	}
	if corsOrigin == "" {
		corsOrigin = cfg.AppOrigin
	}

	srv := httpserver.New(db, httpserver.Options{
		Service:              svc,
		CORSOrigin:           corsOrigin,
		APITokens:            cfg.APITokens,
		PlatformAdminEmails:  cfg.PlatformAdminEmails,
		CheckRateLimitPerMin: cfg.CheckRateLimitPerMin,
		AuthRateLimitPerMin:  cfg.AuthRateLimitPerMin,
		GoogleClientID:       cfg.GoogleClientID,
		GoogleClientSecret:   cfg.GoogleClientSecret,
		OAuthRedirectURL:     cfg.OAuthRedirectURL,
		OAuthStateSecret:     cfg.OAuthStateSecret,
		AppOrigin:            cfg.AppOrigin,
		AuthDevLogin:         cfg.AuthDevLogin,
	})
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf(
			"listening on %s (google_oauth=%v dev_login=%v service_tokens=%d)",
			cfg.HTTPAddr, cfg.GoogleConfigured(), cfg.AuthDevLogin, len(cfg.APITokens),
		)
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
