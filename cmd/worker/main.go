package main

import (
	"log"
	"os"

	"github.com/gsarmaonline/kyc/internal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	addr := envOr("TEMPORAL_ADDRESS", "localhost:7233")
	namespace := envOr("TEMPORAL_NAMESPACE", "default")
	taskQueue := envOr("TEMPORAL_TASK_QUEUE", workflows.TaskQueue)

	c, err := client.Dial(client.Options{
		HostPort:  addr,
		Namespace: namespace,
	})
	if err != nil {
		log.Fatalf("temporal dial %s: %v", addr, err)
	}
	defer c.Close()

	w := worker.New(c, taskQueue, worker.Options{})
	workflows.Register(w)

	log.Printf("temporal worker listening addr=%s namespace=%s queue=%s", addr, namespace, taskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("worker: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
