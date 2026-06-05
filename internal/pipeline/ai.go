package pipeline

import (
	"context"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
)

// IncidentInvestigator runs AI analysis and persists results.
type IncidentInvestigator interface {
	Investigate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, ic *domain.IncidentContext) (*domain.AIInvestigation, error)
}

// RemediationGenerator proposes actions and persists recommendations.
type RemediationGenerator interface {
	Generate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, inv *domain.AIInvestigation) ([]domain.RemediationRecommendation, error)
}
