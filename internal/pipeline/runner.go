package pipeline

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	ctxbuilder "github.com/kube-dashboard/kube_dashboard/internal/context"
	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

// Runner executes the manual investigation pipeline (context → AI → remediation).
type Runner struct {
	clusterID    string
	store        ports.AtlasStore
	client       kubernetes.Interface
	context      *ctxbuilder.Builder
	investigator IncidentInvestigator
	remediation  RemediationGenerator
}

func NewRunner(clusterID string, store ports.AtlasStore, client kubernetes.Interface, inv IncidentInvestigator, rem RemediationGenerator) *Runner {
	return &Runner{
		clusterID:    clusterID,
		store:        store,
		client:       client,
		context:      ctxbuilder.NewBuilder(client, store),
		investigator: inv,
		remediation:  rem,
	}
}

// InvestigateIncident collects context, runs AI analysis, and stores remediation suggestions.
func (r *Runner) InvestigateIncident(ctx context.Context, incidentID string) (*domain.AtlasIncident, *domain.AIInvestigation, []domain.RemediationRecommendation, error) {
	inc, err := r.store.GetAtlasIncident(ctx, incidentID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("incident: %w", err)
	}
	if inc.Status == domain.IncidentResolved {
		return nil, nil, nil, fmt.Errorf("incident already verified")
	}
	res, err := r.store.GetResource(ctx, inc.ResourceID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resource: %w", err)
	}
	_ = r.store.UpdateAtlasIncidentStatus(ctx, incidentID, domain.IncidentInvestigating)

	ic, err := r.context.Collect(ctx, inc, res)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("collect context: %w", err)
	}
	inv, err := r.investigator.Investigate(ctx, inc, res, ic)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("investigate: %w", err)
	}
	_ = r.store.ClearRecommendationsForIncident(ctx, incidentID)
	recs, err := r.remediation.Generate(ctx, inc, res, inv)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("remediation: %w", err)
	}
	if len(recs) > 0 {
		_ = r.store.UpdateAtlasIncidentStatus(ctx, incidentID, domain.IncidentAwaitingApproval)
		inc.Status = domain.IncidentAwaitingApproval
	} else {
		inc.Status = domain.IncidentInvestigating
	}
	return inc, inv, recs, nil
}
