package domain

import (
	"encoding/json"
	"time"
)

type HealthState string

const (
	HealthHealthy  HealthState = "HEALTHY"
	HealthWarning  HealthState = "WARNING"
	HealthCritical HealthState = "CRITICAL"
)

type IncidentSeverity string

const (
	SeverityWarning  IncidentSeverity = "warning"
	SeverityCritical IncidentSeverity = "critical"
)

type IncidentStatus string

const (
	IncidentOpen              IncidentStatus = "open"
	IncidentInvestigating       IncidentStatus = "investigating"
	IncidentAwaitingApproval    IncidentStatus = "awaiting_approval"
	IncidentResolved            IncidentStatus = "resolved"
)

type ActionStatus string

const (
	ActionPending    ActionStatus = "pending"
	ActionApproved   ActionStatus = "approved"
	ActionRejected   ActionStatus = "rejected"
	ActionExecuting  ActionStatus = "executing"
	ActionSucceeded  ActionStatus = "succeeded"
	ActionFailed     ActionStatus = "failed"
	ActionRolledBack ActionStatus = "rolled_back"
)

type ClusterResource struct {
	ID             string          `json:"id"`
	ClusterID      string          `json:"cluster_id"`
	ResourceUID    string          `json:"resource_uid"`
	APIVersion     string          `json:"api_version"`
	Kind           string          `json:"kind"`
	Namespace      string          `json:"namespace"`
	Name           string          `json:"name"`
	Labels         json.RawMessage `json:"labels"`
	SpecSnapshot   json.RawMessage `json:"spec_snapshot"`
	StatusSnapshot json.RawMessage `json:"status_snapshot"`
	NodeName       string          `json:"node_name,omitempty"`
	OwnerKind      string          `json:"owner_kind,omitempty"`
	OwnerName      string          `json:"owner_name,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      *time.Time      `json:"deleted_at,omitempty"`
	Health         *ResourceHealth `json:"health,omitempty"`
}

type ResourceHealth struct {
	ID          string          `json:"id"`
	ClusterID   string          `json:"cluster_id"`
	ResourceID  string          `json:"resource_id"`
	Health      HealthState     `json:"health"`
	Reason      string          `json:"reason"`
	Details     json.RawMessage `json:"details"`
	EvaluatedAt time.Time       `json:"evaluated_at"`
}

type AtlasIncident struct {
	ID           string           `json:"id"`
	ClusterID    string           `json:"cluster_id"`
	ResourceID   string           `json:"resource_id"`
	Resource     *ClusterResource `json:"resource,omitempty"`
	Title        string           `json:"title"`
	Severity     IncidentSeverity `json:"severity"`
	Status       IncidentStatus   `json:"status"`
	Reason       string           `json:"reason"`
	HealthBefore *HealthState     `json:"health_before,omitempty"`
	HealthAfter  HealthState      `json:"health_after"`
	OpenedAt     time.Time        `json:"opened_at"`
	ResolvedAt   *time.Time       `json:"resolved_at,omitempty"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type IncidentContext struct {
	ID              string          `json:"id"`
	IncidentID      string          `json:"incident_id"`
	Logs            json.RawMessage `json:"logs"`
	Events          json.RawMessage `json:"events"`
	DescribeData    json.RawMessage `json:"describe_data"`
	DeploymentYAML  string          `json:"deployment_yaml,omitempty"`
	ReplicaSetInfo  json.RawMessage `json:"replicaset_info"`
	NodeInfo        json.RawMessage `json:"node_info"`
	RestartCount    int             `json:"restart_count"`
	ImageDetails    json.RawMessage `json:"image_details"`
	EnvVars         json.RawMessage `json:"env_vars"`
	VolumeMounts    json.RawMessage `json:"volume_mounts"`
	CollectedAt     time.Time       `json:"collected_at"`
}

type AIInvestigation struct {
	ID               string          `json:"id"`
	IncidentID       string          `json:"incident_id"`
	Summary          string          `json:"summary"`
	RootCause        string          `json:"root_cause"`
	ConfidenceScore  float64         `json:"confidence_score"`
	ImpactAssessment string          `json:"impact_assessment"`
	Evidence         json.RawMessage `json:"evidence"`
	RecommendedFix   string          `json:"recommended_fix"`
	ModelVersion     string          `json:"model_version"`
	InvestigatedAt   time.Time       `json:"investigated_at"`
}

type RemediationRecommendation struct {
	ID               string          `json:"id"`
	IncidentID       string          `json:"incident_id"`
	InvestigationID  string          `json:"investigation_id,omitempty"`
	ActionType       string          `json:"action_type"`
	Reason           string          `json:"reason"`
	ConfidenceScore  float64         `json:"confidence_score"`
	RiskScore        float64         `json:"risk_score"`
	ExpectedOutcome  string          `json:"expected_outcome"`
	Parameters       json.RawMessage `json:"parameters"`
	Status           ActionStatus    `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
}

const (
	ActionRestartPod         = "restart_pod"
	ActionRestartDeployment  = "restart_deployment"
	ActionRollbackDeployment = "rollback_deployment"
	ActionScaleDeployment    = "scale_deployment"
	ActionDeleteFailedPod    = "delete_failed_pod"
)

type OverviewStats struct {
	TotalResources    int `json:"total_resources"`
	HealthyResources  int `json:"healthy_resources"`
	WarningResources  int `json:"warning_resources"`
	CriticalResources int `json:"critical_resources"`
	OpenIncidents     int `json:"open_incidents"`
	ResolvedIncidents int `json:"resolved_incidents"`
}

type ExecutionRecord struct {
	ID               string          `json:"id"`
	ClusterID        string          `json:"cluster_id"`
	RecommendationID string          `json:"recommendation_id"`
	ApprovedBy       string          `json:"approved_by"`
	ActionType       string          `json:"action_type"`
	Parameters       json.RawMessage `json:"parameters"`
	Success          bool            `json:"success"`
	FailureReason    string          `json:"failure_reason,omitempty"`
	RolledBack       bool            `json:"rolled_back"`
	RollbackReason   string          `json:"rollback_reason,omitempty"`
	StartedAt        time.Time       `json:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	DurationMs       int64           `json:"duration_ms,omitempty"`
}
