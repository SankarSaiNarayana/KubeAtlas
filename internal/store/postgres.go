package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kube-dashboard/kube_dashboard/internal/models"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Migrate(ctx context.Context, path string) error {
	sql, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	_, err = s.pool.Exec(ctx, string(sql))
	return err
}

func (s *Store) UpsertGraphNode(ctx context.Context, node models.GraphNode) (models.GraphNode, error) {
	labels, _ := json.Marshal(node.Labels)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO graph_nodes (cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (cluster_id, api_version, kind, namespace, name)
		DO UPDATE SET resource_uid = EXCLUDED.resource_uid, labels = EXCLUDED.labels, status = EXCLUDED.status, updated_at = NOW()
		RETURNING id, cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at
	`, node.ClusterID, nullStr(node.ResourceUID), node.APIVersion, node.Kind, node.Namespace, node.Name, labels, node.Status)

	var out models.GraphNode
	var labelsRaw []byte
	var uid *string
	if err := row.Scan(&out.ID, &out.ClusterID, &uid, &out.APIVersion, &out.Kind, &out.Namespace, &out.Name, &labelsRaw, &out.Status, &out.UpdatedAt); err != nil {
		return out, err
	}
	if uid != nil {
		out.ResourceUID = *uid
	}
	_ = json.Unmarshal(labelsRaw, &out.Labels)
	if out.Labels == nil {
		out.Labels = map[string]string{}
	}
	return out, nil
}

func (s *Store) UpsertGraphEdge(ctx context.Context, edge models.GraphEdge) error {
	meta, _ := json.Marshal(edge.Metadata)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO graph_edges (cluster_id, source_id, target_id, edge_type, metadata)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cluster_id, source_id, target_id, edge_type) DO NOTHING
	`, edge.ClusterID, edge.SourceID, edge.TargetID, edge.EdgeType, meta)
	return err
}

func (s *Store) GetGraph(ctx context.Context, clusterID, namespace string, includeDemo bool) (models.GraphResponse, error) {
	var resp models.GraphResponse

	var nodeQuery string
	args := []any{clusterID}
	if namespace != "" {
		nodeQuery = `
			SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at
			FROM graph_nodes WHERE cluster_id = $1 AND namespace = $2`
		args = append(args, namespace)
	} else {
		nodeQuery = `
			SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at
			FROM graph_nodes WHERE cluster_id = $1`
	}
	if !includeDemo {
		nodeQuery += `
			AND (labels->>'demo_seed' IS NULL OR labels->>'demo_seed' <> 'true')`
	}
	nodeQuery += `
			ORDER BY kind, namespace, name
		`

	rows, err := s.pool.Query(ctx, nodeQuery, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	nodeIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var n models.GraphNode
		var labelsRaw []byte
		var uid *string
		if err := rows.Scan(&n.ID, &n.ClusterID, &uid, &n.APIVersion, &n.Kind, &n.Namespace, &n.Name, &labelsRaw, &n.Status, &n.UpdatedAt); err != nil {
			return resp, err
		}
		if uid != nil {
			n.ResourceUID = *uid
		}
		_ = json.Unmarshal(labelsRaw, &n.Labels)
		if n.Labels == nil {
			n.Labels = map[string]string{}
		}
		resp.Nodes = append(resp.Nodes, n)
		nodeIDs = append(nodeIDs, n.ID)
	}
	if len(nodeIDs) == 0 {
		return resp, rows.Err()
	}

	edgeRows, err := s.pool.Query(ctx, `
		SELECT id, cluster_id, source_id, target_id, edge_type, metadata
		FROM graph_edges WHERE cluster_id = $1 AND source_id = ANY($2) AND target_id = ANY($2)
	`, clusterID, nodeIDs)
	if err != nil {
		return resp, err
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var e models.GraphEdge
		var metaRaw []byte
		if err := edgeRows.Scan(&e.ID, &e.ClusterID, &e.SourceID, &e.TargetID, &e.EdgeType, &metaRaw); err != nil {
			return resp, err
		}
		_ = json.Unmarshal(metaRaw, &e.Metadata)
		resp.Edges = append(resp.Edges, e)
	}
	return resp, rows.Err()
}

func (s *Store) InsertChangeEvent(ctx context.Context, in models.ChangeEventInput) (models.ChangeEvent, error) {
	payload, _ := json.Marshal(in.Payload)
	if in.Payload == nil {
		payload = []byte("{}")
	}
	occurred := time.Now().UTC()
	if in.OccurredAt != nil {
		occurred = in.OccurredAt.UTC()
	}
	if in.ClusterID == "" {
		in.ClusterID = "local"
	}

	// Basic de-duplication: noisy controllers/webhooks can emit identical events repeatedly.
	// If the same event appears again within a short window, ignore it.
	{
		var exists bool
		err := s.pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM change_events
				WHERE cluster_id = $1
				  AND kind = $2 AND namespace = $3 AND name = $4
				  AND verb = $5 AND actor = $6 AND source = $7 AND diff_summary = $8
				  AND occurred_at >= $9
			)
		`, in.ClusterID, in.Kind, in.Namespace, in.Name, in.Verb, in.Actor, in.Source, in.DiffSummary, occurred.Add(-10*time.Second)).Scan(&exists)
		if err == nil && exists {
			return models.ChangeEvent{
				ID:        uuid.Nil,
				ClusterID: in.ClusterID,
				ResourceRef: models.ResourceRef{
					ClusterID:  in.ClusterID,
					APIVersion: in.APIVersion,
					Kind:       in.Kind,
					Namespace:  in.Namespace,
					Name:       in.Name,
					UID:        in.ResourceUID,
				},
				Verb:        in.Verb,
				Actor:       in.Actor,
				Source:      in.Source,
				DiffSummary: in.DiffSummary,
				Payload:     in.Payload,
				OccurredAt:  occurred,
			}, nil
		}
	}

	graphNodeID, err := s.resolveGraphNodeID(ctx, in.ClusterID, in.APIVersion, in.Kind, in.Namespace, in.Name)
	if err != nil {
		return models.ChangeEvent{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO change_events (cluster_id, resource_uid, api_version, kind, namespace, name, graph_node_id, verb, actor, source, diff_summary, payload, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, cluster_id, resource_uid, api_version, kind, namespace, name, verb, actor, source, diff_summary, payload, occurred_at
	`, in.ClusterID, nullStr(in.ResourceUID), in.APIVersion, in.Kind, in.Namespace, in.Name, graphNodeID, in.Verb, in.Actor, in.Source, in.DiffSummary, payload, occurred)

	var out models.ChangeEvent
	var uid *string
	var payloadRaw []byte
	if err := row.Scan(
		&out.ID, &out.ClusterID, &uid, &out.ResourceRef.APIVersion, &out.ResourceRef.Kind, &out.ResourceRef.Namespace, &out.ResourceRef.Name,
		&out.Verb, &out.Actor, &out.Source, &out.DiffSummary, &payloadRaw, &out.OccurredAt,
	); err != nil {
		return out, err
	}
	out.ResourceRef.ClusterID = in.ClusterID
	if uid != nil {
		out.ResourceRef.UID = *uid
	}
	_ = json.Unmarshal(payloadRaw, &out.Payload)
	return out, nil
}

func (s *Store) ListChanges(ctx context.Context, clusterID, namespace, kind, name, actor, source, verb string, since time.Time, limit int, includeDemo bool) ([]models.ChangeEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, verb, actor, source, diff_summary, payload, occurred_at
		FROM change_events
		WHERE cluster_id = $1 AND occurred_at >= $2
	`
	args := []any{clusterID, since}
	n := 3
	if namespace != "" {
		query += fmt.Sprintf(" AND namespace = $%d", n)
		args = append(args, namespace)
		n++
	}
	if kind != "" {
		query += fmt.Sprintf(" AND kind = $%d", n)
		args = append(args, kind)
		n++
	}
	if name != "" {
		query += fmt.Sprintf(" AND name = $%d", n)
		args = append(args, name)
		n++
	}
	if actor != "" {
		query += fmt.Sprintf(" AND actor = $%d", n)
		args = append(args, actor)
		n++
	}
	if source != "" {
		query += fmt.Sprintf(" AND source = $%d", n)
		args = append(args, source)
		n++
	}
	if verb != "" {
		query += fmt.Sprintf(" AND verb = $%d", n)
		args = append(args, verb)
		n++
	}
	if !includeDemo {
		query += fmt.Sprintf(" AND (payload->>'demo_seed' IS NULL OR payload->>'demo_seed' <> 'true')")
	}
	query += fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT $%d", n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.ChangeEvent
	for rows.Next() {
		var e models.ChangeEvent
		var uid *string
		var payloadRaw []byte
		if err := rows.Scan(
			&e.ID, &e.ClusterID, &uid, &e.ResourceRef.APIVersion, &e.ResourceRef.Kind, &e.ResourceRef.Namespace, &e.ResourceRef.Name,
			&e.Verb, &e.Actor, &e.Source, &e.DiffSummary, &payloadRaw, &e.OccurredAt,
		); err != nil {
			return nil, err
		}
		e.ResourceRef.ClusterID = clusterID
		if uid != nil {
			e.ResourceRef.UID = *uid
		}
		_ = json.Unmarshal(payloadRaw, &e.Payload)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) resolveGraphNodeID(ctx context.Context, clusterID, apiVersion, kind, namespace, name string) (*uuid.UUID, error) {
	var nodeID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM graph_nodes
		WHERE cluster_id = $1 AND api_version = $2 AND kind = $3 AND namespace = $4 AND name = $5
		LIMIT 1
	`, clusterID, apiVersion, kind, namespace, name).Scan(&nodeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &nodeID, nil
}

func (s *Store) GetGraphNode(ctx context.Context, clusterID string, id uuid.UUID) (models.GraphNode, error) {
	var node models.GraphNode
	var labelsRaw []byte
	var uid *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at
		FROM graph_nodes WHERE cluster_id = $1 AND id = $2
	`, clusterID, id).Scan(&node.ID, &node.ClusterID, &uid, &node.APIVersion, &node.Kind, &node.Namespace, &node.Name, &labelsRaw, &node.Status, &node.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.GraphNode{}, nil
		}
		return models.GraphNode{}, err
	}
	if uid != nil {
		node.ResourceUID = *uid
	}
	_ = json.Unmarshal(labelsRaw, &node.Labels)
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	return node, nil
}

func (s *Store) GetResourceNeighbors(ctx context.Context, clusterID string, nodeID uuid.UUID, depth int, direction string) (models.GraphResponse, error) {
	var resp models.GraphResponse
	if depth <= 0 {
		depth = 1
	}
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		direction = "both"
	}
	if direction != "incoming" && direction != "outgoing" && direction != "both" {
		direction = "both"
	}

	query := ``
	var args []any
	if depth == 1 {
		if direction == "outgoing" {
			query = `
			SELECT id, cluster_id, source_id, target_id, edge_type, metadata
			FROM graph_edges
			WHERE cluster_id = $1 AND source_id = $2
			`
			args = []any{clusterID, nodeID}
		} else if direction == "incoming" {
			query = `
			SELECT id, cluster_id, source_id, target_id, edge_type, metadata
			FROM graph_edges
			WHERE cluster_id = $1 AND target_id = $2
			`
			args = []any{clusterID, nodeID}
		} else {
			query = `
			SELECT id, cluster_id, source_id, target_id, edge_type, metadata
			FROM graph_edges
			WHERE cluster_id = $1 AND (source_id = $2 OR target_id = $2)
			`
			args = []any{clusterID, nodeID}
		}
		rows, err := s.pool.Query(ctx, query, args...)
		if err != nil {
			return resp, err
		}
		defer rows.Close()
		nodeIDs := []uuid.UUID{nodeID}
		for rows.Next() {
			var e models.GraphEdge
			var metaRaw []byte
			if err := rows.Scan(&e.ID, &e.ClusterID, &e.SourceID, &e.TargetID, &e.EdgeType, &metaRaw); err != nil {
				return resp, err
			}
			_ = json.Unmarshal(metaRaw, &e.Metadata)
			resp.Edges = append(resp.Edges, e)
			nodeIDs = append(nodeIDs, e.SourceID, e.TargetID)
		}
		if len(nodeIDs) == 0 {
			return resp, nil
		}
		nodeRows, err := s.pool.Query(ctx, `
			SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at
			FROM graph_nodes
			WHERE cluster_id = $1 AND id = ANY($2)
		`, clusterID, nodeIDs)
		if err != nil {
			return resp, err
		}
		defer nodeRows.Close()
		for nodeRows.Next() {
			var n models.GraphNode
			var labelsRaw []byte
			var uid *string
			if err := nodeRows.Scan(&n.ID, &n.ClusterID, &uid, &n.APIVersion, &n.Kind, &n.Namespace, &n.Name, &labelsRaw, &n.Status, &n.UpdatedAt); err != nil {
				return resp, err
			}
			if uid != nil {
				n.ResourceUID = *uid
			}
			_ = json.Unmarshal(labelsRaw, &n.Labels)
			if n.Labels == nil {
				n.Labels = map[string]string{}
			}
			resp.Nodes = append(resp.Nodes, n)
		}
		return resp, rows.Err()
	}

	query = `
		WITH RECURSIVE search AS (
			SELECT source_id, target_id, 1 AS depth
			FROM graph_edges
			WHERE cluster_id = $1 AND (source_id = $2 OR target_id = $2)
			UNION ALL
			SELECT e.source_id, e.target_id, search.depth + 1
			FROM graph_edges e
			JOIN search ON (
				$3 = 'both' AND (e.source_id = search.target_id OR e.target_id = search.source_id)
				OR $3 = 'outgoing' AND e.source_id = search.target_id
				OR $3 = 'incoming' AND e.target_id = search.source_id
			)
			WHERE search.depth < $4
		)
		SELECT DISTINCT id, cluster_id, source_id, target_id, edge_type, metadata
		FROM graph_edges
		WHERE cluster_id = $1
		  AND (source_id, target_id) IN (SELECT source_id, target_id FROM search)
	`
	args = []any{clusterID, nodeID, direction, depth}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()
	neighborIDs := []uuid.UUID{nodeID}
	for rows.Next() {
		var e models.GraphEdge
		var metaRaw []byte
		if err := rows.Scan(&e.ID, &e.ClusterID, &e.SourceID, &e.TargetID, &e.EdgeType, &metaRaw); err != nil {
			return resp, err
		}
		_ = json.Unmarshal(metaRaw, &e.Metadata)
		resp.Edges = append(resp.Edges, e)
		neighborIDs = append(neighborIDs, e.SourceID, e.TargetID)
	}
	if len(neighborIDs) == 0 {
		return resp, nil
	}

	nodeRows, err := s.pool.Query(ctx, `
		SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, labels, status, updated_at
		FROM graph_nodes
		WHERE cluster_id = $1 AND id = ANY($2)
	`, clusterID, neighborIDs)
	if err != nil {
		return resp, err
	}
	defer nodeRows.Close()
	for nodeRows.Next() {
		var n models.GraphNode
		var labelsRaw []byte
		var uid *string
		if err := nodeRows.Scan(&n.ID, &n.ClusterID, &uid, &n.APIVersion, &n.Kind, &n.Namespace, &n.Name, &labelsRaw, &n.Status, &n.UpdatedAt); err != nil {
			return resp, err
		}
		if uid != nil {
			n.ResourceUID = *uid
		}
		_ = json.Unmarshal(labelsRaw, &n.Labels)
		if n.Labels == nil {
			n.Labels = map[string]string{}
		}
		resp.Nodes = append(resp.Nodes, n)
	}
	return resp, rows.Err()
}

func (s *Store) ListResourceChanges(ctx context.Context, clusterID string, nodeID uuid.UUID, since time.Time, limit int) ([]models.ChangeEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	var node models.GraphNode
	if err := s.pool.QueryRow(ctx, `
		SELECT kind, namespace, name, api_version
		FROM graph_nodes
		WHERE cluster_id = $1 AND id = $2
	`, clusterID, nodeID).Scan(&node.Kind, &node.Namespace, &node.Name, &node.APIVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []models.ChangeEvent{}, nil
		}
		return nil, err
	}

	query := `
		SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name, verb, actor, source, diff_summary, payload, occurred_at
		FROM change_events
		WHERE cluster_id = $1
		  AND occurred_at >= $2
		  AND (graph_node_id = $3 OR (graph_node_id IS NULL AND api_version = $4 AND kind = $5 AND namespace = $6 AND name = $7))
		ORDER BY occurred_at DESC
		LIMIT $8
	`
	rows, err := s.pool.Query(ctx, query, clusterID, since, nodeID, node.APIVersion, node.Kind, node.Namespace, node.Name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.ChangeEvent
	for rows.Next() {
		var e models.ChangeEvent
		var uid *string
		var payloadRaw []byte
		if err := rows.Scan(
			&e.ID, &e.ClusterID, &uid, &e.ResourceRef.APIVersion, &e.ResourceRef.Kind, &e.ResourceRef.Namespace, &e.ResourceRef.Name,
			&e.Verb, &e.Actor, &e.Source, &e.DiffSummary, &payloadRaw, &e.OccurredAt,
		); err != nil {
			return nil, err
		}
		e.ResourceRef.ClusterID = clusterID
		if uid != nil {
			e.ResourceRef.UID = *uid
		}
		_ = json.Unmarshal(payloadRaw, &e.Payload)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) CreateIncident(ctx context.Context, inc models.Incident) (models.Incident, error) {
	labels, _ := json.Marshal(inc.AlertLabels)
	if inc.AlertLabels == nil {
		labels = []byte("{}")
	}

	// De-dup: if a matching open incident already exists, reuse it.
	{
		var existing models.Incident
		var kind, ns, name *string
		var labelsRaw []byte
		err := s.pool.QueryRow(ctx, `
			SELECT id, cluster_id, title, status, resource_kind, resource_ns, resource_name, alert_labels, started_at
			FROM incidents
			WHERE cluster_id = $1 AND status = 'open'
			  AND title = $2
			  AND COALESCE(resource_kind,'') = COALESCE($3,'')
			  AND COALESCE(resource_ns,'') = COALESCE($4,'')
			  AND COALESCE(resource_name,'') = COALESCE($5,'')
			  AND started_at >= $6
			ORDER BY started_at DESC
			LIMIT 1
		`, inc.ClusterID, inc.Title, nullStr(inc.ResourceKind), nullStr(inc.ResourceNS), nullStr(inc.ResourceName), time.Now().UTC().Add(-10*time.Minute)).
			Scan(&existing.ID, &existing.ClusterID, &existing.Title, &existing.Status, &kind, &ns, &name, &labelsRaw, &existing.StartedAt)
		if err == nil {
			if kind != nil {
				existing.ResourceKind = *kind
			}
			if ns != nil {
				existing.ResourceNS = *ns
			}
			if name != nil {
				existing.ResourceName = *name
			}
			_ = json.Unmarshal(labelsRaw, &existing.AlertLabels)
			return existing, nil
		}
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO incidents (cluster_id, title, status, resource_kind, resource_ns, resource_name, alert_labels, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, cluster_id, title, status, resource_kind, resource_ns, resource_name, alert_labels, started_at
	`, inc.ClusterID, inc.Title, inc.Status, nullStr(inc.ResourceKind), nullStr(inc.ResourceNS), nullStr(inc.ResourceName), labels, inc.StartedAt)

	var out models.Incident
	var kind, ns, name *string
	var labelsRaw []byte
	if err := row.Scan(&out.ID, &out.ClusterID, &out.Title, &out.Status, &kind, &ns, &name, &labelsRaw, &out.StartedAt); err != nil {
		return out, err
	}
	if kind != nil {
		out.ResourceKind = *kind
	}
	if ns != nil {
		out.ResourceNS = *ns
	}
	if name != nil {
		out.ResourceName = *name
	}
	_ = json.Unmarshal(labelsRaw, &out.AlertLabels)
	return out, nil
}

func (s *Store) ListIncidents(ctx context.Context, clusterID, status string, limit int, includeDemo bool) ([]models.Incident, error) {
	if limit <= 0 {
		limit = 50
	}
	if status == "" {
		status = "open"
	}
	query := `
		SELECT id, cluster_id, title, status, resource_kind, resource_ns, resource_name, alert_labels, started_at
		FROM incidents WHERE cluster_id = $1 AND status = $2`
	args := []any{clusterID, status}
	if !includeDemo {
		query += ` AND (alert_labels->>'demo_seed' IS NULL OR alert_labels->>'demo_seed' <> 'true')`
	}
	query += ` ORDER BY started_at DESC LIMIT $3`
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []models.Incident
	for rows.Next() {
		var inc models.Incident
		var kind, ns, name *string
		var labelsRaw []byte
		if err := rows.Scan(&inc.ID, &inc.ClusterID, &inc.Title, &inc.Status, &kind, &ns, &name, &labelsRaw, &inc.StartedAt); err != nil {
			return nil, err
		}
		if kind != nil {
			inc.ResourceKind = *kind
		}
		if ns != nil {
			inc.ResourceNS = *ns
		}
		if name != nil {
			inc.ResourceName = *name
		}
		_ = json.Unmarshal(labelsRaw, &inc.AlertLabels)
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
