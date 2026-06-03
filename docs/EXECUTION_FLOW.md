# kube-dashboard Execution Flow

This document explains what happens inside the code when you start the kube-dashboard services. It describes the runtime pipeline in order and shows how the graph builder, API server, and database interact.

## 1. Startup sequence

The typical runtime flow is:

1. Start infrastructure: PostgreSQL and Redis via Docker Compose.
2. Start the graph builder service: `go run ./cmd/graph`.
3. Start the API service: `go run ./cmd/api`.
4. Optionally start the frontend: `cd web && npm run dev`.

Each step is described below.

---

## 2. Infrastructure setup

The Docker Compose file is `deploy/docker-compose.yml`.

It brings up:
- `postgres` on port `5433` with `POSTGRES_USER=kube`, `POSTGRES_PASSWORD=kube`, and `POSTGRES_DB=kubedashboard`
- `redis` on port `6379`

The graph builder and API code both use PostgreSQL through a `DATABASE_URL` environment variable.

If the local shell does not have `psql`, migrations can be applied directly inside the PostgreSQL container.

---

## 3. `go run ./cmd/graph` — graph builder startup

File: `cmd/graph/main.go`

### What happens

1. `config.Load()` loads configuration from environment variables (or `.env`):
   - `DATABASE_URL`
   - `API_ADDR`
   - `CLUSTER_ID`

2. The process creates a cancellable context that stops on `SIGINT`/`SIGTERM`.

3. `store.New(ctx, cfg.DatabaseURL)` opens a PostgreSQL connection pool and verifies connectivity.

4. `graph.NewBuilder(cfg.ClusterID, kubeconfig, st)` loads the Kubernetes kubeconfig and creates a `kubernetes.Clientset`.
   - If `KUBECONFIG` is not set, it defaults to `~/.kube/config`.
   - This is how the local host is connected to the Kubernetes cluster: the code reads kubeconfig and authenticates to the cluster API.
   - The graph builder then uses that API client to watch the live resources in the local cluster.

5. `builder.Run(ctx)` starts the graph builder.

### Inside `graph.Builder.Run`

File: `internal/graph/builder.go`

1. A shared informer factory is created for the Kubernetes client.
2. Informers are created for:
   - Deployments
   - Services
   - Ingresses
   - Pods
3. Each informer registers event handlers for add/update events.
4. The informers start and their caches sync.
5. The process logs that it is watching cluster resources.
6. The builder blocks forever, processing events until the context is cancelled.

### Event handlers

The graph builder updates the database whenever Kubernetes resources change.

#### `onDeployment`
- Builds a `GraphNode` for the Deployment.
- Sets `Kind = Deployment`, `Namespace`, `Name`, labels, and status.
- Calls `store.UpsertGraphNode()` to insert or update the node.
- Scans volume references for `ConfigMap` and `Secret` mounts.
- Creates `mounts` edges linking the Deployment to those referenced resources.

#### `onService`
- Builds a `GraphNode` for the Service.
- Calls `store.UpsertGraphNode()`.
- Lists Deployments in the Service namespace.
- Matches Service selectors against Deployment pod labels.
- Creates `selects` edges from Service → Deployment.

#### `onIngress`
- Builds a `GraphNode` for the Ingress.
- Calls `store.UpsertGraphNode()`.
- Visits HTTP rules and backend service references.
- Creates `exposes` edges from Ingress → Service.

#### `onPod`
- Builds a `GraphNode` for the Pod.
- Sets Pod status based on phase and container readiness.
- Calls `store.UpsertGraphNode()`.
- Links the Pod to its owner Deployment via ReplicaSet:
  - `owned_by` edge from Pod → Deployment.

#### `linkByName`
- Ensures a target node exists for the referenced resource.
- Upserts that target node if needed.
- Creates a `GraphEdge` between the source node and the target node.
- Edge types are: `mounts`, `selects`, `exposes`, `owned_by`.

---

## 4. `go run ./cmd/api` — API service startup

File: `cmd/api/main.go`

### What happens

1. `config.Load()` loads environment settings.
2. A cancellable context is created for graceful shutdown.
3. `store.New(ctx, cfg.DatabaseURL)` opens PostgreSQL.
4. `st.Migrate(ctx, migration)` executes `migrations/001_init.sql` to create required tables.
5. `http.NewServeMux()` creates the HTTP router.
6. `handlers.New(st, cfg)` constructs the API handler object.
7. `h.Register(mux)` attaches all HTTP endpoints.
8. The server is wrapped with `middleware.CORS(mux)`.
9. `server.ListenAndServe()` starts listening on the configured API address.
10. The process waits for cancellation and shuts down gracefully.

### CORS middleware

File: `internal/api/middleware/cors.go`

- Adds CORS response headers to allow cross-origin requests.
- Allows browsers to call the API from the frontend port.
- Handles `OPTIONS` preflight requests.

---

## 5. API request flow

File: `internal/api/handlers/handlers.go`

### Registered routes

- `GET /health` → `Handler.Health`
- `GET /api/v1/status` → `Handler.Status`
- `GET /api/v1/graph` → `Handler.GetGraph`
- `GET /api/v1/changes` → `Handler.ListChanges`
- `POST /api/v1/changes` → `Handler.CreateChange`
- `GET /api/v1/incidents` → `Handler.ListIncidents`
- `POST /api/v1/incidents` → `Handler.CreateIncidentFromAlert`
- `POST /api/v1/webhooks/robusta` → `Handler.RobustaWebhook`
- `POST /api/v1/webhooks/gitops` → `Handler.GitOpsWebhook`

### Health

- Returns a simple JSON payload confirming the API is running.

### Graph endpoint

- `GetGraph()` calls `store.GetGraph()`.
- Returns all graph nodes and edges to the frontend.
- Supports optional namespace filtering.

### Change endpoints

- `ListChanges()` loads change history from `change_events`.
- Supports filters: namespace, kind, name, actor, time window, limit.
- `CreateChange()` accepts manual change payloads and inserts them into the database.

### Incident endpoints

- `ListIncidents()` reads incident records.
- `CreateIncidentFromAlert()` accepts Alertmanager webhook payloads.
- It converts alert data into an incident record and stores it.

### Webhooks

- `RobustaWebhook()` receives scaler events from Robusta and records change/incident data.
- `GitOpsWebhook()` receives deployment sync events from GitOps tools and records changes.

---

## 6. Database layer

File: `internal/store/postgres.go`

### Connection and migration

- `Store.New()` opens a PostgreSQL pool and pings the database.
- `Store.Migrate()` reads `migrations/001_init.sql` and executes it.

### Graph persistence

- `UpsertGraphNode()` inserts or updates `graph_nodes`.
- `UpsertGraphEdge()` creates edges in `graph_edges`.
- `GetGraph()` fetches all nodes and edges for the requested cluster.

### Change events

- `InsertChangeEvent()` inserts records into `change_events`.
- `ListChanges()` queries past change events with filters.

### Incidents

- `CreateIncident()` inserts alert incidents.
- `ListIncidents()` returns recent incident records.

---

## 7. End-to-end behavior

When the system is running:

1. The graph builder watches Kubernetes resources.
2. Every change to Deployments, Services, Ingresses, and Pods is written to PostgreSQL.
3. The API service reads that graph and serves it to the frontend.
4. Webhooks and manual API calls can add change events and incidents.
5. The dashboard UI consumes the API and renders current state, history, and issues.

---

## 8. How the frontend fits in

The frontend is not described in full here, but its main behavior is:
- Query `/api/v1/graph` to render topology.
- Query `/api/v1/status` to show cluster health and counts.
- Query `/api/v1/changes` and `/api/v1/incidents` for timelines and issue lists.
- Send webhook or manual event data through the API.

---

## 9. Recommended command order

From a fresh shell:

```bash
cd /Users/yadlapallisankarsainarayana/kube_dashboard
docker compose -f deploy/docker-compose.yml up -d

docker exec -i deploy-postgres-1 psql -U kube -d kubedashboard -f - < migrations/001_init.sql

# In one terminal
go run ./cmd/graph

# In another terminal
go run ./cmd/api

# In a third terminal (optional frontend)
cd /Users/yadlapallisankarsainarayana/kube_dashboard/web
npm install
npm run dev
```

---

## 10. Quick troubleshooting

- If the API cannot reach PostgreSQL, verify `DATABASE_URL`.
- If graph nodes are missing, confirm the graph builder can access Kubernetes via `KUBECONFIG`.
- If the frontend is blocked by CORS, ensure the API server is running with the `CORS` middleware enabled.
