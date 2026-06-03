# 📐 Kube Dashboard Architecture

## System Overview

Kube Dashboard is a **real-time SRE (Site Reliability Engineering) dashboard** that tracks Kubernetes resources, dependencies, changes, and incidents in one place.

```
┌─────────────────────────────────────────────────────────────────┐
│                    BROWSER (React Frontend)                      │
│                   http://localhost:5176                          │
│  ┌──────────────┬─────────────┬──────────────┬──────────────┐   │
│  │ Home Page    │ Graph Page  │ Changes Page │ Incidents    │   │
│  │ (Stats)      │ (Topology)  │ (Timeline)   │ (Alerts)     │   │
│  └──────────────┴─────────────┴──────────────┴──────────────┘   │
└────────────────────┬──────────────────────────────────────────────┘
                     │ HTTP Polling (every 12s)
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│              API SERVER (Go + net/http)                          │
│              http://localhost:8080                               │
│  ┌──────────────┬──────────────┬──────────────┬──────────────┐  │
│  │ GET /graph   │ GET /changes │ GET /status  │ POST /       │  │
│  │ (topology)   │ (timeline)   │ (stats)      │ webhooks/*   │  │
│  │              │              │              │ (Robusta,    │  │
│  │              │              │              │  GitOps)     │  │
│  └──────────────┴──────────────┴──────────────┴──────────────┘  │
│                        ▲                                          │
│        SQL Queries & Inserts via Store Layer                     │
└────────────────────────┼──────────────────────────────────────────┘
                         │
        ┌────────────────┴────────────────┬─────────────────┐
        ▼                                  ▼                 ▼
┌──────────────────┐           ┌─────────────────┐   ┌────────────┐
│ GRAPH BUILDER    │           │  POSTGRES DB    │   │   REDIS    │
│ (Go Watcher)     │           │  (Port 5433)    │   │ (Port 6379)│
│ Watches K8s ──┐  │           │                 │   │            │
│ Updates Graph │──►   INSERT/UPDATE graph_nodes │   │ (Future:   │
│               │     graph_edges change_events  │   │  caching)  │
│               │     incidents                  │   │            │
│               │                                 │   │            │
└──────────────────┘           └─────────────────┘   └────────────┘
        │
        │ Watches via K8s Informers
        ▼
┌──────────────────────────────────────────────────────────────────┐
│          KUBERNETES CLUSTER (Kind - Local)                       │
│          https://127.0.0.1:50689                                 │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ demo namespace:                                          │   │
│  │  • nginx Deployment (2 replicas)                         │   │
│  │  • api Deployment (1 replica)                            │   │
│  │  • Services (nginx-svc, api-svc)                         │   │
│  │  • ConfigMaps (app-config)                               │   │
│  │  • Pods (auto-created by controllers)                    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ monitoring namespace:                                    │   │
│  │  • Prometheus (metrics collection)                       │   │
│  │  • Alertmanager (alert routing)                          │   │
│  │  • Grafana (visualization)                               │   │
│  │  • kube-state-metrics (K8s metrics)                      │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ kube-system namespace:                                   │   │
│  │  • CoreDNS (service discovery)                           │   │
│  │  • kube-proxy (networking)                               │   │
│  │  • kubelet (container runtime interface)                 │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

---

## Core Components

### 1. **Frontend** (`web/`)
- **Type**: React 18 + TypeScript + Vite
- **Purpose**: Real-time SRE dashboard UI
- **Location**: http://localhost:5176
- **Polling**: Calls API every 12 seconds
- **Pages**:
  - Home: Stats overview + recent changes + open incidents
  - Graph: Interactive topology visualization
  - Changes: Timeline of who changed what
  - Incidents: Alert-based issues

### 2. **API Server** (`cmd/api/main.go`)
- **Type**: Go HTTP server (net/http)
- **Purpose**: REST API that serves dashboard data and accepts webhooks
- **Location**: http://localhost:8080
- **Endpoints**:
  - `GET /health` → System health check
  - `GET /api/v1/status` → Dashboard stats (nodes, edges, changes, incidents)
  - `GET /api/v1/graph` → Full K8s resource graph with dependencies
  - `GET /api/v1/changes` → Change history with filtering
  - `GET /api/v1/incidents` → List of incidents (alerts)
  - `POST /api/v1/webhooks/robusta` → Accept Robusta scaling/update webhooks
  - `POST /api/v1/webhooks/gitops` → Accept Argo CD / Flux sync events
  - `POST /api/v1/incidents` → Create incidents from Alertmanager webhooks
- **Database**: Connects to PostgreSQL via Store layer

### 3. **Graph Builder** (`cmd/graph/main.go`)
- **Type**: Go daemon with K8s client-go informers
- **Purpose**: Watch Kubernetes cluster and populate graph database
- **Watches**:
  - **Deployments** → Track replicas, status, volumes
  - **Services** → Track label selectors, endpoint bindings
  - **Ingresses** → Track service routing
  - **Pods** → Track individual pod status (ready/pending/failed)
  - **ConfigMaps & Secrets** → Track mounts/references
- **Output**: Inserts/updates `graph_nodes` and `graph_edges` in PostgreSQL
- **Edge Types**:
  - `mounts`: Deployment mounts a ConfigMap/Secret
  - `selects`: Service selects Deployment via label matcher
  - `exposes`: Ingress exposes a Service
  - `owned_by`: Pod owned by Deployment

### 4. **Store Layer** (`internal/store/postgres.go`)
- **Type**: Go database abstraction over PostgreSQL
- **Purpose**: Persists K8s graph and change events
- **Tables**:
  - `graph_nodes`: K8s resources (Deployments, Pods, Services, etc.)
  - `graph_edges`: Relationships between resources
  - `change_events`: Who changed what (actor, verb, diff, timestamp)
  - `incidents`: Alertmanager alerts mapped to resources
- **Key Methods**:
  - `UpsertGraphNode()` → Insert/update a resource
  - `UpsertGraphEdge()` → Insert/update a relationship
  - `InsertChangeEvent()` → Record a change
  - `CreateIncident()` → Record an alert/issue
  - `ListChanges()` → Query change history
  - `GetGraph()` → Get all resources + relationships

### 5. **Ingest Service** (`cmd/ingest/main.go`)
- **Type**: Go daemon for change capture
- **Purpose**: Currently unused (placeholder for audit log tailing)
- **Future**: Will tail Kubernetes audit logs and parse them into change events
- **Status**: Scaffolded but not actively used

### 6. **Configuration** (`internal/config/config.go`)
- **Type**: Go configuration loader
- **Purpose**: Loads environment variables and provides config to services
- **Key Variables**:
  - `DATABASE_URL`: PostgreSQL connection string
  - `CLUSTER_ID`: Cluster identifier ("local" for dev)
  - `KUBECONFIG`: Path to K8s config for client-go
  - `API_ADDR`: API server listen address (:8080)

### 7. **Models** (`internal/models/models.go`)
- **Type**: Go data structures
- **Purpose**: Shared types across services
- **Key Types**:
  - `GraphNode`: Represents a K8s resource
  - `GraphEdge`: Represents relationship between resources
  - `ChangeEvent`: Represents a change (who, what, when)
  - `Incident`: Represents an alert/issue

---

## Data Flow: Start to Finish

### **Scenario 1: SRE Opens Dashboard**

```
1. Browser (React)
   └─ Loads http://localhost:5176/
   └─ Calls useContext(DashboardContext) → useDashboard hook
      
2. Frontend (useDashboard.ts)
   └─ useEffect → Polls API every 12 seconds
   └─ Calls: GET /api/v1/status + GET /api/v1/graph + GET /api/v1/changes + GET /api/v1/incidents
      
3. API Server (handlers.go)
   └─ Receives GET requests
   └─ Queries PostgreSQL via Store layer
   └─ Returns JSON:
      {
        "stats": {"graph_nodes": 34, "graph_edges": 15, "changes_24h": 2, "open_incidents": 1},
        "nodes": [...], "edges": [...],
        "changes": [...],
        "incidents": [...]
      }
      
4. Frontend (React Components)
   └─ Renders StatCard components with numbers
   └─ Renders GraphCanvas with D3 visualization
   └─ Renders ChangeTimeline with sorted changes
   └─ Renders IncidentPage with alert cards
   └─ Auto-refreshes every 12 seconds
```

### **Scenario 2: Kubernetes Deployment Scales (2→5 replicas)**

```
1. SRE runs:
   $ kubectl scale deployment nginx --replicas=5 -n demo

2. Kubernetes API
   └─ Updates nginx Deployment spec.replicas to 5
   └─ Triggers deployment controller

3. Graph Builder (watching K8s)
   └─ Deployment informer detects UPDATE event
   └─ Calls onDeployment() handler
   └─ Extracts: name=nginx, namespace=demo, replicas=5, status=not_ready
   └─ Calls store.UpsertGraphNode() → PostgreSQL
   
4. Kubernetes Controllers
   └─ ReplicaSet controller creates 3 new Pods (2→5)
   └─ Kubelet starts containers
   
5. Graph Builder Pod Watcher
   └─ Detects 3 new Pod ADD events
   └─ Each pod: calls store.UpsertGraphNode() → PostgreSQL
   └─ Each pod: calls store.UpsertGraphEdge(pod→deployment, "owned_by")
   
6. SRE (via webhook/Robusta) sends:
   $ curl -X POST http://localhost:8080/api/v1/webhooks/robusta \
     -d '{"kind":"Deployment", "name":"nginx", "namespace":"demo", 
          "verb":"scale", "user":"sre@company.com", "diff":"2→5"}'

7. API Server (RobustaWebhook handler)
   └─ Parses JSON payload
   └─ Calls store.InsertChangeEvent()
   └─ Creates change_events row:
      {
        "kind": "Deployment", "name": "nginx", "namespace": "demo",
        "verb": "scale", "actor": "sre@company.com", "diff": "2 → 5",
        "source": "robusta", "occurred_at": "now"
      }
   └─ Returns HTTP 201 + ChangeEvent JSON

8. Frontend Polling (next 12s cycle)
   └─ Calls GET /api/v1/status
   └─ API returns: {"stats": {"graph_nodes": 39, "graph_edges": 20, "changes_24h": 1}}
   └─ React sees new numbers → updates StatCards
   └─ Calls GET /api/v1/changes
   └─ API returns new change with actor, verb, diff
   └─ React renders in ChangeTimeline
   
9. SRE Dashboard Now Shows:
   ✓ nginx pod count: 5/5 ready
   ✓ Recent change: "nginx scaled 2→5 by sre@company.com"
   ✓ Timestamp: when it happened
```

### **Scenario 3: Pod Crashes and Alert Fires**

```
1. Pod Crash
   └─ A pod's container exits with error code
   └─ Kubelet detects failure → marks pod as Failed/CrashLoopBackOff

2. Graph Builder Pod Watcher
   └─ Pod informer detects UPDATE event (status change)
   └─ Calls onPod() handler
   └─ Updates pod status in database: status="not_ready"

3. Prometheus + Alertmanager
   └─ kube-state-metrics exports pod status metrics
   └─ Prometheus scrapes every 15s
   └─ Alert rule fires: KubePodCrashLooping (severity=critical)
   └─ Alertmanager evaluates routing rules
   └─ Sends webhook to Kube Dashboard

4. API Server (CreateIncidentFromAlert handler)
   └─ Receives Alertmanager webhook:
      {
        "alerts": [{
          "status": "firing",
          "labels": {"alertname": "KubePodCrashLooping", "pod": "nginx-abc123", "namespace": "demo"}
        }]
      }
   └─ Calls store.CreateIncident()
   └─ Creates incidents row: {title, status="open", resource_pod, alert_labels}

5. Frontend Polling (next 12s cycle)
   └─ Calls GET /api/v1/status
   └─ API returns: {"stats": {"open_incidents": 1}}
   └─ React sees incident count increased
   └─ Calls GET /api/v1/incidents
   └─ Renders incident card on dashboard

6. SRE Dashboard Now Shows:
   ✓ Pod status: not_ready (red indicator)
   ✓ Open incident: "Pod nginx-abc123 is crash looping"
   ✓ Linked to resource on graph

7. SRE Fixes Issue (e.g., updates image)
   └─ kubectl set image deployment/nginx nginx=nginx:1.21
   
8. Pod Recovers
   └─ New pod starts with fixed image
   └─ Kubernetes marks pod as Running → Ready
   
9. Alertmanager
   └─ Alert resolves (no longer firing)
   └─ Sends webhook with status="resolved"
   
10. API Server
    └─ Receives resolved webhook
    └─ Updates incident: status="resolved"
    
11. Frontend Polling
    └─ GET /api/v1/incidents returns only open incidents
    └─ Resolved incident no longer shown
    └─ Dashboard updates
```

### **Scenario 4: GitOps Deployment (Argo CD Sync)**

```
1. SRE commits code to git repository

2. Argo CD detects change
   └─ Pulls new manifest
   └─ Compares with cluster state
   └─ Applies diff to K8s (deployment spec change)

3. Argo CD sends webhook:
   $ curl -X POST http://localhost:8080/api/v1/webhooks/gitops \
     -d '{"app_name":"my-app", "namespace":"demo", "synced_by":"devops@ci", "revision":"abc123"}'

4. API Server (GitOpsWebhook handler)
   └─ Parses payload
   └─ Calls store.InsertChangeEvent()
   └─ Creates change_events row:
      {
        "kind": "Application", "name": "my-app",
        "verb": "sync", "actor": "devops@ci", "source": "gitops"
      }

5. Kubernetes + Graph Builder
   └─ Deployment controller applies new spec
   └─ Replicas/image/env vars update
   └─ Graph builder detects UPDATE event
   └─ Updates graph_nodes with new status

6. Frontend Polling
   └─ Shows new change: "Application my-app synced by devops@ci"
   └─ Source: gitops (distinguishes from kubectl/Robusta)
   └─ SRE can trace who deployed what via which tool
```

---

## Database Schema

### `graph_nodes`
Represents any Kubernetes resource (Deployment, Pod, Service, ConfigMap, etc.)

```sql
CREATE TABLE graph_nodes (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  resource_uid VARCHAR(255),
  api_version VARCHAR(255),
  kind VARCHAR(100),         -- e.g., "Deployment", "Pod", "Service"
  namespace VARCHAR(255),
  name VARCHAR(255),
  labels JSONB,              -- e.g., {"app": "nginx", "version": "1.0"}
  status VARCHAR(100),       -- e.g., "ready", "not_ready", "pending"
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  
  UNIQUE(cluster_id, api_version, kind, namespace, name)
);
```

### `graph_edges`
Represents relationships (mounts, selects, exposes, owned_by)

```sql
CREATE TABLE graph_edges (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  source_id UUID REFERENCES graph_nodes(id),  -- From resource
  target_id UUID REFERENCES graph_nodes(id),  -- To resource
  edge_type VARCHAR(50),                      -- "mounts"|"selects"|"exposes"|"owned_by"
  metadata JSONB,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  
  UNIQUE(cluster_id, source_id, target_id, edge_type)
);
```

### `change_events`
Records who changed what and when

```sql
CREATE TABLE change_events (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  resource_uid VARCHAR(255),
  api_version VARCHAR(255),
  kind VARCHAR(100),
  namespace VARCHAR(255),
  name VARCHAR(255),
  verb VARCHAR(50),          -- e.g., "scale", "update", "sync"
  actor VARCHAR(255),        -- e.g., "sre@company.com", "devops@ci"
  source VARCHAR(50),        -- "robusta", "gitops", "audit", "kubectl"
  diff_summary TEXT,         -- Human-readable diff (e.g., "2 → 5 replicas")
  payload JSONB,             -- Full webhook payload
  occurred_at TIMESTAMP,
  created_at TIMESTAMP,
  
  INDEX ON (cluster_id, kind, namespace, name),
  INDEX ON (occurred_at DESC)
);
```

### `incidents`
Represents alerts and issues

```sql
CREATE TABLE incidents (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  title VARCHAR(255),        -- Alert summary
  status VARCHAR(50),        -- "open", "resolved"
  resource_kind VARCHAR(100),
  resource_namespace VARCHAR(255),
  resource_name VARCHAR(255),
  alert_labels JSONB,        -- Alertmanager labels
  started_at TIMESTAMP,
  resolved_at TIMESTAMP DEFAULT NULL,
  created_at TIMESTAMP
);
```

---

## Service Startup Sequence

When you run `./scripts/start-dev.sh` or individual `make run-*` commands:

```
1. Docker Compose (PostgreSQL + Redis)
   $ docker-compose -f deploy/docker-compose.yml up -d
   └─ PostgreSQL 16 starts on :5433
   └─ Redis 7 starts on :6379

2. Database Migrations
   $ make migrate
   └─ Reads migrations/001_init.sql
   └─ Creates graph_nodes, graph_edges, change_events, incidents tables
   └─ Creates indices for fast queries

3. API Server
   $ make run-api
   $ go run ./cmd/api/main.go
   └─ Loads config from environment
   └─ Connects to PostgreSQL (via Store layer)
   └─ Starts HTTP server on :8080
   └─ Listens for REST calls and webhooks

4. Graph Builder
   $ make run-graph
   $ go run ./cmd/graph/main.go
   └─ Loads K8s config (KUBECONFIG env var)
   └─ Creates Kubernetes client
   └─ Starts informers for Deployments, Services, Ingresses, Pods
   └─ Watches K8s cluster for changes
   └─ On each event, updates PostgreSQL

5. Frontend
   $ make run-web
   $ npm run dev
   └─ Starts Vite dev server on :5176
   └─ React hot reload enabled
   └─ Browser auto-opens to http://localhost:5176

6. All Components Running
   ✓ Frontend polling API every 12s
   ✓ Graph builder watching K8s in real-time
   ✓ API serving dashboard data + accepting webhooks
   ✓ PostgreSQL persisting everything
```

---

## End-to-End Request Flow

### HTTP Request Example: SRE opens dashboard

```
Timeline: 0ms → 100ms

0ms   │ Browser loads http://localhost:5176/
      └─ React app mounts
      
5ms   │ useDashboard hook fires useEffect
      └─ Makes 4 parallel HTTP requests to API:
         - GET /api/v1/status
         - GET /api/v1/graph
         - GET /api/v1/changes
         - GET /api/v1/incidents
      
10ms  │ API Server receives requests
      │ handlers.go methods execute:
      ├─ Status() → store.GetStatus() → queries stats
      ├─ GetGraph() → store.GetGraph() → joins graph_nodes + graph_edges
      ├─ ListChanges() → store.ListChanges() → queries change_events
      └─ ListIncidents() → store.ListIncidents() → queries incidents
      
15ms  │ PostgreSQL processes queries
      │ Executes SQL (typically <5ms for indexed queries):
      ├─ SELECT COUNT(*) FROM graph_nodes  (with index)
      ├─ SELECT * FROM graph_nodes, graph_edges WHERE cluster_id=?
      ├─ SELECT * FROM change_events WHERE occurred_at > ? ORDER BY DESC LIMIT 50
      └─ SELECT * FROM incidents WHERE status='open' LIMIT 50
      
50ms  │ PostgreSQL returns result sets
      │ API server marshals JSON
      
60ms  │ HTTP responses sent to browser (4 responses, ~100KB total)

70ms  │ React receives JSON
      │ DashboardContext state updated
      
80ms  │ React components re-render
      ├─ HomePage: StatCard components display stats
      ├─ GraphPage: GraphCanvas renders nodes/edges with D3
      ├─ ChangesPage: ChangeTimeline renders change list
      └─ IncidentsPage: Incident cards displayed
      
100ms │ Page fully rendered
      │ User sees:
      ✓ 34 resources on graph
      ✓ 15 dependencies
      ✓ 2 recent changes
      ✓ 1 open incident

12s   │ Next polling cycle
      └─ useDashboard useEffect triggers again
      └─ Repeats the sequence
      └─ Updates changed data (diff updates only)
```

---

## Summary

| Component | Language | Purpose | Port |
|-----------|----------|---------|------|
| API Server | Go | REST API for dashboard | 8080 |
| Graph Builder | Go | K8s watcher | N/A (background) |
| Frontend | React/TS | UI/UX | 5176 |
| PostgreSQL | SQL | Data persistence | 5433 |
| Redis | Cache | Future caching layer | 6379 |

**All connected by**: PostgreSQL as single source of truth, HTTP polling for frontend sync, K8s informers for real-time change detection.
