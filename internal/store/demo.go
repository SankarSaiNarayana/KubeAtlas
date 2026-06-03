package store

// import (
// 	"context"
// 	"time"

// 	"github.com/kube-dashboard/kube_dashboard/internal/models"
// )

// // SeedDemo populates sample graph, changes, and an incident for local development.
// func (s *Store) SeedDemo(ctx context.Context, clusterID string) (DashboardStats, error) {
// 	// Make demo seeding idempotent.
// 	_, _ = s.pool.Exec(ctx, `
// 		DELETE FROM incidents
// 		WHERE cluster_id = $1
// 		  AND alert_labels->>'demo_seed' = 'true'
// 	`, clusterID)
// 	_, _ = s.pool.Exec(ctx, `
// 		DELETE FROM change_events
// 		WHERE cluster_id = $1
// 		  AND payload->>'demo_seed' = 'true'
// 	`, clusterID)

// 	ing, _ := s.UpsertGraphNode(ctx, models.GraphNode{
// 		ClusterID: clusterID, APIVersion: "networking.k8s.io/v1", Kind: "Ingress",
// 		Namespace: "demo", Name: "payments-ingress", Labels: map[string]string{"app": "payments", "demo_seed": "true"}, Status: "active",
// 	})
// 	svc, _ := s.UpsertGraphNode(ctx, models.GraphNode{
// 		ClusterID: clusterID, APIVersion: "v1", Kind: "Service",
// 		Namespace: "demo", Name: "payments-api", Labels: map[string]string{"app": "payments", "demo_seed": "true"}, Status: "active",
// 	})
// 	dep, _ := s.UpsertGraphNode(ctx, models.GraphNode{
// 		ClusterID: clusterID, APIVersion: "apps/v1", Kind: "Deployment",
// 		Namespace: "demo", Name: "payments-api", Labels: map[string]string{"app": "payments", "demo_seed": "true"}, Status: "not_ready",
// 	})
// 	cm, _ := s.UpsertGraphNode(ctx, models.GraphNode{
// 		ClusterID: clusterID, APIVersion: "v1", Kind: "ConfigMap",
// 		Namespace: "demo", Name: "payments-config", Labels: map[string]string{"demo_seed": "true"}, Status: "referenced",
// 	})

// 	_ = s.UpsertGraphEdge(ctx, models.GraphEdge{ClusterID: clusterID, SourceID: ing.ID, TargetID: svc.ID, EdgeType: "exposes"})
// 	_ = s.UpsertGraphEdge(ctx, models.GraphEdge{ClusterID: clusterID, SourceID: svc.ID, TargetID: dep.ID, EdgeType: "selects"})
// 	_ = s.UpsertGraphEdge(ctx, models.GraphEdge{ClusterID: clusterID, SourceID: dep.ID, TargetID: cm.ID, EdgeType: "mounts"})

// 	now := time.Now().UTC()
// 	t1 := now.Add(-45 * time.Minute)
// 	t2 := now.Add(-12 * time.Minute)
// 	t3 := now.Add(-2 * time.Minute)

// 	_, _ = s.InsertChangeEvent(ctx, models.ChangeEventInput{
// 		ClusterID: clusterID, APIVersion: "apps/v1", Kind: "Deployment", Namespace: "demo", Name: "payments-api",
// 		Verb: "update", Actor: "deploy-bot@ci", Source: "gitops",
// 		DiffSummary: "image: payments-api:v1.2.0 → v1.3.0", OccurredAt: &t1,
// 		Payload: map[string]any{"demo_seed": true},
// 	})
// 	_, _ = s.InsertChangeEvent(ctx, models.ChangeEventInput{
// 		ClusterID: clusterID, APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Namespace: "demo", Name: "payments-ingress",
// 		Verb: "patch", Actor: "alice@platform.io", Source: "kubectl",
// 		DiffSummary: "tls secret reference updated", OccurredAt: &t2,
// 		Payload: map[string]any{"demo_seed": true},
// 	})
// 	_, _ = s.InsertChangeEvent(ctx, models.ChangeEventInput{
// 		ClusterID: clusterID, APIVersion: "v1", Kind: "ConfigMap", Namespace: "demo", Name: "payments-config",
// 		Verb: "update", Actor: "alice@platform.io", Source: "gitops",
// 		DiffSummary: "replica count env PAYMENTS_REPLICAS: 3 → 5", OccurredAt: &t3,
// 		Payload: map[string]any{"demo_seed": true},
// 	})

// 	_, _ = s.CreateIncident(ctx, models.Incident{
// 		ClusterID: clusterID, Title: "Pod payments-api is crash looping", Status: "open",
// 		ResourceKind: "Pod", ResourceNS: "demo", ResourceName: "payments-api-7d4f8b",
// 		AlertLabels: map[string]any{"alertname": "KubePodCrashLooping", "severity": "critical", "demo_seed": true},
// 		StartedAt:   now.Add(-10 * time.Minute),
// 	})

// 	return s.DashboardStats(ctx, clusterID)
// }
