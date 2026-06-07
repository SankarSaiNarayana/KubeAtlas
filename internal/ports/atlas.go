package ports

import (
	"context"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
)

// ResourceRepository defines the port for storing and retrieving cluster resources.
// Implementations persist resource metadata, mark deleted resources, and offer
// list and lookup operations used by the pipeline and API.
type ResourceRepository interface {
	UpsertResource(ctx context.Context, r *domain.ClusterResource) error
	MarkResourceDeleted(ctx context.Context, clusterID, resourceUID string) error
	GetResourceByUID(ctx context.Context, clusterID, resourceUID string) (*domain.ClusterResource, error)
	GetResource(ctx context.Context, id string) (*domain.ClusterResource, error)
	ListResources(ctx context.Context, clusterID string, kind, namespace string, limit int) ([]domain.ClusterResource, error)
}

// HealthRepository defines the port for resource health persistence.
// It tracks the latest health state for each resource and returns the
// prior state so incident handling can compare transitions.
type HealthRepository interface {
	UpsertHealth(ctx context.Context, h *domain.ResourceHealth) (previous domain.HealthState, err error)
	GetHealth(ctx context.Context, resourceID string) (*domain.ResourceHealth, error)
}

// IncidentRepository defines the port for incident lifecycle operations.
// The pipeline uses it to create, resolve, and update incident state, while
// the API uses it to retrieve incidents for the dashboard.
type IncidentRepository interface {
	CreateAtlasIncident(ctx context.Context, inc *domain.AtlasIncident) error
	GetOpenAtlasIncidentForResource(ctx context.Context, resourceID string) (*domain.AtlasIncident, error)
	ResolveAtlasIncident(ctx context.Context, incidentID string) error
	UpdateAtlasIncidentStatus(ctx context.Context, incidentID string, status domain.IncidentStatus) error
	UpdateAtlasIncidentReason(ctx context.Context, incidentID, reason string, health domain.HealthState) error
	GetAtlasIncident(ctx context.Context, id string) (*domain.AtlasIncident, error)
	ListAtlasIncidents(ctx context.Context, clusterID string, status string, limit int) ([]domain.AtlasIncident, error)
}

// ContextRepository defines the port for saving and retrieving investigation context.
// The pipeline stores event/log context for each incident so the AI and UI can
// inspect the incident details later.
type ContextRepository interface {
	SaveContext(ctx context.Context, c *domain.IncidentContext) error
	GetContext(ctx context.Context, incidentID string) (*domain.IncidentContext, error)
}

// InvestigationRepository defines the port for persisting AI investigation results.
// The AI service or rules engine writes summaries, root cause, and confidence
// data so the API can return it to the frontend.
type InvestigationRepository interface {
	SaveInvestigation(ctx context.Context, inv *domain.AIInvestigation) error
	GetInvestigation(ctx context.Context, incidentID string) (*domain.AIInvestigation, error)
}

// RemediationRepository defines the port for storing remediation recommendations.
// Recommendations can be listed by incident or status, and their workflow status
// is updated as they are approved, rejected, or executed.
type RemediationRepository interface {
	SaveRecommendation(ctx context.Context, rec *domain.RemediationRecommendation) error
	GetRecommendation(ctx context.Context, id string) (*domain.RemediationRecommendation, error)
	ListRecommendations(ctx context.Context, incidentID string) ([]domain.RemediationRecommendation, error)
	ClearRecommendationsForIncident(ctx context.Context, incidentID string) error
	ListByStatus(ctx context.Context, clusterID string, status domain.ActionStatus, limit int) ([]domain.RemediationRecommendation, error)
	UpdateRecommendationStatus(ctx context.Context, id string, status domain.ActionStatus) error
}

// ApprovalRepository defines the port for approval workflow tracking.
// It records pending approval requests, approves recommendations, and checks
// whether a remediation is authorized to execute.
type ApprovalRepository interface {
	CreatePending(ctx context.Context, recommendationID, requestedBy string) error
	Approve(ctx context.Context, recommendationID, approvedBy string) error
	IsApproved(ctx context.Context, recommendationID string) (bool, string, error)
}

// ExecutionRepository defines the port for recording remediation or investigation execution history.
// It persists execution results and allows the API to list workflow runs for the dashboard.
type ExecutionRepository interface {
	SaveExecution(ctx context.Context, rec *domain.ExecutionRecord) error
	ListExecutions(ctx context.Context, clusterID string, limit int) ([]domain.ExecutionRecord, error)
}

// OverviewRepository defines the port for dashboard summary statistics.
// It returns cluster overview data used by the frontend home page.
type OverviewRepository interface {
	GetOverview(ctx context.Context, clusterID string) (*domain.OverviewStats, error)
}

// AtlasStore is the aggregate port that groups all atlas persistence interfaces.
// Concrete store implementations satisfy this interface when they provide all
// resource, health, incident, context, investigation, remediation, approval,
// execution, and overview persistence capabilities.
type AtlasStore interface {
	ResourceRepository
	HealthRepository
	IncidentRepository
	ContextRepository
	InvestigationRepository
	RemediationRepository
	ApprovalRepository
	ExecutionRepository
	OverviewRepository
}
