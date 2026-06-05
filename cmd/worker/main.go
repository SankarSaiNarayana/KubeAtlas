package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/kube-dashboard/kube_dashboard/internal/ai"
	"github.com/kube-dashboard/kube_dashboard/internal/config"
	"github.com/kube-dashboard/kube_dashboard/internal/k8sclient"
	"github.com/kube-dashboard/kube_dashboard/internal/pipeline"
	"github.com/kube-dashboard/kube_dashboard/internal/realtime"
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

	for _, path := range []string{
		"migrations/001_init.sql",
		"migrations/002_add_graph_node_id.sql",
		"migrations/003_kubeatlas.sql",
	} {
		if err := st.Migrate(ctx, path); err != nil {
			log.Fatalf("migrate %s: %v", path, err)
		}
	}

	client, _, err := k8sclient.New()
	if err != nil {
		log.Fatalf("kubernetes: %v", err)
	}

	aiSvc := ai.NewService(cfg.AIServiceURL, st, cfg.AIServiceTimeout)
	inv := aiSvc
	rem := aiSvc

	if aiSvc.Enabled() {
		if err := aiSvc.Ping(ctx); err != nil {
			log.Printf("ai service unreachable at %s; incident investigation will record AI service error details instead of using fallback rules: %v", cfg.AIServiceURL, err)
		} else {
			log.Printf("ai service connected: %s", cfg.AIServiceURL)
		}
	} else {
		log.Printf("AI_SERVICE_URL unset; incident investigation will record AI service unavailable errors instead of using fallback rules")
	}

	hub := realtime.NewHub()
	orch := pipeline.New(cfg.ClusterID, st, client, hub, inv, rem)
	log.Printf("kubeatlas worker started (cluster_id=%s)", cfg.ClusterID)
	if err := orch.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("worker: %v", err)
	}
}
