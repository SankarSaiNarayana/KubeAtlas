package store

import (
	"context"
	"time"
)

type DashboardStats struct {
	GraphNodes    int `json:"graph_nodes"`
	GraphEdges    int `json:"graph_edges"`
	Changes24h    int `json:"changes_24h"`
	OpenIncidents int `json:"open_incidents"`
}

func (s *Store) DashboardStats(ctx context.Context, clusterID string) (DashboardStats, error) {
	var stats DashboardStats
	since := time.Now().UTC().Add(-24 * time.Hour)

	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*)::int FROM graph_nodes WHERE cluster_id = $1 AND (labels->>'demo_seed' IS NULL OR labels->>'demo_seed' <> 'true')),
			(SELECT COUNT(*)::int FROM graph_edges e
			 WHERE cluster_id = $1
			   AND source_id IN (SELECT id FROM graph_nodes WHERE cluster_id = $1 AND (labels->>'demo_seed' IS NULL OR labels->>'demo_seed' <> 'true'))
			   AND target_id IN (SELECT id FROM graph_nodes WHERE cluster_id = $1 AND (labels->>'demo_seed' IS NULL OR labels->>'demo_seed' <> 'true'))),
			(SELECT COUNT(*)::int FROM change_events WHERE cluster_id = $1 AND occurred_at >= $2 AND (payload->>'demo_seed' IS NULL OR payload->>'demo_seed' <> 'true')),
			(SELECT COUNT(*)::int FROM incidents WHERE cluster_id = $1 AND status = 'open' AND (alert_labels->>'demo_seed' IS NULL OR alert_labels->>'demo_seed' <> 'true'))
	`, clusterID, since).Scan(&stats.GraphNodes, &stats.GraphEdges, &stats.Changes24h, &stats.OpenIncidents)
	return stats, err
}
