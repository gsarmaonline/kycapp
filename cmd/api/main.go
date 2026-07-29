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
	"github.com/gsarmaonline/kyc/internal/mailer"
	"github.com/gsarmaonline/kyc/internal/observability"
	"github.com/gsarmaonline/kyc/internal/payments"
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

	obs, err := observability.NewFromURL(ctx, cfg.ObservabilityDatabaseURL)
	if err != nil {
		log.Fatalf("observability: %v", err)
	}
	defer obs.Close()

	svc := service.New(db)
	svc.SetObservability(obs)
	svc.ConfigureAssets(cfg.UploadDir, cfg.PublicBaseURL)
	pay, err := payments.NewFromConfig(payments.Config{
		Provider:      cfg.PaymentsProvider,
		StripeSecret:  cfg.StripeSecretKey,
		WebhookSecret: cfg.StripeWebhookSecret,
	})
	if err != nil {
		log.Fatalf("payments: %v", err)
	}
	svc.SetPayments(pay, cfg.StripeSuccessURL, cfg.StripeCancelURL, cfg.AppOrigin)
	mail, err := mailer.NewFromConfig(mailer.Config{
		Provider: cfg.EmailProvider,
		APIKey:   cfg.ResendAPIKey,
		From:     cfg.EmailFrom,
	})
	if err != nil {
		log.Fatalf("mailer: %v", err)
	}
	svc.SetMailer(mail)
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

	var obsPinger httpserver.DBPinger
	if cfg.ObservabilityDatabaseURL != "" {
		obsPinger = obs
	}
	srv := httpserver.New(db, httpserver.Options{
		Service:              svc,
		Observability:        obsPinger,
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
			"listening on %s (google_oauth=%v dev_login=%v service_tokens=%d payments=%s email=%s obs=%v)",
			cfg.HTTPAddr, cfg.GoogleConfigured(), cfg.AuthDevLogin, len(cfg.APITokens), pay.Name(), mail.Name(),
			cfg.ObservabilityDatabaseURL != "",
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
