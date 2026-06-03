# 🔄 End-to-End Code Flow

This document explains exactly what happens when you run the kube-dashboard system and how all the code pieces connect together.

---

## **PART 1: STARTUP SEQUENCE**

### Step 1: Docker Containers Start

```bash
$ docker-compose -f deploy/docker-compose.yml up -d
```

**File**: `deploy/docker-compose.yml`

**What Happens**:
1. Docker Compose reads the YAML configuration
2. Starts PostgreSQL 16 container on port 5433
   - Database name: `kubedashboard`
   - User: `kube`
   - Password: `kube`
3. Starts Redis 7 container on port 6379
4. Both containers run in background with `volumes` for persistence

**Output**:
```
Creating network "deploy_default" with the default driver
Creating deploy-postgres-1 ... done
Creating deploy-redis-1 ... done
```

---

### Step 2: Migrations Run

```bash
$ make migrate
```

**Execution Flow**:
```
Makefile → make migrate
  └─ go run ./cmd/api/main.go [with MIGRATION_PATH set]
     └─ main.go: Load config
     └─ main.go: st.Migrate(ctx, "migrations/001_init.sql")
        └─ store/postgres.go: Migrate(context, migrationFile)
           └─ Read migrations/001_init.sql
           └─ Parse SQL statements
           └─ Execute CREATE TABLE statements:
              • graph_nodes (with UNIQUE constraint)
              • graph_edges (with UNIQUE constraint, FK to graph_nodes)
              • change_events (with indices on cluster_id, kind, occurred_at)
              • incidents (main incident tracking table)
           └─ Exit after migrations complete
```

**File**: `migrations/001_init.sql`

**Tables Created**:
- `graph_nodes`: Stores K8s resources
- `graph_edges`: Stores relationships between resources
- `change_events`: Stores change history
- `incidents`: Stores alerts/issues

**Status After**:
```
PostgreSQL has 4 empty tables ready to receive data
```

---

### Step 3: API Server Starts

```bash
$ make run-api
```

**Execution Flow**:
```
Makefile → go run ./cmd/api/main.go
  │
  └─ cmd/api/main.go
     │
     ├─ 1. config.Load()
     │  └─ Reads environment variables:
     │     • DATABASE_URL (postgres://kube:kube@localhost:5433/kubedashboard)
     │     • CLUSTER_ID (local)
     │     • API_ADDR (:8080)
     │  └─ Returns Config struct
     │
     ├─ 2. signal.NotifyContext(ctx, syscall.SIGINT, SIGTERM)
     │  └─ Creates context that cancels on Ctrl+C or termination signals
     │
     ├─ 3. store.New(ctx, cfg.DatabaseURL)
     │  └─ internal/store/postgres.go: New()
     │     └─ Creates PostgreSQL connection pool
     │     └─ Tests connection (ping)
     │     └─ Returns *Store with db/sql connection
     │     └─ Store provides all query methods (UpsertGraphNode, InsertChangeEvent, etc.)
     │
     ├─ 4. st.Migrate(ctx, "migrations/001_init.sql") [if needed]
     │  └─ Already done in Step 2, skipped
     │
     ├─ 5. handlers.New(st, cfg)
     │  └─ internal/api/handlers/handlers.go: New()
     │     └─ Creates Handler struct with:
     │        • store: *Store (for DB access)
     │        • config: Config
     │
     ├─ 6. h.Register(mux)
     │  └─ internal/api/handlers/handlers.go: Register()
     │     └─ Registers all HTTP endpoints:
     │        mux.HandleFunc("GET /health", h.Health)
     │        mux.HandleFunc("GET /api/v1/status", h.Status)
     │        mux.HandleFunc("POST /api/v1/demo/seed", h.SeedDemo)
     │        mux.HandleFunc("GET /api/v1/graph", h.GetGraph)
     │        mux.HandleFunc("GET /api/v1/changes", h.ListChanges)
     │        mux.HandleFunc("POST /api/v1/changes", h.CreateChange)
     │        mux.HandleFunc("GET /api/v1/incidents", h.ListIncidents)
     │        mux.HandleFunc("POST /api/v1/incidents", h.CreateIncidentFromAlert)
     │        mux.HandleFunc("POST /api/v1/webhooks/robusta", h.RobustaWebhook)
     │        mux.HandleFunc("POST /api/v1/webhooks/gitops", h.GitOpsWebhook)
     │
     ├─ 7. Create HTTP Server
     │  └─ server := &http.Server{
     │       Addr: ":8080",
     │       Handler: middleware.CORS(mux)
     │     }
     │     └─ Wraps mux with CORS middleware
     │     └─ Allows cross-origin requests from frontend
     │
     ├─ 8. server.ListenAndServe() [in goroutine]
     │  └─ Starts blocking HTTP listener on :8080
     │  └─ Prints "api listening on :8080 (cluster_id=local)"
     │
     └─ 9. <-ctx.Done() [blocks here]
        └─ Waits for Ctrl+C signal
        └─ When received:
           └─ server.Shutdown()
           └─ st.Close() [close DB connections]
           └─ Exit
```

**API Server Now**:
- ✅ Connected to PostgreSQL
- ✅ Listening on http://localhost:8080
- ✅ Ready to serve dashboard requests
- ✅ Ready to accept webhooks

**Output**:
```
2026/05/27 00:24:24 api listening on :8080 (cluster_id=local)
```

---

### Step 4: Graph Builder Starts

```bash
$ make run-graph
```

**Execution Flow**:
```
Makefile → go run ./cmd/graph/main.go
  │
  └─ cmd/graph/main.go: main()
     │
     ├─ 1. signal.NotifyContext(ctx, SIGINT, SIGTERM)
     │  └─ Same as API: context cancels on Ctrl+C
     │
     ├─ 2. graph.NewBuilder(cfg.KubeConfig, st)
     │  └─ internal/graph/builder.go: NewBuilder()
     │     └─ Reads $HOME/.kube/config (K8s credentials)
     │     └─ Creates Kubernetes client (client-go)
     │     └─ Connects to K8s API server (127.0.0.1:50689)
     │     └─ Returns *Builder with:
     │        • clientset: *kubernetes.Clientset (K8s API client)
     │        • clusterID: "local"
     │        • store: *Store (for DB writes)
     │
     ├─ 3. b.Run(ctx)
     │  └─ internal/graph/builder.go: Run()
     │     │
     │     ├─ Creates SharedInformerFactory
     │     │  └─ factory := informers.NewSharedInformerFactory(clientset, 0)
     │     │     └─ "0" means resync period = disabled (uses watch-only)
     │     │
     │     ├─ Creates Informers for 4 resource types:
     │     │  ├─ deployments := factory.Apps().V1().Deployments().Informer()
     │     │  ├─ services := factory.Core().V1().Services().Informer()
     │     │  ├─ ingresses := factory.Networking().V1().Ingresses().Informer()
     │     │  └─ pods := factory.Core().V1().Pods().Informer()
     │     │
     │     ├─ Registers Event Handlers (for each resource type)
     │     │  │
     │     │  └─ For Deployments:
     │     │     └─ AddEventHandler(ResourceEventHandlerFuncs{
     │     │          AddFunc: func(obj) { b.onDeployment(ctx, obj) },
     │     │          UpdateFunc: func(_, obj) { b.onDeployment(ctx, obj) },
     │     │          DeleteFunc: [not used, we don't track deletes yet]
     │     │        })
     │     │     └─ Now: When a Deployment is added/updated:
     │     │        └─ onDeployment() is called
     │     │        └─ Extracts Deployment data
     │     │        └─ Creates GraphNode
     │     │        └─ Calls store.UpsertGraphNode() → INSERT/UPDATE in PostgreSQL
     │     │        └─ For each ConfigMap mount:
     │     │           └─ Calls store.UpsertGraphEdge() → INSERT edge "mounts"
     │     │
     │     ├─ factory.Start(ctx.Done())
     │     │  └─ Starts all informers
     │     │  └─ Begins watching K8s API for changes
     │     │
     │     ├─ factory.WaitForCacheSync(ctx.Done())
     │     │  └─ Blocks until all informer caches are populated
     │     │  └─ Typically ~1-2 seconds
     │     │  └─ After this, onDeployment/onService/onIngress/onPod have been called
     │     │     for all existing resources in cluster
     │     │  └─ PostgreSQL now has all current resources + relationships
     │     │
     │     ├─ Prints "graph builder running for cluster local"
     │     │
     │     └─ <-ctx.Done()
     │        └─ Blocks here waiting for Ctrl+C
     │        └─ Continuously watches K8s and updates DB
```

**Graph Builder Now**:
- ✅ Connected to Kubernetes cluster
- ✅ Watching 4 resource types
- ✅ PostgreSQL populated with all current K8s resources
- ✅ Listening for real-time changes

**Output**:
```
2026/05/27 00:28:47 graph builder running for cluster local (watching: deployments, services, ingresses, pods)
```

**At this point, PostgreSQL contains**:
```
graph_nodes:
  - 34 rows (Deployments, Services, Ingresses, ConfigMaps, Secrets, Pods)
  
graph_edges:
  - 26 rows (deployment→configmap "mounts", service→deployment "selects", etc.)
  
change_events: (empty, no webhooks sent yet)

incidents: (empty, no alerts fired yet)
```

---

### Step 5: Frontend Starts

```bash
$ make run-web
```

**Execution Flow**:
```
Makefile → npm run dev
  │
  └─ vite.config.ts: Vite dev server starts
     │
     ├─ Reads web/src/main.tsx (React entry point)
     │
     ├─ Transpiles TypeScript → JavaScript
     │  └─ tsconfig.json: compiler options
     │
     ├─ Bundles React components + libraries
     │
     ├─ Starts dev server on :5176
     │  └─ Prints "ready in 172 ms"
     │  └─ Local: http://localhost:5176/
     │
     └─ Opens browser to http://localhost:5176/
        │
        └─ Browser loads HTML
           │
           ├─ Loads web/index.html
           │  └─ <div id="root"></div>
           │  └─ <script type="module" src="/src/main.tsx"></script>
           │
           ├─ Executes main.tsx
           │  └─ ReactDOM.createRoot(document.getElementById("root"))
           │  └─ .render(<App />)
           │
           ├─ App.tsx mounts
           │  └─ Creates Router with 4 pages
           │  └─ Routes:
           │     • "/" → HomePage
           │     • "/graph" → GraphPage
           │     • "/changes" → ChangesPage
           │     • "/incidents" → IncidentsPage
           │
           ├─ Default page: HomePage
           │  │
           │  ├─ Renders <DashboardProvider>
           │  │  └─ Wraps entire app with DashboardContext
           │  │  └─ Provides dashboard state to all child components
           │  │
           │  ├─ Inside HomePage component
           │  │  │
           │  │  ├─ Calls useContext(DashboardContext)
           │  │  │  └─ Gets { resources, changes, incidents, stats }
           │  │  │
           │  │  ├─ Renders StatCard components
           │  │  │  └─ Displays stats.graph_nodes, stats.graph_edges, etc.
           │  │  │
           │  │  └─ Renders ChangeTimeline
           │  │     └─ Displays recent changes
           │
           └─ Front-end ready
              └─ User sees dashboard homepage
              └─ But shows default/empty stats (not yet fetched)
```

**Frontend Now**:
- ✅ React app running on http://localhost:5176
- ✅ Components mounted and ready
- ✅ Waiting to fetch data from API

---

## **PART 2: FIRST API REQUEST (Frontend Polling)**

### Step 6: Frontend Polling Data

**Timeline: 0-100ms after app loads**

```
0ms   │ HomePage component mounts
      │ useEffect hook runs (with empty dependency array)
      │
      ├─ ctx/DashboardContext.tsx: fetchDashboardData() called
      │  │
      │  ├─ Makes 4 parallel HTTP requests:
      │  │  1. GET http://localhost:8080/api/v1/status
      │  │  2. GET http://localhost:8080/api/v1/graph
      │  │  3. GET http://localhost:8080/api/v1/changes
      │  │  4. GET http://localhost:8080/api/v1/incidents
      │  │
      │  └─ All 4 requests sent simultaneously (Promise.all)
      │
10ms  │ Network travel time
      │ Requests reach API server
      │
      ├─ API Server processes each request
      │
      │  REQUEST 1: GET /api/v1/status
      │  └─ handlers.go: Status()
      │     ├─ store.GetStatus(ctx, clusterID)
      │     │  └─ store/postgres.go: GetStatus()
      │     │     └─ Executes SQL:
      │     │        SELECT 
      │     │          COUNT(*) as graph_nodes,
      │     │          (SELECT COUNT(*) FROM graph_edges WHERE cluster_id=?) as graph_edges,
      │     │          (SELECT COUNT(*) FROM change_events WHERE cluster_id=? AND occurred_at > ?) as changes_24h,
      │     │          (SELECT COUNT(*) FROM incidents WHERE cluster_id=? AND status='open') as open_incidents
      │     │        FROM graph_nodes WHERE cluster_id=?
      │     │     └─ Returns stats struct
      │     ├─ writeJSON(w, http.StatusOK, status)
      │     └─ Returns HTTP 200 + JSON response
      │
      │  REQUEST 2: GET /api/v1/graph
      │  └─ handlers.go: GetGraph()
      │     ├─ store.GetGraph(ctx, clusterID, namespace)
      │     │  └─ store/postgres.go: GetGraph()
      │     │     └─ Executes SQL:
      │     │        SELECT * FROM graph_nodes WHERE cluster_id=? [AND namespace=?]
      │     │        SELECT * FROM graph_edges WHERE cluster_id=?
      │     │     └─ Returns Graph{nodes, edges}
      │     ├─ writeJSON(w, http.StatusOK, graph)
      │     └─ Returns HTTP 200 + JSON with nodes + edges
      │
      │  REQUEST 3: GET /api/v1/changes
      │  └─ handlers.go: ListChanges()
      │     ├─ Parse query params (since, limit, kind, namespace, etc.)
      │     ├─ store.ListChanges(ctx, clusterID, namespace, kind, name, actor, since, limit)
      │     │  └─ store/postgres.go: ListChanges()
      │     │     └─ Executes SQL:
      │     │        SELECT * FROM change_events 
      │     │        WHERE cluster_id=? AND occurred_at > ? [AND kind=?] [AND namespace=?] ...
      │     │        ORDER BY occurred_at DESC
      │     │        LIMIT ?
      │     │     └─ Returns []ChangeEvent
      │     ├─ writeJSON(w, http.StatusOK, map{"changes": events})
      │     └─ Returns HTTP 200 + JSON with change list
      │
      │  REQUEST 4: GET /api/v1/incidents
      │  └─ handlers.go: ListIncidents()
      │     ├─ store.ListIncidents(ctx, clusterID, limit)
      │     │  └─ store/postgres.go: ListIncidents()
      │     │     └─ Executes SQL:
      │     │        SELECT * FROM incidents
      │     │        WHERE cluster_id=? AND status='open'
      │     │        LIMIT ?
      │     │     └─ Returns []Incident
      │     ├─ writeJSON(w, http.StatusOK, map{"incidents": incidents})
      │     └─ Returns HTTP 200 + JSON with incident list
      │
50ms  │ All 4 responses ready
      │ Sent back to browser
      │
      ├─ Browser receives 4 JSON responses
      │
      ├─ React context (DashboardContext) updates state:
      │  ├─ setStats(response1.stats)
      │  ├─ setResources(response2.nodes + response2.edges)
      │  ├─ setChanges(response3.changes)
      │  └─ setIncidents(response4.incidents)
      │
      ├─ Components re-render with new data
      │  ├─ StatCard: displays "34 Nodes", "15 Edges", "0 Changes", "0 Incidents"
      │  ├─ GraphCanvas: D3 visualization renders 34 nodes + 15 edges
      │  ├─ ChangeTimeline: empty (no changes yet)
      │  └─ IncidentPage: empty (no incidents yet)
      │
100ms │ Dashboard fully rendered with live data
      └─ User sees complete topology
```

**State After First Poll**:
```json
{
  "stats": {
    "graph_nodes": 34,
    "graph_edges": 15,
    "changes_24h": 0,
    "open_incidents": 0
  },
  "resources": {
    "nodes": [
      {"kind": "Deployment", "name": "nginx", "namespace": "demo", "status": "ready", ...},
      {"kind": "Deployment", "name": "api", "namespace": "demo", "status": "ready", ...},
      ...
    ],
    "edges": [
      {"source": "nginx-deploy", "target": "nginx-configmap", "type": "mounts"},
      ...
    ]
  },
  "changes": [],
  "incidents": []
}
```

---

## **PART 3: REAL-TIME CHANGE (Scaling Deployment)**

### Step 7: SRE Scales Deployment

**Timeline: User manually scales nginx from 2→5 replicas**

```bash
$ kubectl scale deployment nginx --replicas=5 -n demo
```

```
IMMEDIATELY in Kubernetes:
  │
  ├─ Deployment `nginx` spec.replicas changed from 2 to 5
  │
  ├─ ReplicaSet controller (in K8s)
  │  └─ Detects Deployment has spec.replicas=5 but only 2 running
  │  └─ Creates 3 new Pod objects
  │  └─ Kubelet pulls image + starts containers
  │
  └─ Kubernetes events emitted:
     ├─ UPDATE event on Deployment
     └─ 3x ADD events on Pods

GRAPH BUILDER REACTS:
  │
  ├─ Deployment informer detects UPDATE event
  │  └─ Calls b.onDeployment(ctx, updatedDeploymentObject)
  │     └─ internal/graph/builder.go: onDeployment()
  │        ├─ Extracts Deployment data:
  │        │  ├─ name: "nginx"
  │        │  ├─ namespace: "demo"
  │        │  ├─ replicas: 5 (NEW!)
  │        │  └─ status: "not_ready" (not all ready yet)
  │        │
  │        ├─ Creates GraphNode struct
  │        ├─ Calls store.UpsertGraphNode(ctx, node)
  │        │  └─ store/postgres.go: UpsertGraphNode()
  │        │     └─ SQL: INSERT INTO graph_nodes (...) VALUES (...)
  │        │         ON CONFLICT (cluster_id, api_version, kind, namespace, name)
  │        │         DO UPDATE SET status='not_ready', updated_at=NOW()
  │        │     └─ PostgreSQL: Updates existing row (UPSERT)
  │        │
  │        └─ Processes volume mounts (ConfigMaps/Secrets)
  │           └─ For each volume:
  │              └─ store.UpsertGraphEdge() → edge "mounts"
  │
  ├─ Pod informer detects 3 ADD events
  │  └─ For each new pod:
  │     ├─ Calls b.onPod(ctx, newPodObject)
  │     │  └─ internal/graph/builder.go: onPod()
  │     │     ├─ Extracts Pod data:
  │     │     │  ├─ name: "nginx-b6485fcbb-xyz1" (auto-generated)
  │     │     │  ├─ namespace: "demo"
  │     │     │  ├─ status: "Pending" (container starting)
  │     │     │  └─ labels: {"app": "nginx", "pod-template-hash": "b6485fcbb"}
  │     │     │
  │     │     ├─ Creates GraphNode struct
  │     │     ├─ Calls store.UpsertGraphNode()
  │     │     │  └─ INSERT new Pod into graph_nodes
  │     │     │
  │     │     └─ Links pod to owner deployment
  │     │        ├─ Pod.ownerReferences[0].kind == "ReplicaSet"
  │     │        ├─ Looks up ReplicaSet's owner (Deployment)
  │     │        └─ Calls store.UpsertGraphEdge(pod→deployment, "owned_by")
  │     │           └─ INSERT edge into graph_edges
  │     │
  │     └─ Repeat for pods 2 and 3
  │
  └─ PostgreSQL updated
     ├─ graph_nodes: 1 Deployment row updated (replicas=5, status=not_ready)
     ├─ graph_nodes: 3 new Pod rows inserted
     ├─ graph_edges: 3 new "owned_by" edges inserted
     └─ stats changed: 34→37 nodes, 15→18 edges
```

**Status After Graph Builder Processes**:
```sql
-- PostgreSQL state:
SELECT COUNT(*) FROM graph_nodes;  -- 37 (was 34, +3 pods)

SELECT COUNT(*) FROM graph_edges;  -- 18 (was 15, +3 owned_by edges)

SELECT * FROM graph_nodes WHERE name LIKE 'nginx%' AND kind='Pod';
-- Returns 5 Pod rows (2 original + 3 new)
```

---

### Step 8: SRE Sends Webhook (Robusta)

**Timeline: 2 seconds after scaling (pods still starting)**

```bash
$ curl -X POST http://localhost:8080/api/v1/webhooks/robusta \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "Deployment",
    "name": "nginx",
    "namespace": "demo",
    "verb": "scale",
    "user": "sre@company.com",
    "diff": "replicas: 2 → 5"
  }'
```

```
WEBHOOK RECEIVED:
  │
  ├─ HTTP POST arrives at API server
  │
  ├─ handlers.go: RobustaWebhook()
  │  │
  │  ├─ Parse JSON body → map[string]interface{}
  │  │  └─ Extracts: kind, name, namespace, verb, user, diff
  │  │
  │  ├─ Create ChangeEventInput struct
  │  │  └─ models.ChangeEventInput{
  │  │      ClusterID:   "local",
  │  │      Kind:        "Deployment",
  │  │      Namespace:   "demo",
  │  │      Name:        "nginx",
  │  │      Verb:        "scale",
  │  │      Actor:       "sre@company.com",
  │  │      Source:      "robusta",
  │  │      DiffSummary: "replicas: 2 → 5",
  │  │      Payload:     {...full JSON...}
  │  │    }
  │  │
  │  ├─ store.InsertChangeEvent(ctx, changeEventInput)
  │  │  └─ store/postgres.go: InsertChangeEvent()
  │  │     ├─ Generate UUID for change
  │  │     ├─ INSERT INTO change_events (
  │  │     │    id, cluster_id, kind, namespace, name,
  │  │     │    verb, actor, source, diff_summary, payload,
  │  │     │    occurred_at, created_at
  │  │     │  ) VALUES (?)
  │  │     └─ Returns inserted ChangeEvent row
  │  │
  │  └─ Write HTTP 201 + JSON response
  │     └─ Returns ChangeEvent with ID, timestamps
  │
  └─ PostgreSQL updated
     ├─ change_events: 1 new row inserted
     │  (id, actor=sre@company.com, verb=scale, diff="2→5", source=robusta)
     └─ stats changed: changes_24h = 0→1
```

**HTTP Response**:
```json
{
  "id": "fd64e929-08ab-43b1-9102-7b71b692b7b7",
  "cluster_id": "local",
  "resource": {"kind": "Deployment", "namespace": "demo", "name": "nginx"},
  "verb": "scale",
  "actor": "sre@company.com",
  "source": "robusta",
  "diff_summary": "replicas: 2 → 5",
  "occurred_at": "2026-05-27T00:33:16+05:30"
}
```

---

### Step 9: Frontend Polling (12 seconds later)

**Timeline: Next automatic poll cycle (every 12s)**

```
12s   │ React useEffect timer fires
      │ fetchDashboardData() called again
      │
      ├─ 4 parallel HTTP requests again:
      │  1. GET /api/v1/status
      │  2. GET /api/v1/graph
      │  3. GET /api/v1/changes
      │  4. GET /api/v1/incidents
      │
      ├─ API Server responses:
      │  │
      │  ├─ /api/v1/status
      │  │  └─ SQL: SELECT COUNT(*) FROM graph_nodes, change_events (WHERE occurred_at > now()-24h), ...
      │  │  └─ Returns:
      │  │     {
      │  │       "stats": {
      │  │         "graph_nodes": 37,        ← Changed from 34 (+3 pods)
      │  │         "graph_edges": 18,        ← Changed from 15 (+3 edges)
      │  │         "changes_24h": 1,         ← Changed from 0 (+1 webhook)
      │  │         "open_incidents": 0       ← Still 0
      │  │       }
      │  │     }
      │  │
      │  ├─ /api/v1/graph
      │  │  └─ SQL: SELECT * FROM graph_nodes, graph_edges WHERE cluster_id=? ...
      │  │  └─ Returns updated graph with:
      │  │     - nginx Deployment now shows 5 pods instead of 2
      │  │     - 3 new Pod nodes with status "Pending" or "Ready"
      │  │     - 3 new edges: Pod→Deployment "owned_by"
      │  │
      │  ├─ /api/v1/changes (LIMIT 50)
      │  │  └─ SQL: SELECT * FROM change_events WHERE cluster_id=? ORDER BY occurred_at DESC LIMIT 50
      │  │  └─ Returns:
      │  │     {
      │  │       "changes": [
      │  │         {
      │  │           "id": "fd64e929...",
      │  │           "kind": "Deployment",
      │  │           "name": "nginx",
      │  │           "namespace": "demo",
      │  │           "verb": "scale",
      │  │           "actor": "sre@company.com",
      │  │           "source": "robusta",
      │  │           "diff_summary": "replicas: 2 → 5",
      │  │           "occurred_at": "2026-05-27T00:33:16+05:30"
      │  │         }
      │  │       ]
      │  │     }
      │  │
      │  └─ /api/v1/incidents
      │     └─ Returns empty (no alerts fired)
      │
      ├─ React receives responses
      │  └─ DashboardContext.setState() updates:
      │     ├─ setStats({...new stats...})
      │     ├─ setResources({...37 nodes, 18 edges...})
      │     ├─ setChanges([{scaling event}])
      │     └─ setIncidents([])
      │
      └─ Components re-render with new data
         ├─ StatCard: displays "37 Nodes" (was 34), "18 Edges" (was 15), "1 Change" (was 0)
         ├─ GraphCanvas: visualization updates with 3 new Pod nodes, edges repaint
         ├─ ChangeTimeline: shows 1 item "nginx scaled 2→5 by sre@company.com"
         └─ IncidentPage: still empty
```

**User Sees on Dashboard**:
```
BEFORE scaling:
✓ Nodes: 34
✓ Edges: 15
✓ Changes: 0
✓ Incidents: 0

AFTER scaling + polling:
✓ Nodes: 37          ← Updated
✓ Edges: 18          ← Updated
✓ Changes: 1         ← Updated
✓ Incidents: 0       ← Same
✓ New event: "nginx scaled 2→5 by sre@company.com"    ← Visible
```

---

## **PART 4: INCIDENT ALERT (Pod Crashes)**

### Step 10: Alert Fires

**Timeline: Pod container crashes (exit code 1)**

```
K8s detects pod container failure
  │
  ├─ Kubelet marks pod status: phase=Failed
  │
  ├─ kube-state-metrics (Prometheus exporter) scrapes pod status
  │  └─ Exports metric: kube_pod_container_status_last_state_reason{pod="nginx-xyz", reason="Error"}
  │
  ├─ Prometheus scrapes metric every 15s
  │  └─ Stores time-series data
  │
  ├─ Alert rule evaluates:
  │  └─ "KubePodCrashLooping": 
  │     if container restarts > 5 times in 10 minutes → FIRE alert
  │
  ├─ Alertmanager receives alert
  │  └─ Matches routing rules
  │  └─ Determines: send to Kube Dashboard webhook endpoint
  │
  └─ Alertmanager sends webhook to API:
     $ curl -X POST http://localhost:8080/api/v1/incidents \
       -H "Content-Type: application/json" \
       -d '{
         "alerts": [{
           "status": "firing",
           "labels": {
             "alertname": "KubePodCrashLooping",
             "pod": "nginx-xyz",
             "namespace": "demo",
             "severity": "critical"
           },
           "annotations": {
             "summary": "Pod nginx-xyz is crash looping",
             "description": "..."
           },
           "startsAt": "2026-05-27T00:40:00Z"
         }]
       }'
```

### Step 11: Incident Created

```
WEBHOOK RECEIVED:
  │
  ├─ HTTP POST arrives at API server
  │
  ├─ handlers.go: CreateIncidentFromAlert()
  │  │
  │  ├─ Parse Alertmanager webhook body
  │  │
  │  ├─ Extract first alert from payload
  │  │  └─ status: "firing"
  │  │  └─ labels: {alertname, pod, namespace, severity}
  │  │  └─ annotations: {summary, description}
  │  │
  │  ├─ Create Incident struct
  │  │  └─ models.Incident{
  │  │      ClusterID:      "local",
  │  │      Title:          "Pod nginx-xyz is crash looping",
  │  │      Status:         "open",              ← FIRING = open
  │  │      ResourceName:   "nginx-xyz",
  │  │      ResourceNS:     "demo",
  │  │      AlertLabels:    {...severity, pod, namespace...}
  │  │      StartedAt:      timestamp
  │  │    }
  │  │
  │  ├─ store.CreateIncident(ctx, incident)
  │  │  └─ store/postgres.go: CreateIncident()
  │  │     └─ INSERT INTO incidents (...) VALUES (...)
  │  │
  │  └─ Return HTTP 201 + Incident JSON
  │
  └─ PostgreSQL updated
     ├─ incidents: 1 new row inserted
     │  (id, title, status='open', resource_pod, alert_labels)
     └─ open_incidents count: 0→1
```

### Step 12: Frontend Polling (Next Cycle)

```
12s   │ Frontend polls API again
      │
      ├─ GET /api/v1/status
      │  └─ Returns: {stats: {open_incidents: 1}}  ← Changed!
      │
      ├─ GET /api/v1/incidents
      │  └─ Returns: {incidents: [{title: "Pod nginx-xyz is crash looping", status: "open"}]}
      │
      └─ React re-renders
         ├─ StatCard: displays "1 Incident" (was 0) - RED indicator
         ├─ IncidentsPage: Shows incident card with:
         │  ├─ Title: "Pod nginx-xyz is crash looping"
         │  ├─ Status: "open" (red badge)
         │  ├─ Severity: "critical"
         │  └─ Affected resource link
         └─ User sees alert on dashboard
```

---

## **PART 5: INCIDENT RESOLUTION**

### Step 13: SRE Fixes Pod

```bash
$ kubectl set image deployment/nginx nginx=nginx:1.21 --image-pull-policy=IfNotPresent
```

```
Pod recovers:
  ├─ New pod created with fixed image
  ├─ Container starts successfully
  ├─ kube-state-metrics updates metrics
  ├─ Prometheus detects: pod no longer crashing
  └─ Alert resolves (status=resolved sent by Alertmanager)
```

### Step 14: Resolved Alert Received

```
Alertmanager sends webhook with status="resolved":
  │
  └─ API Server: CreateIncidentFromAlert()
     ├─ Parse webhook (status="resolved")
     ├─ Create Incident with status="resolved"
     ├─ INSERT into incidents (resolved_at=now)
     └─ Return HTTP 201
```

### Step 15: Frontend Polling

```
GET /api/v1/incidents (filters status='open')
  │
  ├─ SQL: SELECT * FROM incidents WHERE cluster_id=? AND status='open'
  │
  ├─ Returns: empty (resolved incident not included)
  │
  └─ React re-renders
     ├─ StatCard: "0 Incidents" (was 1)
     ├─ IncidentPage: no cards visible
     └─ Dashboard shows cleared alert
```

---

## Summary: Complete Flow

| Step | Component | Action | Database Change |
|------|-----------|--------|-----------------|
| 1 | Docker | Start PG + Redis | Tables created |
| 2 | API | Start server | Listening on :8080 |
| 3 | Graph Builder | Connect K8s | Populate 34 nodes, 15 edges |
| 4 | Frontend | Load app | Ready on :5176 |
| 5 | Frontend | Poll API | Display stats + graph |
| 6 | SRE | Scale deployment | Replicas: 2→5 |
| 7 | Graph Builder | Detect change | 37 nodes, 18 edges |
| 8 | SRE | Send webhook | +1 change event |
| 9 | Frontend | Poll API | Show 37 nodes, 1 change |
| 10 | Alertmanager | Fire alert | Pod crash detected |
| 11 | API | Receive alert | +1 open incident |
| 12 | Frontend | Poll API | Show 1 incident |
| 13 | SRE | Fix pod | Container recovers |
| 14 | Alertmanager | Resolve alert | Alert cleared |
| 15 | API | Receive resolved | Incident marked resolved |
| 16 | Frontend | Poll API | 0 incidents shown |

**All data flows through PostgreSQL. All UI updates via HTTP polling every 12 seconds.**
