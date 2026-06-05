package remediation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Generator interface {
	Generate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, inv *domain.AIInvestigation) ([]domain.RemediationRecommendation, error)
}

type Engine struct {
	store ports.RemediationRepository
}

func NewEngine(store ports.RemediationRepository) *Engine {
	return &Engine{store: store}
}

func (e *Engine) Generate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, inv *domain.AIInvestigation) ([]domain.RemediationRecommendation, error) {
	recs := buildRecommendations(inc, resource, inv)
	var out []domain.RemediationRecommendation
	for i := range recs {
		recs[i].IncidentID = inc.ID
		recs[i].InvestigationID = inv.ID
		recs[i].Status = domain.ActionPending
		if err := e.store.SaveRecommendation(ctx, &recs[i]); err != nil {
			return nil, err
		}
		out = append(out, recs[i])
	}
	return out, nil
}

func buildRecommendations(inc *domain.AtlasIncident, resource *domain.ClusterResource, inv *domain.AIInvestigation) []domain.RemediationRecommendation {
	var recs []domain.RemediationRecommendation
	reason := strings.ToLower(inc.Reason)
	rc := strings.ToLower(inv.RootCause)

	switch resource.Kind {
	case "Pod":
		if strings.Contains(reason, "crashloop") || strings.Contains(rc, "crash") {
			recs = append(recs, rec(domain.ActionDeleteFailedPod, "Remove failing pod to allow controller recreation",
				0.85, 0.35, "New pod scheduled with fresh state", params(resource)))
			recs = append(recs, rec(domain.ActionRestartPod, "Delete pod to trigger restart by owner",
				0.8, 0.3, "Pod recreated by ReplicaSet/Deployment", params(resource)))
		} else {
			recs = append(recs, rec(domain.ActionRestartPod, "Restart pod to recover transient failure",
				0.7, 0.25, "Pod replaced with same spec", params(resource)))
		}
	case "Deployment":
		if strings.Contains(reason, "replica") || strings.Contains(rc, "replica") {
			recs = append(recs, rec(domain.ActionRestartDeployment, "Rollout restart to recreate pods",
				0.82, 0.45, "Rolling restart of all deployment pods", params(resource)))
			recs = append(recs, rec(domain.ActionRollbackDeployment, "Rollback to previous revision if recent change caused failure",
				0.75, 0.55, "Deployment rolled back to prior ReplicaSet", params(resource)))
		}
		recs = append(recs, rec(domain.ActionScaleDeployment, "Temporary scale up for redundancy during fix",
			0.6, 0.4, "Additional replica may absorb traffic during recovery", scaleParams(resource, 1)))
	default:
		if resource.Kind == "Pod" {
			recs = append(recs, rec(domain.ActionRestartPod, "Restart affected workload", 0.65, 0.3, "Workload restarted", params(resource)))
		}
	}
	return recs
}

func rec(actionType, reason string, conf, risk float64, outcome string, p map[string]string) domain.RemediationRecommendation {
	b, _ := json.Marshal(p)
	return domain.RemediationRecommendation{
		ActionType:      actionType,
		Reason:          reason,
		ConfidenceScore: conf,
		RiskScore:       risk,
		ExpectedOutcome: outcome,
		Parameters:      b,
	}
}

func params(r *domain.ClusterResource) map[string]string {
	return map[string]string{
		"namespace": r.Namespace, "name": r.Name, "kind": r.Kind, "uid": r.ResourceUID,
	}
}

func scaleParams(r *domain.ClusterResource, delta int) map[string]string {
	p := params(r)
	p["scale_delta"] = strconv.Itoa(delta)
	return p
}
