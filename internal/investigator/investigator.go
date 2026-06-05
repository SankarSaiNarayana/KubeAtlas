package investigator

import (
	"context"
	"fmt"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Investigator interface {
	Investigate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, ic *domain.IncidentContext) (*domain.AIInvestigation, error)
}

type Engine struct {
	store ports.InvestigationRepository
}

func NewEngine(store ports.InvestigationRepository) *Engine {
	return &Engine{store: store}
}

func (e *Engine) Investigate(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource, ic *domain.IncidentContext) (*domain.AIInvestigation, error) {
	// AI investigation is now delegated to the Python AI service
	// This method is reserved for LLM-based analysis only
	return nil, fmt.Errorf("AI investigation requires the Python LLM service to be running")
}
