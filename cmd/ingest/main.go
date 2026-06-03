package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/kube-dashboard/kube_dashboard/internal/config"
	"github.com/kube-dashboard/kube_dashboard/internal/ingest"
	"github.com/kube-dashboard/kube_dashboard/internal/store"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	if err := ingest.RunIngest(ctx, cfg, st); err != nil && err != context.Canceled {
		log.Fatalf("ingest stopped: %v", err)
	}
}
