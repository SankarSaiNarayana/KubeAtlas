package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
)

var _ = (*Store)(nil) // Store implements atlas via methods below

func (s *Store) UpsertResource(ctx context.Context, r *domain.ClusterResource) error {
	labels := defaultJSON(r.Labels)
	spec := defaultJSON(r.SpecSnapshot)
	status := defaultJSON(r.StatusSnapshot)
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO cluster_resources (
			cluster_id, resource_uid, api_version, kind, namespace, name,
			labels, spec_snapshot, status_snapshot, node_name, owner_kind, owner_name, updated_at, deleted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NULL)
		ON CONFLICT (cluster_id, resource_uid) DO UPDATE SET
			api_version = EXCLUDED.api_version,
			kind = EXCLUDED.kind,
			namespace = EXCLUDED.namespace,
			name = EXCLUDED.name,
			labels = EXCLUDED.labels,
			spec_snapshot = EXCLUDED.spec_snapshot,
			status_snapshot = EXCLUDED.status_snapshot,
			node_name = EXCLUDED.node_name,
			owner_kind = EXCLUDED.owner_kind,
			owner_name = EXCLUDED.owner_name,
			updated_at = NOW(),
			deleted_at = NULL
		RETURNING id
	`, r.ClusterID, r.ResourceUID, r.APIVersion, r.Kind, r.Namespace, r.Name,
		labels, spec, status, nullStr(r.NodeName), nullStr(r.OwnerKind), nullStr(r.OwnerName)).Scan(&id)
	if err != nil {
		return err
	}
	r.ID = id.String()
	return nil
}

func (s *Store) MarkResourceDeleted(ctx context.Context, clusterID, resourceUID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE cluster_resources SET deleted_at = NOW(), updated_at = NOW()
		WHERE cluster_id = $1 AND resource_uid = $2
	`, clusterID, resourceUID)
	return err
}

func (s *Store) GetResourceByUID(ctx context.Context, clusterID, resourceUID string) (*domain.ClusterResource, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name,
			labels, spec_snapshot, status_snapshot, COALESCE(node_name,''), COALESCE(owner_kind,''), COALESCE(owner_name,''),
			created_at, updated_at, deleted_at
		FROM cluster_resources WHERE cluster_id = $1 AND resource_uid = $2
	`, clusterID, resourceUID)
	return scanResource(row)
}

func (s *Store) GetResource(ctx context.Context, id string) (*domain.ClusterResource, error) {
	rid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, resource_uid, api_version, kind, namespace, name,
			labels, spec_snapshot, status_snapshot, COALESCE(node_name,''), COALESCE(owner_kind,''), COALESCE(owner_name,''),
			created_at, updated_at, deleted_at
		FROM cluster_resources WHERE id = $1
	`, rid)
	return scanResource(row)
}

func (s *Store) ListResources(ctx context.Context, clusterID, kind, namespace string, limit int) ([]domain.ClusterResource, error) {
	if limit <= 0 {
		limit = 500
	}
	q := `
		SELECT r.id, r.cluster_id, r.resource_uid, r.api_version, r.kind, r.namespace, r.name,
			r.labels, r.spec_snapshot, r.status_snapshot, COALESCE(r.node_name,''), COALESCE(r.owner_kind,''), COALESCE(r.owner_name,''),
			r.created_at, r.updated_at, r.deleted_at,
			h.id, h.cluster_id, h.resource_id, h.health::text, h.reason, h.details, h.evaluated_at
		FROM cluster_resources r
		LEFT JOIN resource_health h ON h.resource_id = r.id
		WHERE r.cluster_id = $1 AND r.deleted_at IS NULL`
	args := []any{clusterID}
	n := 2
	if kind != "" {
		q += fmt.Sprintf(" AND r.kind = $%d", n)
		args = append(args, kind)
		n++
	}
	if namespace != "" {
		q += fmt.Sprintf(" AND r.namespace = $%d", n)
		args = append(args, namespace)
		n++
	}
	q += fmt.Sprintf(" ORDER BY r.kind, r.namespace, r.name LIMIT $%d", n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ClusterResource
	for rows.Next() {
		var r domain.ClusterResource
		var labels, spec, status []byte
		var deleted *time.Time
		var hID, hResID *uuid.UUID
		var hClusterID *string
		var hHealth, hReason *string
		var hDetails []byte
		var hEval *time.Time
		if err := rows.Scan(
			&r.ID, &r.ClusterID, &r.ResourceUID, &r.APIVersion, &r.Kind, &r.Namespace, &r.Name,
			&labels, &spec, &status, &r.NodeName, &r.OwnerKind, &r.OwnerName,
			&r.CreatedAt, &r.UpdatedAt, &deleted,
			&hID, &hClusterID, &hResID, &hHealth, &hReason, &hDetails, &hEval,
		); err != nil {
			return nil, err
		}
		r.Labels = labels
		r.SpecSnapshot = spec
		r.StatusSnapshot = status
		r.DeletedAt = deleted
		if hID != nil && hHealth != nil && hClusterID != nil && hResID != nil {
			r.Health = &domain.ResourceHealth{
				ID: hID.String(), ClusterID: *hClusterID, ResourceID: hResID.String(),
				Health: domain.HealthState(*hHealth), Reason: *hReason, Details: hDetails, EvaluatedAt: *hEval,
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpsertHealth(ctx context.Context, h *domain.ResourceHealth) (domain.HealthState, error) {
	prev := domain.HealthHealthy
	if existing, err := s.GetHealth(ctx, h.ResourceID); err == nil {
		prev = existing.Health
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return prev, err
	}
	details := defaultJSON(h.Details)
	rid, err := uuid.Parse(h.ResourceID)
	if err != nil {
		return prev, err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO resource_health (cluster_id, resource_id, health, reason, details, evaluated_at)
		VALUES ($2, $1, $3::health_state, $4, $5, NOW())
		ON CONFLICT (resource_id) DO UPDATE SET
			health = EXCLUDED.health,
			reason = EXCLUDED.reason,
			details = EXCLUDED.details,
			evaluated_at = NOW()
		RETURNING id
	`, rid, h.ClusterID, string(h.Health), h.Reason, details).Scan(&id)
	if err != nil {
		return prev, err
	}
	h.ID = id.String()
	return prev, nil
}

func (s *Store) GetHealth(ctx context.Context, resourceID string) (*domain.ResourceHealth, error) {
	rid, err := uuid.Parse(resourceID)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, resource_id::text, health::text, reason, details, evaluated_at
		FROM resource_health WHERE resource_id = $1
	`, rid)
	var h domain.ResourceHealth
	var health string
	if err := row.Scan(&h.ID, &h.ClusterID, &h.ResourceID, &health, &h.Reason, &h.Details, &h.EvaluatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	h.Health = domain.HealthState(health)
	return &h, nil
}

func (s *Store) CreateAtlasIncident(ctx context.Context, inc *domain.AtlasIncident) error {
	var hb *string
	if inc.HealthBefore != nil {
		s := string(*inc.HealthBefore)
		hb = &s
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO atlas_incidents (
			cluster_id, resource_id, title, severity, status, reason, health_before, health_after
		) VALUES ($1,$2,$3,$4::incident_severity,$5::incident_status,$6,$7::health_state,$8::health_state)
		RETURNING id, opened_at, updated_at
	`, inc.ClusterID, inc.ResourceID, inc.Title, string(inc.Severity), string(inc.Status), inc.Reason,
		hb, string(inc.HealthAfter)).Scan(&id, &inc.OpenedAt, &inc.UpdatedAt)
	if err != nil {
		return err
	}
	inc.ID = id.String()
	return nil
}

func (s *Store) GetOpenAtlasIncidentForResource(ctx context.Context, resourceID string) (*domain.AtlasIncident, error) {
	rid, err := uuid.Parse(resourceID)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, resource_id, title, severity::text, status::text, reason,
			health_before::text, health_after::text, opened_at, resolved_at, updated_at
		FROM atlas_incidents
		WHERE resource_id = $1 AND status <> 'resolved'
		ORDER BY opened_at DESC LIMIT 1
	`, rid)
	return scanIncident(row)
}

func (s *Store) ResolveAtlasIncident(ctx context.Context, incidentID string) error {
	iid, err := uuid.Parse(incidentID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE atlas_incidents SET status = 'resolved', resolved_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, iid)
	return err
}

func (s *Store) UpdateAtlasIncidentStatus(ctx context.Context, incidentID string, status domain.IncidentStatus) error {
	iid, err := uuid.Parse(incidentID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE atlas_incidents SET status = $2::incident_status, updated_at = NOW() WHERE id = $1
	`, iid, string(status))
	return err
}

func (s *Store) GetAtlasIncident(ctx context.Context, id string) (*domain.AtlasIncident, error) {
	iid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, resource_id, title, severity::text, status::text, reason,
			health_before::text, health_after::text, opened_at, resolved_at, updated_at
		FROM atlas_incidents WHERE id = $1
	`, iid)
	return scanIncident(row)
}

func (s *Store) ListAtlasIncidents(ctx context.Context, clusterID, status string, limit int) ([]domain.AtlasIncident, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `
		SELECT i.id, i.cluster_id, i.resource_id, i.title, i.severity::text, i.status::text, i.reason,
			i.health_before::text, i.health_after::text, i.opened_at, i.resolved_at, i.updated_at
		FROM atlas_incidents i WHERE i.cluster_id = $1`
	args := []any{clusterID}
	n := 2
	if status != "" && status != "all" {
		q += fmt.Sprintf(` AND i.status = $%d::incident_status`, n)
		args = append(args, status)
		n++
	}
	q += fmt.Sprintf(` ORDER BY i.opened_at DESC LIMIT $%d`, n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AtlasIncident
	for rows.Next() {
		inc, err := scanIncidentRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inc)
	}
	return out, rows.Err()
}

func (s *Store) SaveContext(ctx context.Context, c *domain.IncidentContext) error {
	iid, err := uuid.Parse(c.IncidentID)
	if err != nil {
		return err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO incident_context (
			incident_id, logs, events, describe_data, deployment_yaml, replicaset_info,
			node_info, restart_count, image_details, env_vars, volume_mounts, collected_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
		ON CONFLICT (incident_id) DO UPDATE SET
			logs = EXCLUDED.logs, events = EXCLUDED.events, describe_data = EXCLUDED.describe_data,
			deployment_yaml = EXCLUDED.deployment_yaml, replicaset_info = EXCLUDED.replicaset_info,
			node_info = EXCLUDED.node_info, restart_count = EXCLUDED.restart_count,
			image_details = EXCLUDED.image_details, env_vars = EXCLUDED.env_vars,
			volume_mounts = EXCLUDED.volume_mounts, collected_at = NOW()
		RETURNING id, collected_at
	`, iid, defaultJSON(c.Logs), defaultJSON(c.Events), defaultJSON(c.DescribeData),
		nullStr(c.DeploymentYAML), defaultJSON(c.ReplicaSetInfo), defaultJSON(c.NodeInfo),
		c.RestartCount, defaultJSON(c.ImageDetails), defaultJSON(c.EnvVars), defaultJSON(c.VolumeMounts),
	).Scan(&id, &c.CollectedAt)
	if err != nil {
		return err
	}
	c.ID = id.String()
	return nil
}

func (s *Store) GetContext(ctx context.Context, incidentID string) (*domain.IncidentContext, error) {
	iid, err := uuid.Parse(incidentID)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, incident_id, logs, events, describe_data, COALESCE(deployment_yaml,''),
			replicaset_info, node_info, restart_count, image_details, env_vars, volume_mounts, collected_at
		FROM incident_context WHERE incident_id = $1
	`, iid)
	var c domain.IncidentContext
	if err := row.Scan(&c.ID, &c.IncidentID, &c.Logs, &c.Events, &c.DescribeData, &c.DeploymentYAML,
		&c.ReplicaSetInfo, &c.NodeInfo, &c.RestartCount, &c.ImageDetails, &c.EnvVars, &c.VolumeMounts, &c.CollectedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) SaveInvestigation(ctx context.Context, inv *domain.AIInvestigation) error {
	iid, err := uuid.Parse(inv.IncidentID)
	if err != nil {
		return err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO ai_investigations (
			incident_id, summary, root_cause, confidence_score, impact_assessment, evidence, recommended_fix, model_version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (incident_id) DO UPDATE SET
			summary = EXCLUDED.summary, root_cause = EXCLUDED.root_cause,
			confidence_score = EXCLUDED.confidence_score, impact_assessment = EXCLUDED.impact_assessment,
			evidence = EXCLUDED.evidence, recommended_fix = EXCLUDED.recommended_fix,
			model_version = EXCLUDED.model_version, investigated_at = NOW()
		RETURNING id, investigated_at
	`, iid, inv.Summary, inv.RootCause, inv.ConfidenceScore, inv.ImpactAssessment,
		defaultJSON(inv.Evidence), inv.RecommendedFix, inv.ModelVersion).Scan(&id, &inv.InvestigatedAt)
	if err != nil {
		return err
	}
	inv.ID = id.String()
	return nil
}

func (s *Store) GetInvestigation(ctx context.Context, incidentID string) (*domain.AIInvestigation, error) {
	iid, err := uuid.Parse(incidentID)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, incident_id, summary, root_cause, confidence_score, impact_assessment, evidence, recommended_fix, model_version, investigated_at
		FROM ai_investigations WHERE incident_id = $1
	`, iid)
	var inv domain.AIInvestigation
	if err := row.Scan(&inv.ID, &inv.IncidentID, &inv.Summary, &inv.RootCause, &inv.ConfidenceScore,
		&inv.ImpactAssessment, &inv.Evidence, &inv.RecommendedFix, &inv.ModelVersion, &inv.InvestigatedAt); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) SaveRecommendation(ctx context.Context, rec *domain.RemediationRecommendation) error {
	iid, err := uuid.Parse(rec.IncidentID)
	if err != nil {
		return err
	}
	var invID *uuid.UUID
	if rec.InvestigationID != "" {
		u, err := uuid.Parse(rec.InvestigationID)
		if err == nil {
			invID = &u
		}
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO remediation_recommendations (
			incident_id, investigation_id, action_type, reason, confidence_score, risk_score, expected_outcome, parameters, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::action_status)
		RETURNING id, created_at
	`, iid, invID, rec.ActionType, rec.Reason, rec.ConfidenceScore, rec.RiskScore, rec.ExpectedOutcome,
		defaultJSON(rec.Parameters), string(rec.Status)).Scan(&id, &rec.CreatedAt)
	if err != nil {
		return err
	}
	rec.ID = id.String()
	if rec.Status == domain.ActionPending {
		_, _ = s.pool.Exec(ctx, `INSERT INTO pending_actions (recommendation_id) VALUES ($1) ON CONFLICT DO NOTHING`, id)
	}
	return nil
}

func (s *Store) GetRecommendation(ctx context.Context, id string) (*domain.RemediationRecommendation, error) {
	rid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, incident_id, COALESCE(investigation_id::text,''), action_type, reason,
			confidence_score, risk_score, expected_outcome, parameters, status::text, created_at
		FROM remediation_recommendations WHERE id = $1
	`, rid)
	return scanRecommendation(row)
}

func (s *Store) ListRecommendations(ctx context.Context, incidentID string) ([]domain.RemediationRecommendation, error) {
	iid, err := uuid.Parse(incidentID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, incident_id, COALESCE(investigation_id::text,''), action_type, reason,
			confidence_score, risk_score, expected_outcome, parameters, status::text, created_at
		FROM remediation_recommendations WHERE incident_id = $1 ORDER BY created_at
	`, iid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecommendationRows(rows)
}

func (s *Store) ListByStatus(ctx context.Context, clusterID string, status domain.ActionStatus, limit int) ([]domain.RemediationRecommendation, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.incident_id, COALESCE(r.investigation_id::text,''), r.action_type, r.reason,
			r.confidence_score, r.risk_score, r.expected_outcome, r.parameters, r.status::text, r.created_at
		FROM remediation_recommendations r
		JOIN atlas_incidents i ON i.id = r.incident_id
		WHERE i.cluster_id = $1 AND r.status = $2::action_status
		ORDER BY r.created_at DESC LIMIT $3
	`, clusterID, string(status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecommendationRows(rows)
}

func (s *Store) UpdateRecommendationStatus(ctx context.Context, id string, status domain.ActionStatus) error {
	rid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE remediation_recommendations SET status = $2::action_status WHERE id = $1`, rid, string(status))
	return err
}

func (s *Store) CreatePending(ctx context.Context, recommendationID, requestedBy string) error {
	rid, err := uuid.Parse(recommendationID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO pending_actions (recommendation_id, requested_by) VALUES ($1, $2)
		ON CONFLICT (recommendation_id) DO NOTHING
	`, rid, requestedBy)
	return err
}

func (s *Store) Approve(ctx context.Context, recommendationID, approvedBy string) error {
	rid, err := uuid.Parse(recommendationID)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO approved_actions (recommendation_id, approved_by) VALUES ($1, $2)
		ON CONFLICT (recommendation_id) DO UPDATE SET approved_by = EXCLUDED.approved_by, approved_at = NOW()
	`, rid, approvedBy)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE remediation_recommendations SET status = 'approved' WHERE id = $1`, rid)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM pending_actions WHERE recommendation_id = $1`, rid)
	return tx.Commit(ctx)
}

func (s *Store) IsApproved(ctx context.Context, recommendationID string) (bool, string, error) {
	rid, err := uuid.Parse(recommendationID)
	if err != nil {
		return false, "", err
	}
	var by string
	err = s.pool.QueryRow(ctx, `SELECT approved_by FROM approved_actions WHERE recommendation_id = $1`, rid).Scan(&by)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	return err == nil, by, err
}

func (s *Store) SaveExecution(ctx context.Context, rec *domain.ExecutionRecord) error {
	rid, err := uuid.Parse(rec.RecommendationID)
	if err != nil {
		return err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO execution_history (
			cluster_id, recommendation_id, approved_by, action_type, parameters,
			success, failure_reason, rolled_back, rollback_reason, started_at, completed_at, duration_ms
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id
	`, rec.ClusterID, rid, rec.ApprovedBy, rec.ActionType, defaultJSON(rec.Parameters),
		rec.Success, nullStr(rec.FailureReason), rec.RolledBack, nullStr(rec.RollbackReason),
		rec.StartedAt, rec.CompletedAt, rec.DurationMs).Scan(&id)
	if err != nil {
		return err
	}
	rec.ID = id.String()
	st := domain.ActionFailed
	if rec.Success {
		st = domain.ActionSucceeded
	}
	_, _ = s.pool.Exec(ctx, `UPDATE remediation_recommendations SET status = $2::action_status WHERE id = $1`, rid, string(st))
	return nil
}

func (s *Store) ListExecutions(ctx context.Context, clusterID string, limit int) ([]domain.ExecutionRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, cluster_id, recommendation_id, approved_by, action_type, parameters,
			success, COALESCE(failure_reason,''), rolled_back, COALESCE(rollback_reason,''),
			started_at, completed_at, COALESCE(duration_ms,0)
		FROM execution_history WHERE cluster_id = $1 ORDER BY started_at DESC LIMIT $2
	`, clusterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ExecutionRecord
	for rows.Next() {
		var e domain.ExecutionRecord
		var recID uuid.UUID
		if err := rows.Scan(&e.ID, &e.ClusterID, &recID, &e.ApprovedBy, &e.ActionType, &e.Parameters,
			&e.Success, &e.FailureReason, &e.RolledBack, &e.RollbackReason, &e.StartedAt, &e.CompletedAt, &e.DurationMs); err != nil {
			return nil, err
		}
		e.RecommendationID = recID.String()
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetOverview(ctx context.Context, clusterID string) (*domain.OverviewStats, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM cluster_resources WHERE cluster_id = $1 AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM resource_health h JOIN cluster_resources r ON r.id = h.resource_id WHERE r.cluster_id = $1 AND h.health = 'HEALTHY'),
			(SELECT COUNT(*) FROM resource_health h JOIN cluster_resources r ON r.id = h.resource_id WHERE r.cluster_id = $1 AND h.health = 'WARNING'),
			(SELECT COUNT(*) FROM resource_health h JOIN cluster_resources r ON r.id = h.resource_id WHERE r.cluster_id = $1 AND h.health = 'CRITICAL'),
			(SELECT COUNT(*) FROM atlas_incidents WHERE cluster_id = $1 AND status <> 'resolved'),
			(SELECT COUNT(*) FROM atlas_incidents WHERE cluster_id = $1 AND status = 'resolved')
	`, clusterID)
	var o domain.OverviewStats
	err := row.Scan(&o.TotalResources, &o.HealthyResources, &o.WarningResources, &o.CriticalResources, &o.OpenIncidents, &o.ResolvedIncidents)
	return &o, err
}

func defaultJSON(b json.RawMessage) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func scanResource(row pgx.Row) (*domain.ClusterResource, error) {
	var r domain.ClusterResource
	var labels, spec, status []byte
	var deleted *time.Time
	err := row.Scan(
		&r.ID, &r.ClusterID, &r.ResourceUID, &r.APIVersion, &r.Kind, &r.Namespace, &r.Name,
		&labels, &spec, &status, &r.NodeName, &r.OwnerKind, &r.OwnerName,
		&r.CreatedAt, &r.UpdatedAt, &deleted,
	)
	if err != nil {
		return nil, err
	}
	r.Labels = labels
	r.SpecSnapshot = spec
	r.StatusSnapshot = status
	r.DeletedAt = deleted
	return &r, nil
}

func scanIncident(row pgx.Row) (*domain.AtlasIncident, error) {
	var inc domain.AtlasIncident
	var sev, st, ha string
	var hb *string
	var resolved *time.Time
	err := row.Scan(&inc.ID, &inc.ClusterID, &inc.ResourceID, &inc.Title, &sev, &st, &inc.Reason,
		&hb, &ha, &inc.OpenedAt, &resolved, &inc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	inc.Severity = domain.IncidentSeverity(sev)
	inc.Status = domain.IncidentStatus(st)
	if hb != nil {
		h := domain.HealthState(*hb)
		inc.HealthBefore = &h
	}
	inc.HealthAfter = domain.HealthState(ha)
	inc.ResolvedAt = resolved
	return &inc, nil
}

type incidentScanner interface {
	Scan(dest ...any) error
}

func scanIncidentRow(rows incidentScanner) (*domain.AtlasIncident, error) {
	var inc domain.AtlasIncident
	var sev, st, ha string
	var hb *string
	var resolved *time.Time
	err := rows.Scan(&inc.ID, &inc.ClusterID, &inc.ResourceID, &inc.Title, &sev, &st, &inc.Reason,
		&hb, &ha, &inc.OpenedAt, &resolved, &inc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	inc.Severity = domain.IncidentSeverity(sev)
	inc.Status = domain.IncidentStatus(st)
	if hb != nil {
		h := domain.HealthState(*hb)
		inc.HealthBefore = &h
	}
	inc.HealthAfter = domain.HealthState(ha)
	inc.ResolvedAt = resolved
	return &inc, nil
}

func scanRecommendation(row pgx.Row) (*domain.RemediationRecommendation, error) {
	var rec domain.RemediationRecommendation
	var st string
	err := row.Scan(&rec.ID, &rec.IncidentID, &rec.InvestigationID, &rec.ActionType, &rec.Reason,
		&rec.ConfidenceScore, &rec.RiskScore, &rec.ExpectedOutcome, &rec.Parameters, &st, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	rec.Status = domain.ActionStatus(st)
	return &rec, nil
}

func scanRecommendationRows(rows pgx.Rows) ([]domain.RemediationRecommendation, error) {
	var out []domain.RemediationRecommendation
	for rows.Next() {
		var rec domain.RemediationRecommendation
		var st string
		if err := rows.Scan(&rec.ID, &rec.IncidentID, &rec.InvestigationID, &rec.ActionType, &rec.Reason,
			&rec.ConfidenceScore, &rec.RiskScore, &rec.ExpectedOutcome, &rec.Parameters, &st, &rec.CreatedAt); err != nil {
			return nil, err
		}
		rec.Status = domain.ActionStatus(st)
		out = append(out, rec)
	}
	return out, rows.Err()
}
