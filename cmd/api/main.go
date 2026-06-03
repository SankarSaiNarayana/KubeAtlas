package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kube-dashboard/kube_dashboard/internal/api/handlers"
	"github.com/kube-dashboard/kube_dashboard/internal/api/middleware"
	"github.com/kube-dashboard/kube_dashboard/internal/config"
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

	migrationPath := os.Getenv("MIGRATION_PATH")
	if migrationPath != "" {
		if err := st.Migrate(ctx, migrationPath); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	} else {
		migrations := []string{
			"migrations/001_init.sql",
			"migrations/002_add_graph_node_id.sql",
		}
		for _, migration := range migrations {
			if err := st.Migrate(ctx, migration); err != nil {
				log.Fatalf("migrate %s: %v", migration, err)
			}
		}
	}

	mux := http.NewServeMux()
	h := handlers.New(st, cfg)
	h.Register(mux)

	server := &http.Server{Addr: cfg.APIAddr, Handler: middleware.CORS(middleware.Auth(mux, cfg))}
	go func() {
		log.Printf("api listening on %s (cluster_id=%s)", cfg.APIAddr, cfg.ClusterID)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	_ = server.Shutdown(context.Background())
}
