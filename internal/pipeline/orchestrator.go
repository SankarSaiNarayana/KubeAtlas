package pipeline

import (
	"context"
	"log"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/kube-dashboard/kube_dashboard/internal/collector"
	ctxbuilder "github.com/kube-dashboard/kube_dashboard/internal/context"
	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/execution"
	"github.com/kube-dashboard/kube_dashboard/internal/health"
	"github.com/kube-dashboard/kube_dashboard/internal/incidents"
	"github.com/kube-dashboard/kube_dashboard/internal/investigator"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
	"github.com/kube-dashboard/kube_dashboard/internal/remediation"
	"github.com/kube-dashboard/kube_dashboard/internal/realtime"
)

// DefaultInvestigator returns the in-process Go rules engine.
func DefaultInvestigator(store ports.InvestigationRepository) IncidentInvestigator {
	return investigator.NewEngine(store)
}

// DefaultRemediation returns the in-process Go remediation engine.
func DefaultRemediation(store ports.RemediationRepository) RemediationGenerator {
	return remediation.NewEngine(store)
}

type Orchestrator struct {
	clusterID   string
	store       ports.AtlasStore
	client      kubernetes.Interface
	hub         *realtime.Hub
	collector   *collector.Collector
	health      *health.Engine
	incidents   *incidents.Engine
	context     *ctxbuilder.Builder
	investigator IncidentInvestigator
	remediation RemediationGenerator
	executor    *execution.Executor
}

func New(clusterID string, store ports.AtlasStore, client kubernetes.Interface, hub *realtime.Hub, inv IncidentInvestigator, rem RemediationGenerator) *Orchestrator {
	if inv == nil {
		inv = DefaultInvestigator(store)
	}
	if rem == nil {
		rem = DefaultRemediation(store)
	}
	o := &Orchestrator{
		clusterID:    clusterID,
		store:        store,
		client:       client,
		hub:          hub,
		context:      ctxbuilder.NewBuilder(client, store),
		investigator: inv,
		remediation:  rem,
	}
	o.collector = collector.New(clusterID, client, store, o)
	o.health = health.NewEngine(clusterID, store, store, o)
	o.incidents = incidents.NewEngine(clusterID, store, o)
	o.executor = execution.NewExecutor(clusterID, client, store, store, store, o)
	return o
}

func (o *Orchestrator) Publish(eventType string, payload any) {
	if o.hub != nil {
		o.hub.Publish(eventType, payload)
	}
	if eventType == "resource.updated" {
		o.handleResourceEvent(payload)
	}
}

func (o *Orchestrator) handleResourceEvent(payload any) {
	m, ok := payload.(map[string]string)
	if !ok || m["id"] == "" {
		return
	}
	ctx := context.Background()
	res, err := o.store.GetResource(ctx, m["id"])
	if err != nil {
		return
	}
	_ = o.health.EvaluateResource(ctx, res, o.onHealthTransition)
}

func (o *Orchestrator) Run(ctx context.Context) error {
	go o.reconcileLoop(ctx)
	return o.collector.Start(ctx)
}

func (o *Orchestrator) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.reconcileAll(ctx)
		}
	}
}

func (o *Orchestrator) reconcileAll(ctx context.Context) {
	resources, err := o.store.ListResources(ctx, o.clusterID, "", "", 2000)
	if err != nil {
		log.Printf("pipeline list resources: %v", err)
		return
	}
	for i := range resources {
		r := &resources[i]
		if r.DeletedAt != nil {
			continue
		}
		_ = o.health.EvaluateResource(ctx, r, o.onHealthTransition)
	}
	o.processOpenIncidents(ctx)
}

func (o *Orchestrator) onHealthTransition(ctx context.Context, t health.Transition) error {
	if err := o.incidents.HandleTransition(ctx, t); err != nil {
		return err
	}
	if t.Current != domain.HealthHealthy && (t.Previous == domain.HealthHealthy || t.Previous == domain.HealthWarning) {
		open, err := o.store.GetOpenAtlasIncidentForResource(ctx, t.Resource.ID)
		if err == nil && open != nil {
			go o.runIncidentPipeline(context.Background(), open, t.Resource)
		}
	}
	return nil
}

func (o *Orchestrator) processOpenIncidents(ctx context.Context) {
	list, err := o.store.ListAtlasIncidents(ctx, o.clusterID, "open", 50)
	if err != nil {
		return
	}
	for i := range list {
		inc := &list[i]
		res, err := o.store.GetResource(ctx, inc.ResourceID)
		if err != nil {
			continue
		}
		if _, err := o.store.GetInvestigation(ctx, inc.ID); err != nil {
			go o.runIncidentPipeline(context.Background(), inc, res)
		}
	}
}

func (o *Orchestrator) runIncidentPipeline(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource) {
	_ = o.store.UpdateAtlasIncidentStatus(ctx, inc.ID, domain.IncidentInvestigating)
	ic, err := o.context.Collect(ctx, inc, resource)
	if err != nil {
		log.Printf("context collect %s: %v", inc.ID, err)
		return
	}
	inv, err := o.investigator.Investigate(ctx, inc, resource, ic)
	if err != nil {
		log.Printf("investigate %s: %v", inc.ID, err)
		return
	}
	recs, err := o.remediation.Generate(ctx, inc, resource, inv)
	if err != nil {
		log.Printf("remediation %s: %v", inc.ID, err)
		return
	}
	if len(recs) > 0 {
		_ = o.store.UpdateAtlasIncidentStatus(ctx, inc.ID, domain.IncidentAwaitingApproval)
		if o.hub != nil {
			o.hub.Publish("incident.pipeline_complete", map[string]any{"incident_id": inc.ID})
		}
	}
}

func (o *Orchestrator) Executor() *execution.Executor {
	return o.executor
}
