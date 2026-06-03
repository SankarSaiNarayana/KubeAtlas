# 📚 Kube Dashboard Code Documentation

Welcome to the Kube Dashboard source code. This document explains what each file does and how they all work together to create a real-time SRE dashboard for Kubernetes.

---

## 📁 Project Structure

```
kube_dashboard/
├── cmd/
│   ├── api/main.go              # API Server entry point
│   ├── graph/main.go            # Graph Builder entry point
│   └── ingest/main.go           # Ingest Service entry point
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── handlers.go      # HTTP endpoint handlers
│   │   │   └── status.go        # Status endpoint (optional)
│   │   └── middleware/
│   │       └── cors.go          # CORS middleware
│   ├── config/
│   │   └── config.go            # Configuration loader
│   ├── graph/
│   │   └── builder.go           # K8s resource watcher
│   ├── ingest/
│   │   ├── service.go           # Change capture service
│   │   └── tail.go              # Audit log tailing
│   ├── models/
│   │   └── models.go            # Data structures
│   └── store/
│       └── postgres.go          # Database queries
├── migrations/
│   └── 001_init.sql             # Database schema
├── web/                         # React frontend
├── docs/
│   ├── ARCHITECTURE.md          # System design
│   ├── FLOW.md                  # End-to-end flow
│   └── README.md                # This file
└── deploy/
    └── docker-compose.yml       # PostgreSQL + Redis setup
```

---

## 🚀 Quick Start

### 1. Start Infrastructure
```bash
docker-compose -f deploy/docker-compose.yml up -d  # PostgreSQL + Redis
make migrate                                         # Create database tables
```

### 2. Start Services
Open 3 terminals:

**Terminal 1: API Server**
```bash
make run-api
# Output: api listening on :8080 (cluster_id=local)
```

**Terminal 2: Graph Builder**
```bash
make run-graph
# Output: graph builder running for cluster local
```

**Terminal 3: Frontend**
```bash
make run-web
# Output: ready in 172 ms at http://localhost:5176
```

### 3. Open Dashboard
```bash
open http://localhost:5176
```

---

## 📖 File-by-File Explanation

### **cmd/api/main.go** - API Server Entry Point

**What it does:**
- Starts HTTP server on port 8080
- Connects to PostgreSQL database
- Initializes HTTP route handlers
- Gracefully shuts down on Ctrl+C

**Key steps:**
1. `config.Load()` - Read environment variables
2. `store.New()` - Connect to PostgreSQL
3. `st.Migrate()` - Create database tables
4. `handlers.New()` - Create HTTP handlers
5. `h.Register(mux)` - Register all routes (GET /api/v1/graph, POST /api/v1/webhooks/*, etc.)
6. `server.ListenAndServe()` - Start listening for HTTP requests

**Used by:** Frontend (http://localhost:5176) calls API endpoints
**Depends on:** PostgreSQL (port 5433), config, handlers, store

---

### **cmd/graph/main.go** - Graph Builder Entry Point

**What it does:**
- Connects to Kubernetes cluster
- Watches for resource changes (Deployments, Services, Ingresses, Pods)
- Updates PostgreSQL with graph topology

**Key steps:**
1. `config.Load()` - Read kubeconfig path
2. `graph.NewBuilder()` - Create K8s client connection
3. `b.Run()` - Start watching and updating database

**Used by:** Continuously runs in background
**Depends on:** Kubernetes cluster (KUBECONFIG), PostgreSQL, config

---

### **cmd/ingest/main.go** - Ingest Service Entry Point

**What it does:**
- Currently a placeholder (not actively used)
- Future: Will tail Kubernetes audit logs and send them to API

**Status:** Scaffolded but not wired into dashboard yet

---

### **internal/api/handlers/handlers.go** - HTTP Endpoint Handlers

**What it does:**
Implements all REST API endpoints that the frontend calls

**Endpoints implemented:**

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Health check (returns {status: ok}) |
| GET | `/api/v1/status` | Dashboard stats (nodes, edges, changes, incidents) |
| GET | `/api/v1/graph` | Full K8s resource graph (nodes + edges) |
| GET | `/api/v1/changes` | Change history with filtering |
| POST | `/api/v1/changes` | Create change event manually |
| GET | `/api/v1/incidents` | List incidents (alerts) |
| POST | `/api/v1/incidents` | Create incident from Alertmanager webhook |
| POST | `/api/v1/webhooks/robusta` | Accept Robusta scaling webhooks |
| POST | `/api/v1/webhooks/gitops` | Accept Argo CD / Flux sync webhooks |

**How it works:**
```
Frontend HTTP Request → Handler function (e.g., GetGraph)
  ↓
Handler parses query params
  ↓
Handler calls store.METHOD() (e.g., store.GetGraph())
  ↓
Store executes SQL query against PostgreSQL
  ↓
Handler marshals result to JSON
  ↓
HTTP response sent to frontend
```

**Key functions:**
- `Health()` - Returns {status: ok}
- `Status()` - Returns dashboard stats
- `GetGraph()` - Returns all nodes and edges
- `ListChanges()` - Returns recent changes
- `RobustaWebhook()` - Accepts Robusta webhooks
- `GitOpsWebhook()` - Accepts GitOps webhooks
- `CreateIncidentFromAlert()` - Accepts Alertmanager webhooks

---

### **internal/graph/builder.go** - K8s Watcher

**What it does:**
Watches Kubernetes cluster and maintains real-time graph in PostgreSQL

**How it works:**
```
Kubernetes API → K8s Client (client-go)
  ↓
Informers watch 4 resource types:
  • Deployments
  • Services
  • Ingresses
  • Pods
  ↓
When resource changes (ADD/UPDATE):
  • Extract resource data
  • Create GraphNode struct
  • Call store.UpsertGraphNode()
  ↓
PostgreSQL updated in real-time
```

**Key functions:**
- `NewBuilder()` - Create K8s client connection
- `Run()` - Start informers and watch for changes
- `onDeployment()` - Handle Deployment changes
- `onService()` - Handle Service changes and find selected Deployments
- `onIngress()` - Handle Ingress changes and find exposed Services
- `onPod()` - Handle Pod changes and link to owner Deployment
- `linkByName()` - Create dependency edges (mounts, selects, exposes, owned_by)

**Edge types detected:**
- `mounts`: Deployment mounts ConfigMap/Secret
- `selects`: Service selects Deployment (via label selector)
- `exposes`: Ingress exposes Service
- `owned_by`: Pod owned by Deployment

---

### **internal/store/postgres.go** - Database Layer

**What it does:**
Provides all SQL queries and database operations

**Key methods:**
- `New()` - Create connection pool to PostgreSQL
- `UpsertGraphNode()` - Insert/update K8s resource
- `UpsertGraphEdge()` - Insert/update resource relationship
- `InsertChangeEvent()` - Record a change (who did what when)
- `CreateIncident()` - Record alert/issue
- `ListChanges()` - Get change history
- `ListIncidents()` - Get open incidents
- `GetStatus()` - Get dashboard stats
- `GetGraph()` - Get all nodes and edges
- `Migrate()` - Create database schema

**SQL Tables:**
- `graph_nodes` - K8s resources (Deployments, Pods, Services, etc.)
- `graph_edges` - Relationships (mounts, selects, exposes, owned_by)
- `change_events` - Change history (who changed what when)
- `incidents` - Alerts and issues

---

### **internal/models/models.go** - Data Structures

**Key types:**

```go
// GraphNode represents a K8s resource
type GraphNode struct {
  ID         uuid.UUID           // Database ID
  ClusterID  string              // "local", "prod-us-east", etc.
  APIVersion string              // "apps/v1", "v1", "networking.k8s.io/v1"
  Kind       string              // "Deployment", "Pod", "Service"
  Namespace  string              // "default", "demo"
  Name       string              // "nginx", "api"
  Labels     map[string]string   // {"app": "nginx", "version": "1.0"}
  Status     string              // "ready", "not_ready", "pending"
  UpdatedAt  time.Time
}

// GraphEdge represents relationship between resources
type GraphEdge struct {
  ID       uuid.UUID // Database ID
  SourceID uuid.UUID // From resource
  TargetID uuid.UUID // To resource
  EdgeType string    // "mounts", "selects", "exposes", "owned_by"
}

// ChangeEvent records a change
type ChangeEvent struct {
  ID         uuid.UUID              // Database ID
  Kind       string                 // What changed (Deployment, ConfigMap, etc.)
  Name       string                 // Resource name
  Namespace  string                 // Resource namespace
  Verb       string                 // "scale", "update", "sync"
  Actor      string                 // "sre@company.com", "devops@ci"
  Source     string                 // "robusta", "gitops", "audit", "kubectl"
  DiffSummary string                // "2 → 5 replicas"
  OccurredAt time.Time              // When it happened
}

// Incident represents an alert or issue
type Incident struct {
  ID                uuid.UUID              // Database ID
  Title             string                 // "Pod nginx is crash looping"
  Status            string                 // "open", "resolved"
  ResourceName      string                 // "nginx"
  ResourceNamespace string                 // "demo"
  AlertLabels       map[string]interface{} // {"severity": "critical", "pod": "..."}
  StartedAt         time.Time
  ResolvedAt        *time.Time
}
```

---

### **internal/config/config.go** - Configuration

**What it does:**
Loads environment variables and provides config to all services

**Environment variables:**
- `DATABASE_URL` - PostgreSQL connection string (default: postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable)
- `CLUSTER_ID` - Cluster identifier (default: local)
- `KUBECONFIG` - Path to K8s config (default: $HOME/.kube/config)
- `API_ADDR` - API server listen address (default: :8080)
- `MIGRATION_PATH` - Path to SQL migrations (default: migrations/001_init.sql)

---

### **web/** - React Frontend

**What it does:**
Interactive dashboard UI showing K8s resources, changes, and incidents

**Key files:**
- `src/App.tsx` - Main app with router
- `src/pages/HomePage.tsx` - Dashboard stats and overview
- `src/pages/GraphPage.tsx` - Interactive topology graph
- `src/pages/ChangesPage.tsx` - Change history timeline
- `src/pages/IncidentsPage.tsx` - Alert list
- `src/components/GraphCanvas.tsx` - D3 visualization
- `src/context/DashboardContext.tsx` - Shared state (polling, caching)
- `src/hooks/useDashboard.ts` - Hook that polls API every 12 seconds
- `src/api/client.ts` - HTTP client for API calls

**Polling behavior:**
```
useEffect (runs every 12 seconds):
  ├─ Fetch GET /api/v1/status
  ├─ Fetch GET /api/v1/graph
  ├─ Fetch GET /api/v1/changes
  ├─ Fetch GET /api/v1/incidents
  └─ Update React state → components re-render
```

---

### **migrations/001_init.sql** - Database Schema

**Tables created:**

**graph_nodes**
```sql
CREATE TABLE graph_nodes (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  api_version VARCHAR(255),
  kind VARCHAR(100),
  namespace VARCHAR(255),
  name VARCHAR(255),
  labels JSONB,
  status VARCHAR(100),
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  UNIQUE(cluster_id, api_version, kind, namespace, name)
);
```

**graph_edges**
```sql
CREATE TABLE graph_edges (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  source_id UUID REFERENCES graph_nodes(id),
  target_id UUID REFERENCES graph_nodes(id),
  edge_type VARCHAR(50),
  metadata JSONB,
  created_at TIMESTAMP,
  UNIQUE(cluster_id, source_id, target_id, edge_type)
);
```

**change_events**
```sql
CREATE TABLE change_events (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  kind VARCHAR(100),
  namespace VARCHAR(255),
  name VARCHAR(255),
  verb VARCHAR(50),
  actor VARCHAR(255),
  source VARCHAR(50),
  diff_summary TEXT,
  payload JSONB,
  occurred_at TIMESTAMP,
  created_at TIMESTAMP,
  INDEX ON (cluster_id, kind, namespace, name),
  INDEX ON (occurred_at DESC)
);
```

**incidents**
```sql
CREATE TABLE incidents (
  id UUID PRIMARY KEY,
  cluster_id VARCHAR(255),
  title VARCHAR(255),
  status VARCHAR(50),
  resource_kind VARCHAR(100),
  resource_namespace VARCHAR(255),
  resource_name VARCHAR(255),
  alert_labels JSONB,
  started_at TIMESTAMP,
  resolved_at TIMESTAMP,
  created_at TIMESTAMP
);
```

---

### **deploy/docker-compose.yml** - Infrastructure

**Services:**
- PostgreSQL 16 (port 5433)
- Redis 7 (port 6379)

**Used by:**
- PostgreSQL: All data persistence (graph, changes, incidents)
- Redis: Currently unused (future caching layer)

---

## 🔄 Data Flow Summary

### Request Flow (Frontend → API → Database)

```
Frontend Browser
  │
  ├─ HTTP GET /api/v1/status
  │  └─ handlers.go: Status()
  │     └─ store.GetStatus()
  │        └─ SELECT COUNT(*) FROM graph_nodes WHERE cluster_id=?
  │           └─ PostgreSQL
  │              └─ Returns stats JSON
  │
  ├─ HTTP GET /api/v1/graph
  │  └─ handlers.go: GetGraph()
  │     └─ store.GetGraph()
  │        └─ SELECT * FROM graph_nodes, graph_edges WHERE cluster_id=?
  │           └─ PostgreSQL
  │              └─ Returns nodes + edges JSON
  │
  ├─ HTTP GET /api/v1/changes
  │  └─ handlers.go: ListChanges()
  │     └─ store.ListChanges()
  │        └─ SELECT * FROM change_events WHERE occurred_at > ?
  │           └─ PostgreSQL
  │              └─ Returns changes JSON
  │
  └─ HTTP GET /api/v1/incidents
     └─ handlers.go: ListIncidents()
        └─ store.ListIncidents()
           └─ SELECT * FROM incidents WHERE status='open'
              └─ PostgreSQL
                 └─ Returns incidents JSON
```

### Webhook Flow (External Tools → API → Database)

```
External Tool (Robusta / Argo CD / Alertmanager)
  │
  └─ HTTP POST /api/v1/webhooks/*
     │
     └─ handlers.go: RobustaWebhook() or GitOpsWebhook() or CreateIncidentFromAlert()
        │
        ├─ Parse JSON payload
        │
        └─ store.InsertChangeEvent() or store.CreateIncident()
           │
           └─ INSERT INTO change_events or incidents
              │
              └─ PostgreSQL updated
                 │
                 └─ Next frontend poll shows new data
```

### K8s Watcher Flow (K8s → Graph Builder → Database)

```
Kubernetes Cluster
  │
  ├─ Deployment changes (e.g., scale 2→5)
  │  └─ K8s informer detects UPDATE event
  │     └─ graph.go: onDeployment()
  │        └─ store.UpsertGraphNode()
  │           └─ INSERT/UPDATE graph_nodes
  │
  ├─ Pod created (by ReplicaSet controller)
  │  └─ K8s informer detects ADD event
  │     └─ graph.go: onPod()
  │        └─ store.UpsertGraphNode() + store.UpsertGraphEdge()
  │           └─ INSERT new Pod row + owned_by edge
  │
  └─ Service created with label selector
     └─ K8s informer detects ADD event
        └─ graph.go: onService()
           └─ Find matching Deployments
           └─ store.UpsertGraphEdge()
              └─ INSERT selects edges

PostgreSQL graph_nodes + graph_edges updated
  │
  └─ Next frontend poll shows updated topology
```

---

## 🎯 Key Concepts

### **Graph Nodes**
Represent Kubernetes resources (what we're tracking):
- Deployments
- Pods
- Services
- Ingresses
- ConfigMaps
- Secrets

### **Graph Edges**
Represent relationships between resources (how they're connected):
- `mounts`: Deployment uses ConfigMap/Secret as volume
- `selects`: Service uses label selector to find Pods from a Deployment
- `exposes`: Ingress exposes a Service to external traffic
- `owned_by`: Pod is owned/controlled by a Deployment

### **Change Events**
Record operational changes (who did what):
- Actor: Who made the change (email/user)
- Verb: What action (scale, update, sync)
- Diff: What changed (2 → 5 replicas)
- Source: How/where change came from (Robusta, GitOps, kubectl, audit)

### **Incidents**
Track alerts and issues (what went wrong):
- Status: open or resolved
- Title: Human-readable alert name
- Resource: What resource is affected
- Labels: Alert metadata (severity, alertname, etc.)

---

## 🛠️ How to Extend

### Add New Endpoint
1. Create handler in `internal/api/handlers/handlers.go`
2. Register in `Handler.Register()`
3. Call store methods as needed
4. Call `writeJSON()` to return response

### Add New K8s Resource Type
1. Add informer in `internal/graph/builder.go: Run()`
2. Create handler (e.g., `onStatefulSet()`)
3. Register handler in `handlers` slice
4. Implement handler logic (extract data, create GraphNode, create edges)

### Add New Webhook Source
1. Create handler in `internal/api/handlers/handlers.go`
2. Parse webhook payload
3. Call `store.InsertChangeEvent()`
4. Register route in `Handler.Register()`

---

## 📊 Database Queries Reference

```bash
# See all resources
docker exec deploy-postgres-1 psql -U kube -d kubedashboard -c "SELECT kind, COUNT(*) FROM graph_nodes GROUP BY kind;"

# See dependencies
docker exec deploy-postgres-1 psql -U kube -d kubedashboard -c "SELECT COUNT(*) FROM graph_edges GROUP BY edge_type;"

# See recent changes
docker exec deploy-postgres-1 psql -U kube -d kubedashboard -c "SELECT * FROM change_events ORDER BY occurred_at DESC LIMIT 5;"

# See incidents
docker exec deploy-postgres-1 psql -U kube -d kubedashboard -c "SELECT * FROM incidents WHERE status='open';"

# Clear test data
docker exec deploy-postgres-1 psql -U kube -d kubedashboard -c "DELETE FROM change_events; DELETE FROM incidents;"
```

---

## 🚨 Common Issues

### API not updating after K8s change
- Check Graph Builder is running: `ps aux | grep "cmd/graph"`
- Check PostgreSQL: `docker-compose ps`
- Check logs: `docker-compose logs postgres`

### Frontend shows empty graph
- Check API is running: http://localhost:8080/api/v1/status
- Check browser console for errors
- Ensure frontend is polling: check Network tab in DevTools

### Webhooks not being received
- Check API is listening: `lsof -i :8080`
- Check webhook URL is correct (http://localhost:8080, not 127.0.0.1)
- Check API logs for errors

---

## 📚 Additional Reading

- [ARCHITECTURE.md](./ARCHITECTURE.md) - System design and components
- [FLOW.md](./FLOW.md) - End-to-end flow from startup to incident resolution
- [QUICKSTART.md](./QUICKSTART.md) - Getting started guide
- [SCOPE.md](./SCOPE.md) - Current feature scope and status

---

**Ready to dive deeper? Start with [FLOW.md](./FLOW.md) to understand the complete system flow!**
