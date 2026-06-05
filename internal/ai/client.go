package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

// Service calls the Python/FastAPI AI microservice for investigation and remediation.
type Service struct {
	baseURL string
	http    *http.Client
	store   ports.AtlasStore
}

func NewService(baseURL string, store ports.AtlasStore, timeout time.Duration) *Service {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &Service{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
		store:   store,
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.baseURL != ""
}

func (s *Service) Investigate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, ic *domain.IncidentContext) (*domain.AIInvestigation, error) {
	var resp investigationResponse
	if err := s.post(ctx, "/v1/investigate", map[string]any{
		"incident": incidentPayload(inc),
		"resource": resourcePayload(resource),
		"context":  contextPayload(ic),
	}, &resp); err != nil {
		return s.saveErrorInvestigation(ctx, inc, resource, err)
	}
	ev, _ := json.Marshal(resp.Evidence)
	inv := &domain.AIInvestigation{
		IncidentID:       inc.ID,
		Summary:          resp.Summary,
		RootCause:        resp.RootCause,
		ConfidenceScore: resp.ConfidenceScore,
		ImpactAssessment: resp.ImpactAssessment,
		Evidence:         ev,
		RecommendedFix:   resp.RecommendedFix,
		ModelVersion:     resp.ModelVersion,
	}
	if err := s.store.SaveInvestigation(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *Service) saveErrorInvestigation(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, err error) (*domain.AIInvestigation, error) {
	summary := "AI investigation unavailable"
	rootCause := fmt.Sprintf("AI service unavailable or failed: %v", err)
	evidence, _ := json.Marshal([]map[string]string{{"source": "ai_service", "detail": err.Error()}})
	inv := &domain.AIInvestigation{
		IncidentID:       inc.ID,
		Summary:          summary,
		RootCause:        rootCause,
		ConfidenceScore:  0.0,
		ImpactAssessment: fmt.Sprintf("Investigation could not be completed for %s %s/%s.", resource.Kind, resource.Namespace, resource.Name),
		Evidence:         evidence,
		RecommendedFix:   "Confirm AI_SERVICE_URL and AI service availability, then retry investigation.",
		ModelVersion:     "kubeatlas-ai-service-error",
	}
	if err := s.store.SaveInvestigation(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *Service) Generate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, inv *domain.AIInvestigation) ([]domain.RemediationRecommendation, error) {
	var resp remediateResponse
	if err := s.post(ctx, "/v1/remediate", map[string]any{
		"incident":      incidentPayload(inc),
		"resource":      resourcePayload(resource),
		"investigation": investigationRef(inv),
	}, &resp); err != nil {
		return nil, err
	}
	var out []domain.RemediationRecommendation
	for _, r := range resp.Recommendations {
		params, _ := json.Marshal(r.Parameters)
		rec := domain.RemediationRecommendation{
			IncidentID:       inc.ID,
			InvestigationID:  inv.ID,
			ActionType:       r.ActionType,
			Reason:           r.Reason,
			ConfidenceScore:  r.ConfidenceScore,
			RiskScore:        r.RiskScore,
			ExpectedOutcome:  r.ExpectedOutcome,
			Parameters:       params,
			Status:           domain.ActionPending,
		}
		if err := s.store.SaveRecommendation(ctx, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func (s *Service) post(ctx context.Context, path string, body any, dest any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("ai service %s: %w", path, err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("ai service %s: HTTP %d: %s", path, res.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("ai service decode: %w", err)
	}
	return nil
}

type investigationResponse struct {
	Summary          string           `json:"summary"`
	RootCause        string           `json:"root_cause"`
	ConfidenceScore  float64          `json:"confidence_score"`
	ImpactAssessment string           `json:"impact_assessment"`
	Evidence         []map[string]string `json:"evidence"`
	RecommendedFix   string           `json:"recommended_fix"`
	ModelVersion     string           `json:"model_version"`
}

type remediateResponse struct {
	Recommendations []remediationItem `json:"recommendations"`
}

type remediationItem struct {
	ActionType      string            `json:"action_type"`
	Reason          string            `json:"reason"`
	ConfidenceScore float64           `json:"confidence_score"`
	RiskScore       float64           `json:"risk_score"`
	ExpectedOutcome string            `json:"expected_outcome"`
	Parameters      map[string]string `json:"parameters"`
}

func incidentPayload(inc *domain.AtlasIncident) map[string]any {
	return map[string]any{
		"id": inc.ID, "cluster_id": inc.ClusterID, "resource_id": inc.ResourceID,
		"title": inc.Title, "severity": inc.Severity, "status": inc.Status,
		"reason": inc.Reason, "health_after": inc.HealthAfter,
	}
}

func resourcePayload(r *domain.ClusterResource) map[string]any {
	return map[string]any{
		"id": r.ID, "cluster_id": r.ClusterID, "kind": r.Kind,
		"namespace": r.Namespace, "name": r.Name, "resource_uid": r.ResourceUID,
		"node_name": r.NodeName, "owner_kind": r.OwnerKind, "owner_name": r.OwnerName,
	}
}

func contextPayload(c *domain.IncidentContext) map[string]any {
	var logs any = []any{}
	var events any = []any{}
	var images any = []any{}
	var env any = []any{}
	var mounts any = []any{}

	var describe map[string]any = map[string]any{}
	var rs map[string]any = map[string]any{}
	var node map[string]any = map[string]any{}

	if len(c.Logs) > 0 {
		_ = json.Unmarshal(c.Logs, &logs)
		if logs == nil {
			logs = []any{}
		}
	}

	if len(c.Events) > 0 {
		_ = json.Unmarshal(c.Events, &events)
		if events == nil {
			events = []any{}
		}
	}

	if len(c.ImageDetails) > 0 {
		_ = json.Unmarshal(c.ImageDetails, &images)
		if images == nil {
			images = []any{}
		}
	}

	if len(c.EnvVars) > 0 {
		_ = json.Unmarshal(c.EnvVars, &env)
		if env == nil {
			env = []any{}
		}
	}

	if len(c.VolumeMounts) > 0 {
		_ = json.Unmarshal(c.VolumeMounts, &mounts)
		if mounts == nil {
			mounts = []any{}
		}
	}

	if len(c.DescribeData) > 0 {
		_ = json.Unmarshal(c.DescribeData, &describe)
		if describe == nil {
			describe = map[string]any{}
		}
	}

	if len(c.ReplicaSetInfo) > 0 {
		_ = json.Unmarshal(c.ReplicaSetInfo, &rs)
		if rs == nil {
			rs = map[string]any{}
		}
	}

	if len(c.NodeInfo) > 0 {
		_ = json.Unmarshal(c.NodeInfo, &node)
		if node == nil {
			node = map[string]any{}
		}
	}

	fmt.Printf("env=%#v\n", env)

	return map[string]any{
		"incident_id":     c.IncidentID,
		"logs":            logs,
		"events":          events,
		"describe_data":   describe,
		"deployment_yaml": c.DeploymentYAML,
		"replicaset_info": rs,
		"node_info":       node,
		"restart_count":   c.RestartCount,
		"image_details":   images,
		"env_vars":        env,
		"volume_mounts":   mounts,
	}
}

func investigationRef(inv *domain.AIInvestigation) map[string]any {
	var evidence any
	_ = json.Unmarshal(inv.Evidence, &evidence)
	return map[string]any{
		"id": inv.ID, "incident_id": inv.IncidentID, "summary": inv.Summary,
		"root_cause": inv.RootCause, "confidence_score": inv.ConfidenceScore,
		"impact_assessment": inv.ImpactAssessment, "evidence": evidence,
		"recommended_fix": inv.RecommendedFix, "model_version": inv.ModelVersion,
	}
}

// Ping checks AI service health (optional startup validation).
func (s *Service) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ai health: HTTP %d", res.StatusCode)
	}
	return nil
}
