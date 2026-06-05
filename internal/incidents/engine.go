package incidents

import (
	"context"
	"fmt"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/health"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Engine struct {
	clusterID string
	store     ports.IncidentRepository
	notify    Notifier
}

type Notifier interface {
	Publish(eventType string, payload any)
}

func NewEngine(clusterID string, store ports.IncidentRepository, notify Notifier) *Engine {
	return &Engine{clusterID: clusterID, store: store, notify: notify}
}

func (e *Engine) HandleTransition(ctx context.Context, t health.Transition) error {
	if t.Previous == domain.HealthHealthy && (t.Current == domain.HealthWarning || t.Current == domain.HealthCritical) {
		return e.openIncident(ctx, t)
	}
	if t.Previous != domain.HealthHealthy && t.Current == domain.HealthHealthy {
		return e.resolveForResource(ctx, t.Resource.ID)
	}
	if t.Current == domain.HealthCritical && t.Previous == domain.HealthWarning {
		open, err := e.store.GetOpenAtlasIncidentForResource(ctx, t.Resource.ID)
		if err == nil && open != nil {
			_ = e.store.UpdateAtlasIncidentStatus(ctx, open.ID, domain.IncidentInvestigating)
		}
	}
	return nil
}

func (e *Engine) openIncident(ctx context.Context, t health.Transition) error {
	existing, err := e.store.GetOpenAtlasIncidentForResource(ctx, t.Resource.ID)
	if err == nil && existing != nil {
		return nil
	}
	sev := domain.SeverityWarning
	if t.Current == domain.HealthCritical {
		sev = domain.SeverityCritical
	}
	hb := t.Previous
	inc := &domain.AtlasIncident{
		ClusterID:    e.clusterID,
		ResourceID:   t.Resource.ID,
		Title:        fmt.Sprintf("%s/%s %s", t.Resource.Namespace, t.Resource.Name, t.Resource.Kind),
		Severity:     sev,
		Status:       domain.IncidentOpen,
		Reason:       t.Health.Reason,
		HealthBefore: &hb,
		HealthAfter:  t.Current,
	}
	if err := e.store.CreateAtlasIncident(ctx, inc); err != nil {
		return err
	}
	if e.notify != nil {
		e.notify.Publish("incident.opened", map[string]any{"id": inc.ID, "severity": inc.Severity})
	}
	return nil
}

func (e *Engine) resolveForResource(ctx context.Context, resourceID string) error {
	open, err := e.store.GetOpenAtlasIncidentForResource(ctx, resourceID)
	if err != nil || open == nil {
		return err
	}
	if err := e.store.ResolveAtlasIncident(ctx, open.ID); err != nil {
		return err
	}
	if e.notify != nil {
		e.notify.Publish("incident.resolved", map[string]any{"id": open.ID})
	}
	return nil
}
