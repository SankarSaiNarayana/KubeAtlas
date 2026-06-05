import { fetchJSON } from "./client";
import type {
  AIInvestigation,
  AtlasIncident,
  AtlasOverview,
  ClusterResource,
  ExecutionRecord,
  RemediationRecommendation,
} from "../types/atlas";

export function getAtlasOverview() {
  return fetchJSON<AtlasOverview>("/api/v1/atlas/overview");
}

export function getAtlasResources(kind?: string, namespace?: string) {
  const params = new URLSearchParams();
  if (kind) params.set("kind", kind);
  if (namespace) params.set("namespace", namespace);
  const q = params.toString() ? `?${params}` : "";
  return fetchJSON<{ resources: ClusterResource[] }>(`/api/v1/atlas/resources${q}`);
}

export function getAtlasIncidents(status = "open") {
  return fetchJSON<{ incidents: AtlasIncident[] }>(
    `/api/v1/atlas/incidents?status=${encodeURIComponent(status)}`,
  );
}

export function getAtlasInvestigations() {
  return fetchJSON<{
    investigations: { incident: AtlasIncident; investigation?: AIInvestigation }[];
  }>("/api/v1/atlas/investigations");
}

export function getAtlasRemediations(status = "pending") {
  return fetchJSON<{ remediations: RemediationRecommendation[] }>(
    `/api/v1/atlas/remediations?status=${encodeURIComponent(status)}`,
  );
}

export function getAtlasExecutions() {
  return fetchJSON<{ executions: ExecutionRecord[] }>("/api/v1/atlas/executions");
}

export function getIncidentRemediations(incidentId: string) {
  return fetchJSON<{ remediations: RemediationRecommendation[] }>(
    `/api/v1/atlas/incidents/${incidentId}/remediations`,
  );
}

export function approveRemediation(id: string) {
  return fetchJSON<{ status: string }>(`/api/v1/atlas/remediations/${id}/approve`, {
    method: "POST",
  });
}

export function rejectRemediation(id: string) {
  return fetchJSON<{ status: string }>(`/api/v1/atlas/remediations/${id}/reject`, {
    method: "POST",
  });
}

export function executeRemediation(id: string) {
  return fetchJSON<ExecutionRecord>(`/api/v1/atlas/remediations/${id}/execute`, {
    method: "POST",
  });
}

export function connectEventStream(onEvent: () => void): () => void {
  const base = import.meta.env.VITE_API_URL ?? "";
  const es = new EventSource(`${base}/api/v1/events/stream`);
  es.onmessage = () => onEvent();
  es.onerror = () => {
    /* browser reconnects */
  };
  return () => es.close();
}
