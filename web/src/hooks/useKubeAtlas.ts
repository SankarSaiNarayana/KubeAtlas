import { useCallback, useEffect, useState } from "react";
import {
  connectEventStream,
  getAtlasExecutions,
  getAtlasIncidents,
  getAtlasInvestigations,
  getAtlasOverview,
  getAtlasRemediations,
  getAtlasResources,
} from "../api/atlas";
import { ApiError, getHealth } from "../api/client";
import type {
  AIInvestigation,
  AtlasIncident,
  AtlasOverview,
  ClusterResource,
  ExecutionRecord,
  RemediationRecommendation,
} from "../types/atlas";

// useKubeAtlas hooks into the backend atlas API and returns all dashboard data.
// It loads overview statistics, cluster resources, open incidents, investigation
// rows, remediation recommendations, and execution history.
export function useKubeAtlas() {
  const [overview, setOverview] = useState<AtlasOverview | null>(null);
  const [resources, setResources] = useState<ClusterResource[]>([]);
  const [atlasIncidents, setAtlasIncidents] = useState<AtlasIncident[]>([]);
  const [investigations, setInvestigations] = useState<
    { incident: AtlasIncident; investigation?: AIInvestigation }[]
  >([]);
  const [remediations, setRemediations] = useState<RemediationRecommendation[]>([]);
  const [executions, setExecutions] = useState<ExecutionRecord[]>([]);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [clusterId, setClusterId] = useState<string>("");

  // refresh fetches all atlas-related data in parallel and updates hook state.
  // It also checks backend health first so the UI can display connection state.
  const refresh = useCallback(async () => {
    try {
      const health = await getHealth();
      setClusterId(health.cluster_id);
      const [o, r, inc, inv, remPending, remApproved, ex] = await Promise.all([
        getAtlasOverview(),
        getAtlasResources(),
        getAtlasIncidents("open"),
        getAtlasInvestigations(),
        getAtlasRemediations("pending"),
        getAtlasRemediations("approved"),
        getAtlasExecutions(),
      ]);
      setOverview(o);
      setResources(r.resources ?? []);
      setAtlasIncidents(inc.incidents ?? []);
      setInvestigations(inv.investigations ?? []);
      setRemediations([
        ...(remPending.remediations ?? []),
        ...(remApproved.remediations ?? []),
      ]);
      setExecutions(ex.executions ?? []);
      setConnected(true);
      setError(null);
    } catch (e) {
      setConnected(false);
      setError(e instanceof ApiError ? e.message : "Connection failed");
    } finally {
      setLoading(false);
    }
  }, []);

  // When the hook mounts, load data and subscribe to backend event stream.
  // Event stream notifications trigger refresh so the UI stays in sync with new incidents,
  // remediation state, or updated resource health.
  useEffect(() => {
    refresh();
    const disconnect = connectEventStream(() => {
      refresh();
    });
    return disconnect;
  }, [refresh]);

  return {
    overview,
    resources,
    atlasIncidents,
    investigations,
    remediations,
    executions,
    connected,
    loading,
    error,
    clusterId,
    refresh,
  };
}
