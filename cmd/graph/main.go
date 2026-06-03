package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kube-dashboard/kube_dashboard/internal/config"
	"github.com/kube-dashboard/kube_dashboard/internal/graph"
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

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}

	builder, err := graph.NewBuilder(cfg.ClusterID, kubeconfig, st)
	if err != nil {
		log.Fatalf("graph builder: %v", err)
	}

	if err := builder.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("graph builder stopped: %v", err)
	}
}
