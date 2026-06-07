package remediation

import (
	"context"
	"encoding/json"
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
	recs := PickBest(buildRecommendations(inc, resource, inv))
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
	reason := strings.ToLower(inc.Reason)
	rc := strings.ToLower(inv.RootCause)

	switch resource.Kind {
	case "Pod":
		if strings.Contains(reason, "crashloop") || strings.Contains(rc, "crash") {
			return []domain.RemediationRecommendation{
				rec(domain.ActionDeleteFailedPod, "Delete the failing pod so the controller recreates it",
					0.85, 0.35, "A fresh pod will be scheduled", params(resource)),
			}
		}
		return []domain.RemediationRecommendation{
			rec(domain.ActionRestartPod, "Restart the pod to recover from a transient failure",
				0.75, 0.25, "Pod replaced with the same spec", params(resource)),
		}
	case "Deployment":
		if strings.Contains(reason, "replica") || strings.Contains(rc, "replica") {
			return []domain.RemediationRecommendation{
				rec(domain.ActionRestartDeployment, "Rollout restart to recreate unhealthy pods",
					0.82, 0.45, "Deployment pods roll out with a clean state", params(resource)),
			}
		}
		return []domain.RemediationRecommendation{
			rec(domain.ActionRestartDeployment, "Rollout restart the deployment",
				0.7, 0.4, "Pods are recreated gradually", params(resource)),
		}
	default:
		return nil
	}
}

func rec(actionType, reason string, conf, risk float64, outcome string, p map[string]string) domain.RemediationRecommendation {
	p = EnrichParameters(actionType, p)
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
