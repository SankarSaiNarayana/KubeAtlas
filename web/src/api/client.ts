export interface GraphNode {
  id: string;
  cluster_id: string;
  kind: string;
  namespace: string;
  name: string;
  status: string;
  labels: Record<string, string>;
  updated_at: string;
}

export interface GraphEdge {
  id: string;
  source_id: string;
  target_id: string;
  edge_type: string;
}

export interface GraphResponse {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface ChangeEvent {
  id: string;
  cluster_id: string;
  resource: {
    kind: string;
    namespace: string;
    name: string;
  };
  verb: string;
  actor: string;
  source: string;
  diff_summary: string;
  occurred_at: string;
}

export interface Incident {
  id: string;
  title: string;
  status: string;
  resource_kind?: string;
  resource_namespace?: string;
  resource_name?: string;
  started_at: string;
}

export interface DashboardStats {
  graph_nodes: number;
  graph_edges: number;
  changes_24h: number;
  open_incidents: number;
}

export interface StatusResponse {
  connected: boolean;
  cluster_id: string;
  database: string;
  stats: DashboardStats;
}

const API_BASE = import.meta.env.VITE_API_URL ?? "";
const USER_EMAIL_KEY = "kube-dashboard-user-email";

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}

function getUserEmail() {
  return localStorage.getItem(USER_EMAIL_KEY) ?? "";
}

export function saveUserEmail(email: string) {
  const trimmed = email.trim();
  if (trimmed) {
    localStorage.setItem(USER_EMAIL_KEY, trimmed);
  } else {
    localStorage.removeItem(USER_EMAIL_KEY);
  }
}

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${API_BASE}${path}`;
  const email = getUserEmail();
  const headers = new Headers(init?.headers as HeadersInit);
  headers.set("Content-Type", "application/json");
  if (email) {
    headers.set("X-User-Email", email);
  }
  try {
    const res = await fetch(url, {
      ...init,
      headers,
    });
    if (!res.ok) {
      const body = await res.text();
      throw new ApiError(body || `HTTP ${res.status}`, res.status);
    }
    return res.json() as Promise<T>;
  } catch (e) {
    if (e instanceof ApiError) throw e;
    throw new ApiError(
      "Cannot reach API — run: go run ./cmd/api (port 8080)",
      0,
    );
  }
}

export function getHealth() {
  return fetchJSON<{ status: string; cluster_id: string; service: string }>(
    "/health",
  );
}

export function getStatus() {
  return fetchJSON<StatusResponse>("/api/v1/status");
}

export function getGraph(namespace?: string) {
  const q = namespace ? `?namespace=${encodeURIComponent(namespace)}` : "";
  return fetchJSON<GraphResponse>(`/api/v1/graph${q}`).then((g) => ({
    nodes: g.nodes ?? [],
    edges: g.edges ?? [],
  }));
}

export function getChanges(since = "24h", limit = 50) {
  return fetchJSON<{ changes: ChangeEvent[] }>(
    `/api/v1/changes?since=${since}&limit=${limit}`,
  );
}

export function getIncidents() {
  return fetchJSON<{ incidents: Incident[] }>("/api/v1/incidents");
}

export function seedDemo() {
  return fetchJSON<{ message: string; stats: DashboardStats }>(
    "/api/v1/demo/seed",
    { method: "POST" },
  );
}
