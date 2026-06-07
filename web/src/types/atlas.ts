export type HealthState = "HEALTHY" | "WARNING" | "CRITICAL";

export interface ResourceHealth {
  id: string;
  health: HealthState;
  reason: string;
  evaluated_at: string;
}

export interface ClusterResource {
  id: string;
  cluster_id: string;
  kind: string;
  namespace: string;
  name: string;
  health?: ResourceHealth;
  updated_at: string;
}

export interface AtlasOverview {
  total_resources: number;
  healthy_resources: number;
  warning_resources: number;
  critical_resources: number;
  open_incidents: number;
  resolved_incidents: number;
}

export interface AtlasIncident {
  id: string;
  cluster_id: string;
  resource_id: string;
  title: string;
  severity: "warning" | "critical";
  status: string;
  reason: string;
  health_after: HealthState;
  opened_at: string;
  resolved_at?: string;
}

export interface AIInvestigation {
  id: string;
  incident_id: string;
  summary: string;
  root_cause: string;
  confidence_score: number;
  impact_assessment: string;
  evidence: unknown[];
  recommended_fix: string;
  model_version: string;
  investigated_at: string;
}

export interface RemediationRecommendation {
  id: string;
  incident_id: string;
  action_type: string;
  reason: string;
  confidence_score: number;
  risk_score: number;
  expected_outcome: string;
  parameters: Record<string, string>;
  status: string;
  created_at: string;
}

export interface IncidentWorkflow {
  incident: AtlasIncident;
  investigation?: AIInvestigation;
  remediations: RemediationRecommendation[];
  resource_health?: ResourceHealth;
}

export interface ExecutionRecord {
  id: string;
  recommendation_id: string;
  action_type: string;
  success: boolean;
  failure_reason?: string;
  approved_by: string;
  duration_ms?: number;
  started_at: string;
}
