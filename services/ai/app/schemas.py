from typing import Any

from pydantic import BaseModel, Field


class ResourcePayload(BaseModel):
    id: str = ""
    cluster_id: str = ""
    kind: str = ""
    namespace: str = ""
    name: str = ""
    resource_uid: str = ""
    node_name: str = ""
    owner_kind: str = ""
    owner_name: str = ""


class IncidentPayload(BaseModel):
    id: str
    cluster_id: str = ""
    resource_id: str = ""
    title: str = ""
    severity: str = "warning"
    status: str = "open"
    reason: str = ""
    health_after: str = "CRITICAL"


class IncidentContextPayload(BaseModel):
    incident_id: str = ""
    logs: list[Any] = Field(default_factory=list)
    events: list[Any] = Field(default_factory=list)
    describe_data: dict[str, Any] = Field(default_factory=dict)
    deployment_yaml: str = ""
    replicaset_info: dict[str, Any] = Field(default_factory=dict)
    node_info: dict[str, Any] = Field(default_factory=dict)
    restart_count: int = 0
    image_details: list[Any] = Field(default_factory=list)
    env_vars: list[Any] = Field(default_factory=list)
    volume_mounts: list[Any] = Field(default_factory=list)


class InvestigateRequest(BaseModel):
    incident: IncidentPayload
    resource: ResourcePayload
    context: IncidentContextPayload


class InvestigationResponse(BaseModel):
    summary: str
    root_cause: str
    confidence_score: float = Field(ge=0, le=1)
    impact_assessment: str
    evidence: list[dict[str, str]] = Field(default_factory=list)
    recommended_fix: str
    model_version: str = "kubeatlas-groq-v1"


class InvestigationStored(InvestigationResponse):
    incident_id: str


class AIInvestigationRef(BaseModel):
    id: str = ""
    incident_id: str = ""
    summary: str = ""
    root_cause: str = ""
    confidence_score: float = 0.5
    impact_assessment: str = ""
    evidence: list[Any] = Field(default_factory=list)
    recommended_fix: str = ""
    model_version: str = ""


class RemediateRequest(BaseModel):
    incident: IncidentPayload
    resource: ResourcePayload
    investigation: AIInvestigationRef


class RemediationItem(BaseModel):
    action_type: str
    reason: str
    confidence_score: float = Field(ge=0, le=1)
    risk_score: float = Field(ge=0, le=1)
    expected_outcome: str
    parameters: dict[str, str] = Field(default_factory=dict)


class RemediateResponse(BaseModel):
    recommendations: list[RemediationItem] = Field(default_factory=list)


class HealthResponse(BaseModel):
    status: str = "ok"
    llm_provider: str = "groq"
    langchain_available: bool = True
