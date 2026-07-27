package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gsarmaonline/kyc/internal/config"
	"github.com/gsarmaonline/kyc/internal/jobs"
	"github.com/gsarmaonline/kyc/internal/mailer"
	"github.com/gsarmaonline/kyc/internal/observability"
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
	mail, err := mailer.NewFromConfig(mailer.Config{
		Provider: cfg.EmailProvider,
		APIKey:   cfg.ResendAPIKey,
		From:     cfg.EmailFrom,
	})
	if err != nil {
		log.Fatalf("mailer: %v", err)
	}
	svc.SetMailer(mail)

	riverClient, err := jobs.NewWorkerClient(db.Pool(), jobs.WorkerHooks{
		Process:      svc.ProcessAutomationEvent,
		Resume:       svc.ResumeAutomation,
		ScheduleTick: svc.ProcessScheduleTick,
	})
	if err != nil {
		log.Fatalf("river: %v", err)
	}
	svc.SetEnqueuer(riverClient)

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := riverClient.Start(runCtx); err != nil {
		log.Fatalf("river start: %v", err)
	}
	log.Printf("automation worker running (river, email=%s, schedules=UTC, obs=%v)", mail.Name(), cfg.ObservabilityDatabaseURL != "")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := riverClient.Stop(shutdownCtx); err != nil {
		log.Printf("river stop: %v", err)
	}
}
