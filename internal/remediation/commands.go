package remediation

import (
	"fmt"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
)

// KubectlCommand returns the manual fix command for an action.
func KubectlCommand(actionType string, namespace, name string) string {
	ns := namespace
	if ns == "" {
		ns = "default"
	}
	switch actionType {
	case domain.ActionDeleteFailedPod, domain.ActionRestartPod:
		return fmt.Sprintf("kubectl delete pod %s -n %s", name, ns)
	case domain.ActionRestartDeployment:
		return fmt.Sprintf("kubectl rollout restart deployment/%s -n %s", name, ns)
	case domain.ActionRollbackDeployment:
		return fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", name, ns)
	case domain.ActionScaleDeployment:
		return fmt.Sprintf("kubectl scale deployment/%s -n %s --replicas=2", name, ns)
	default:
		return fmt.Sprintf("kubectl get %s %s -n %s", "pod", name, ns)
	}
}

// EnrichParameters adds kubectl_command when missing.
func EnrichParameters(actionType string, p map[string]string) map[string]string {
	if p == nil {
		p = map[string]string{}
	}
	if p["kubectl_command"] == "" {
		p["kubectl_command"] = KubectlCommand(actionType, p["namespace"], p["name"])
	}
	return p
}

// PickBest returns the single highest-confidence recommendation.
func PickBest(recs []domain.RemediationRecommendation) []domain.RemediationRecommendation {
	if len(recs) == 0 {
		return recs
	}
	best := recs[0]
	for _, r := range recs[1:] {
		if r.ConfidenceScore > best.ConfidenceScore {
			best = r
		}
	}
	return []domain.RemediationRecommendation{best}
}
