import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  ChangeEvent,
  getChanges,
  getGraph,
  getIncidents,
  getStatus,
  GraphResponse,
  Incident,
  seedDemo,
  StatusResponse,
} from "../api/client";

const POLL_MS = 12_000;
const USER_EMAIL_KEY = "kube-dashboard-user-email";

export function useDashboard() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [graph, setGraph] = useState<GraphResponse>({ nodes: [], edges: [] });
  const [changes, setChanges] = useState<ChangeEvent[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [seeding, setSeeding] = useState(false);
  const [userEmail, setUserEmail] = useState<string>(() => {
    return localStorage.getItem(USER_EMAIL_KEY) ?? "";
  });

  useEffect(() => {
    if (userEmail) {
      localStorage.setItem(USER_EMAIL_KEY, userEmail);
    } else {
      localStorage.removeItem(USER_EMAIL_KEY);
    }
  }, [userEmail]);

  const refresh = useCallback(async () => {
    try {
      const s = await getStatus();
      setStatus(s);
      setConnected(true);
      setError(null);

      try {
        const [g, c, i] = await Promise.all([
          getGraph(),
          getChanges("24h", 20),
          getIncidents(),
        ]);
        setGraph(g);
        setChanges(c.changes ?? []);
        setIncidents(i.incidents ?? []);
      } catch {
        setGraph({ nodes: [], edges: [] });
        setChanges([]);
        setIncidents([]);
      }
    } catch (e) {
      setConnected(false);
      setStatus(null);
      if (e instanceof ApiError) {
        setError(e.message);
      } else {
        setError("Connection failed");
      }
    } finally {
      setLoading(false);
    }
  }, [userEmail]);

  const loadDemo = useCallback(async () => {
    setSeeding(true);
    try {
      await seedDemo();
      await refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Seed failed");
    } finally {
      setSeeding(false);
    }
  }, [refresh]);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, POLL_MS);
    return () => clearInterval(id);
  }, [refresh]);

  const isEmpty =
    connected &&
    status &&
    status.stats.graph_nodes === 0 &&
    status.stats.changes_24h === 0;

  return {
    status,
    graph,
    changes,
    incidents,
    connected,
    loading,
    error,
    seeding,
    isEmpty,
    refresh,
    loadDemo,
    userEmail,
    setUserEmail,
  };
}
