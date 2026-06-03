# Kube Dashboard — Open Source Tech Stack & Phased Plan

> Real-time operational intelligence for Kubernetes: live graph, change attribution, and incident context.
> **All tools listed below are free and open source** (no paid tiers required for MVP).

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Open Source Tech Stack](#open-source-tech-stack)
3. [Phase 0 — Scope & Baseline](#phase-0--scope--baseline)
4. [Phase 1 — Change Capture & Attribution](#phase-1--change-capture--attribution)
5. [Phase 2 — Operational Graph](#phase-2--operational-graph)
6. [Phase 3 — Dashboard MVP](#phase-3--dashboard-mvp)
7. [Phase 4 — Incident Intelligence](#phase-4--incident-intelligence)
8. [Phase 5 — RBAC, Policy & Multi-Cluster](#phase-5--rbac-policy--multi-cluster)
9. [What We Build vs What We Install](#what-we-build-vs-what-we-install)
10. [Local Development Order](#local-development-order)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     kube_dashboard (this repo)                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────────┐ │
│  │   web    │  │   api    │  │  ingest  │  │     graph       │ │
│  │  React   │──│  Go/HTTP │──│  Go      │  │  Go + client-go │ │
│  └──────────┘  └────┬─────┘  └────┬─────┘  └────────┬────────┘ │
└─────────────────────┼─────────────┼─────────────────┼──────────┘
                      │             │                 │
                      ▼             ▼                 ▼
                 PostgreSQL    Audit / GitOps    K8s API (watch)
                      │             │                 │
                      └─────────────┴─────────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              ▼                     ▼                     ▼
         Prometheus            Loki / Promtail        Argo CD or Flux
         Alertmanager          (audit logs)           (GitOps events)
         Grafana                Robusta (diffs)
```

---

## Open Source Tech Stack

### Core Platform (this repository)

| Component | Technology | License | Usage |
|-----------|------------|---------|-------|
| **API server** | Go 1.22+ (`cmd/api`) | BSD | REST API: graph, changes, incidents, health |
| **Change ingester** | Go + client-go (`cmd/ingest`) | Apache 2.0 | Consumes audit/GitOps webhooks; normalizes ChangeEvents |
| **Graph builder** | Go + client-go (`cmd/graph`) | Apache 2.0 | Watches K8s resources; builds nodes/edges in PostgreSQL |
| **Frontend** | React 18 + Vite + TypeScript (`web/`) | MIT | Service graph, change timeline, incident view |
| **Primary DB** | PostgreSQL 16 | PostgreSQL License | Changes, graph nodes/edges, incidents, metadata |
| **Cache** | Redis 7 (optional Phase 3+) | BSD | Hot graph slices, session cache |

### Observability Baseline (install in cluster)

| Tool | License | Usage |
|------|---------|-------|
| [Prometheus](https://github.com/prometheus/prometheus) | Apache 2.0 | Metrics collection; alert source |
| [Alertmanager](https://github.com/prometheus/alertmanager) | Apache 2.0 | Routes alerts to kube_dashboard webhook (Phase 4) |
| [Grafana](https://github.com/grafana/grafana) | AGPL 3.0 | Temporary dashboards until custom UI is ready |
| [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) | Apache 2.0 | Helm bundle for Prometheus + Grafana + Alertmanager |

### Change & Attribution (Phase 1)

| Tool | License | Usage |
|------|---------|-------|
| **Kubernetes Audit Logs** | Apache 2.0 | Apiserver policy: who changed what (kubectl, controllers, SA) |
| [Grafana Loki](https://github.com/grafana/loki) | AGPL 3.0 | Store and query audit log lines (lighter than OpenSearch) |
| [Promtail](https://github.com/grafana/promtail) | AGPL 3.0 | Ship audit logs from nodes to Loki |
| [Robusta](https://github.com/robusta-dev/robusta) | MIT | Resource diffs via `resource_babysitter`; push JSON to ingest API |
| [Argo CD](https://github.com/argoproj/argo-cd) **or** [Flux CD](https://github.com/fluxcd/flux2) | Apache 2.0 | GitOps sync events → planned change attribution |

### Operational Graph (Phase 2)

| Tool | License | Usage |
|------|---------|-------|
| [Ariadne](https://github.com/aalpar/ariadne) | Apache 2.0 | Go library for K8s dependency edges (integrate later) |
| [client-go](https://github.com/kubernetes/client-go) | Apache 2.0 | Informers/watch for Deployments, Services, Ingress, RBAC, etc. |
| PostgreSQL adjacency tables | — | MVP graph storage (no separate graph DB required initially) |

### Optional Graph DB (Phase 2b — when graph grows)

| Tool | License | Usage |
|------|---------|-------|
| [Memgraph](https://github.com/memgraph/memgraph) | BSL / community | Cypher queries, blast-radius traversals at scale |

### Incident Intelligence (Phase 4)

| Tool | License | Usage |
|------|---------|-------|
| [Keep](https://github.com/keephq/keep) | Apache 2.0 | Open-source alert hub; dedup, correlation, workflows |
| [HolmesGPT](https://github.com/robusta-dev/holmesgpt) | Apache 2.0 | CNCF AI investigator (read-only); optional "Investigate" button |
| [K8sGPT](https://github.com/k8sgpt-ai/k8sgpt) | Apache 2.0 | Lightweight K8s diagnostics |
| [kroot](https://github.com/AnonJon/kroot) | — | CLI blast-radius analysis; wrap or port logic |

### Policy & Security (Phase 5)

| Tool | License | Usage |
|------|---------|-------|
| [Kyverno](https://github.com/kyverno/kyverno) | Apache 2.0 | PolicyReports → policy violation nodes on graph |
| [Falco](https://github.com/falcosecurity/falco) | Apache 2.0 | Runtime security events on incident timeline |
| [rbac-lookup](https://github.com/cyberark/rbac-lookup) | Apache 2.0 | CLI: who has access to a resource |

### Runtime Network Dependencies (Phase 5+)

| Tool | License | Usage |
|------|---------|-------|
| [Cilium](https://github.com/cilium/cilium) + [Hubble](https://github.com/cilium/hubble) | Apache 2.0 | Real Pod→Service traffic edges |

### Local Development

| Tool | License | Usage |
|------|---------|-------|
| [Docker Compose](https://github.com/docker/compose) | Apache 2.0 | Local PostgreSQL + Redis |
| [kind](https://github.com/kubernetes-sigs/kind) | Apache 2.0 | Local Kubernetes cluster for testing |
| [Helm](https://github.com/helm/helm) | Apache 2.0 | Deploy stack to cluster |

### Explicitly NOT the platform (supporting tools only)

| Tool | Role |
|------|------|
| Argo CD / Flux | Change **data source** only |
| Grafana | Temporary metrics UI |
| kubectl-tree / kube-lineage | Manual debug CLI |

---

## Phase 0 — Scope & Baseline

**Duration:** Week 1  
**Goal:** One cluster, 1–2 namespaces, 2–3 services. Observability baseline running.

### Steps

- [ ] **0.1** Create local cluster with `kind` or use existing dev cluster
- [ ] **0.2** Deploy sample app (nginx or your real service) in namespace `demo`
- [ ] **0.3** Install kube-prometheus-stack (Prometheus + Grafana + Alertmanager)
- [ ] **0.4** Verify metrics and a test alert fire in Alertmanager
- [ ] **0.5** Document target namespaces and services in `docs/SCOPE.md`
- [ ] **0.6** Run `docker compose up` for local PostgreSQL + Redis
- [ ] **0.7** Run migrations: `make migrate`
- [ ] **0.8** Start API: `make run-api` — confirm `GET /health` returns OK

### Exit Criteria

- Prometheus scraping cluster; Grafana accessible
- PostgreSQL running; API health check passes
- One demo Deployment visible via `kubectl get deploy -n demo`

---

## Phase 1 — Change Capture & Attribution

**Duration:** Weeks 2–4  
**Goal:** Answer "what changed, when, and by whom" for core resources.

### Steps

- [ ] **1.1** Enable Kubernetes audit policy (see `deploy/k8s/audit-policy.yaml`)
- [ ] **1.2** Deploy Promtail → Loki for audit log shipping
- [ ] **1.3** Implement audit log parser in `cmd/ingest` (or poll Loki API)
- [ ] **1.4** Install Robusta; configure `json_change_tracker` webhook → `POST /api/v1/changes`
- [ ] **1.5** Install Argo CD **or** Flux; wire sync webhooks to ingest API
- [ ] **1.6** Normalize all sources into `ChangeEvent` model (see `internal/models/change.go`)
- [ ] **1.7** Store events in PostgreSQL `change_events` table
- [ ] **1.8** API: `GET /api/v1/changes?resource=&since=&actor=`
- [ ] **1.9** Test: deploy a change → see event with actor within 60 seconds

### Resources to Track (minimum)

- Deployments, StatefulSets, DaemonSets
- Services, Ingresses
- ConfigMaps, Secrets (metadata only)
- Roles, RoleBindings, ClusterRoles, ClusterRoleBindings

### Exit Criteria

- Query all changes to a Deployment in last 24h with actor in < 2s

---

## Phase 2 — Operational Graph

**Duration:** Weeks 4–7  
**Goal:** Live dependency map linked to change events.

### Steps

- [ ] **2.1** Graph builder watches core resources via informers (`cmd/graph`)
- [ ] **2.2** Persist nodes in `graph_nodes`, edges in `graph_edges`
- [ ] **2.3** Edge types: `owns`, `selects`, `mounts`, `references`, `exposes`
- [ ] **2.4** Link ChangeEvents to graph nodes by resource UID/GVK+ns+name
- [ ] **2.5** API: `GET /api/v1/graph?namespace=`
- [ ] **2.6** API: `GET /api/v1/resources/{id}/neighbors?direction=both&depth=2`
- [ ] **2.7** API: `GET /api/v1/resources/{id}/changes?since=1h`
- [ ] **2.8** (Optional) Integrate Ariadne library for richer CRD edges
- [ ] **2.9** (Optional) Add Memgraph sync for Cypher blast-radius queries

### Exit Criteria

- Graph updates within ~30s of a deploy
- Upstream/downstream of a Service visible via API

---

## Phase 3 — Dashboard MVP

**Duration:** Weeks 7–10  
**Goal:** Single UI for SRE incident workflow.

### Steps

- [ ] **3.1** React app: layout, routing, API client
- [ ] **3.2** **Service detail page:** graph neighborhood + recent changes
- [ ] **3.3** **Change timeline page:** filter by namespace, resource, actor
- [ ] **3.4** **Graph explorer:** interactive Cytoscape.js view
- [ ] **3.5** **Cluster overview:** resource counts, recent change summary
- [ ] **3.6** Wire real-time polling (30s) or SSE for graph/change updates
- [ ] **3.7** Dockerize web + api; Helm chart for cluster deploy

### Exit Criteria

- SRE opens one page during test incident: sees dependencies + changes without kubectl

---

## Phase 4 — Incident Intelligence

**Duration:** Weeks 10–14  
**Goal:** Alert-driven incident mode with blast radius.

### Steps

- [ ] **4.1** Install Keep as alert hub (optional) or direct Alertmanager webhook
- [ ] **4.2** `POST /api/v1/incidents` webhook from Alertmanager
- [ ] **4.3** Incident model: link alert → resource → subgraph
- [ ] **4.4** Highlight changes in last 1h on affected subgraph
- [ ] **4.5** Integrate kroot or custom blast-radius traversal
- [ ] **4.6** Incident page in web UI
- [ ] **4.7** (Optional) HolmesGPT read-only investigate integration

### Exit Criteria

- Test alert → dashboard opens incident with subgraph, changes, impacted services

---

## Phase 5 — RBAC, Policy & Multi-Cluster

**Duration:** Weeks 14+  
**Goal:** Production hardening and enterprise scope.

### Steps

- [ ] **5.1** Kyverno PolicyReports → ingest as policy violation events
- [ ] **5.2** RBAC graph edges: RoleBinding → Subject, Role → Rule
- [ ] **5.3** Falco events on incident timeline
- [ ] **5.4** Multi-cluster: `cluster_id` on all nodes and changes
- [ ] **5.5** Cilium Hubble for runtime COMMUNICATES_WITH edges
- [ ] **5.6** Auth: OIDC for the cluster's existing SSO for dashboard

### Exit Criteria

- Two clusters visible; RBAC change shows affected workloads on graph

---

## What We Build vs What We Install

| Install (OSS) | Build in this repo |
|---------------|-------------------|
| Prometheus, Loki, Grafana | ChangeEvent schema + multi-source merge |
| Argo CD / Flux, Robusta | Ingest service + PostgreSQL storage |
| Kyverno, Falco (later) | Graph builder + link changes to nodes |
| Keep, HolmesGPT (later) | Dashboard + incident mode UX |
| Memgraph (optional) | Correlation: "change before alert" |

**Our product IP:** unified graph + change attribution + incident-first dashboard.

---

## Local Development Order

Run these commands in order when starting development:

```bash
# 1. Start local dependencies
docker compose up -d

# 2. Run database migrations
make migrate

# 3. Terminal A — API server
make run-api

# 4. Terminal B — Graph builder (needs kubeconfig)
make run-graph

# 5. Terminal C — Change ingester
make run-ingest

# 6. Terminal D — Frontend
make run-web
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable` | PostgreSQL connection |
| `REDIS_URL` | `redis://localhost:6379` | Redis (optional) |
| `API_ADDR` | `:8080` | API listen address |
| `CLUSTER_ID` | `local` | Cluster identifier for multi-cluster |
| `KUBECONFIG` | `~/.kube/config` | Kubernetes config for graph/ingest |

---

## Repository Structure

```
kube_dashboard/
├── cmd/
│   ├── api/          # REST API server
│   ├── ingest/       # Change event ingester
│   └── graph/        # K8s graph builder
├── internal/
│   ├── models/       # Shared data models
│   ├── store/        # PostgreSQL access
│   ├── api/          # HTTP handlers
│   ├── ingest/       # Audit/GitOps parsers
│   └── graph/        # Informer + edge resolution
├── web/              # React frontend
├── migrations/       # SQL schema
├── deploy/
│   ├── docker-compose.yml
│   └── k8s/          # Audit policy, sample manifests
├── docs/
│   └── PLAN.md       # This file
├── Makefile
└── README.md
```

---

## Next Action

Start **Phase 0**: run `docker compose up -d && make migrate && make run-api`, then deploy kube-prometheus-stack to your cluster.
