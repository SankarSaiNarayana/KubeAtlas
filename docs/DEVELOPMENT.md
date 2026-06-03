# Step-by-Step Development Guide

> Work through phases in order. Do not skip Phase 1 — change attribution is the core of this product.

**Current focus:** Phase 0 → Phase 1

---

## Progress tracker

| Phase | Name | Status | Exit test |
|-------|------|--------|-----------|
| **0** | Scope & baseline | 🟡 In progress | `./scripts/phase0-verify.sh` |
| **1** | Change capture | 🟡 Started | `./scripts/phase1-verify.sh` |
| **2** | Operational graph | 🟡 Partial | Graph updates after `kubectl apply` |
| **3** | Dashboard MVP | 🟡 Partial | Service detail + filters |
| **4** | Incident intelligence | ⬜ Not started | Alert → incident page |
| **5** | RBAC & multi-cluster | ⬜ Not started | RBAC on graph |

---

## How to run during development

Open **4 terminals** once Phase 0 is done:

```bash
# Terminal 1 — database
make docker-up

# Terminal 2 — API
make run-api

# Terminal 3 — graph builder (needs kubeconfig)
make run-graph

# Terminal 4 — change ingester (cluster watch OR audit log)
make run-ingest

# Terminal 5 — UI
make run-web
```

Dashboard: **http://localhost:5173**

---

# Phase 0 — Scope & Baseline

**Goal:** Local platform running + optional demo cluster.

**Duration:** ~1 day

### Step 0.1 — Install tools

```bash
brew install go kind kubectl
docker --version && go version
```

### Step 0.2 — Start PostgreSQL

```bash
cd kube_dashboard
make docker-up
cp -n .env.example .env
```

Postgres listens on **localhost:5433**.

### Step 0.3 — Start API (auto-runs migrations)

```bash
make run-api
# verify
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/status
```

### Step 0.4 — Start UI

```bash
make run-web
# open http://localhost:5173
# click "Load demo data" if empty
```

### Step 0.5 — Optional: kind cluster + demo app

```bash
chmod +x scripts/*.sh
./scripts/phase0-setup-kind.sh
kubectl get all -n demo
```

### Step 0.6 — Optional: Prometheus stack

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install monitoring prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace
```

### Step 0.7 — Verify Phase 0

```bash
./scripts/phase0-verify.sh
```

### Phase 0 exit criteria

- [ ] `curl /health` returns OK
- [ ] Dashboard shows connected (green pill)
- [ ] PostgreSQL container running
- [ ] (Optional) `demo-api` deployment in `demo` namespace

---

# Phase 1 — Change Capture & Attribution

**Goal:** Every mutation answers **what changed, when, who**.

**Duration:** ~1–2 weeks

### What we built in code

| Component | File | Purpose |
|-----------|------|---------|
| Cluster watch ingester | `internal/ingest/watcher.go` | Watches K8s without audit logs (dev mode) |
| Audit log parser | `internal/ingest/service.go` | Production path via `AUDIT_LOG_PATH` |
| GitOps webhook | `POST /api/v1/webhooks/gitops` | Argo CD / Flux sync events |
| Robusta webhook | `POST /api/v1/webhooks/robusta` | Resource diffs |
| Change API | `GET/POST /api/v1/changes` | Query + manual ingest |

### Step 1.1 — Start change ingester

```bash
make run-ingest
# logs: "cluster watch active for cluster local"
```

Uses `~/.kube/config` when `AUDIT_LOG_PATH` is not set.

### Step 1.2 — Trigger a real change

```bash
kubectl rollout restart deployment/demo-api -n demo
kubectl scale deployment demo-api -n demo --replicas=2
kubectl create configmap demo-cfg -n demo --from-literal=foo=bar
```

Refresh **Changes** page — events should appear within seconds.

### Step 1.3 — Verify Phase 1

```bash
./scripts/phase1-verify.sh
curl "http://localhost:8080/api/v1/changes?namespace=demo&since=1h"
```

### Step 1.3b — Useful query filters (Phase 1 enhancement)

You can filter the change timeline:

```bash
curl "http://localhost:8080/api/v1/changes?since=24h&source=cluster-watch"
curl "http://localhost:8080/api/v1/changes?since=24h&verb=update"
curl "http://localhost:8080/api/v1/changes?since=24h&actor=phase1-test"
```

### Step 1.4 — Production audit logs (later)

1. Apply `deploy/k8s/audit-policy.yaml` on your cluster apiserver
2. Ship logs with Promtail → Loki
3. Set `AUDIT_LOG_PATH=/var/log/kubernetes/audit.log` on ingest

### Step 1.5 — GitOps webhook (Argo CD example)

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/gitops \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "payments",
    "namespace": "demo",
    "revision": "abc123",
    "synced_by": "alice@company.com",
    "kind": "Deployment",
    "name": "demo-api"
  }'
```

### Phase 1 exit criteria

- [ ] `make run-ingest` running
- [ ] `kubectl` change → appears in Changes UI within 60s
- [ ] Query by actor works: `/api/v1/changes?actor=...`
- [ ] GitOps webhook creates event with `source: gitops`

---

# Phase 2 — Operational Graph

**Goal:** Live dependency map linked to changes.

**Status:** Graph builder exists for Deployment, Service, Ingress.

### Step 2.1 — Run graph builder

```bash
make run-graph
kubectl apply -f deploy/k8s/sample-demo.yaml
```

### Step 2.2 — Verify graph

```bash
curl http://localhost:8080/api/v1/graph?namespace=demo
```

Expected edges: `Ingress → Service → Deployment → ConfigMap`

### Step 2.3 — Next code tasks (TODO)

- [ ] Add Pod + StatefulSet + DaemonSet informers
- [ ] `GET /api/v1/resources/{id}/neighbors`
- [ ] `GET /api/v1/resources/{id}/changes?since=1h`
- [ ] Link change events to graph nodes by UID

### Phase 2 exit criteria

- [ ] Graph updates within ~30s of deploy
- [ ] Service upstream/downstream visible in API

---

# Phase 3 — Dashboard MVP

**Goal:** Incident-first single pane.

**Status:** Overview, Graph, Changes, Incidents pages exist.

### Next code tasks (TODO)

- [ ] Service detail page (`/resources/:kind/:ns/:name`)
- [ ] Filters on Changes (namespace, actor, source)
- [ ] Interactive graph (zoom/pan, click node)
- [ ] Docker + Helm chart for deploy

---

# Phase 4 — Incident Intelligence

**Goal:** Alert → subgraph + recent changes + blast radius.

### Next tasks (TODO)

- [ ] Wire Alertmanager to `POST /api/v1/incidents`
- [ ] Incident detail page with linked changes (last 1h)
- [ ] Blast radius traversal API
- [ ] Optional: HolmesGPT investigate button

---

# Phase 5 — RBAC, Policy, Multi-Cluster

### Next tasks (TODO)

- [ ] Kyverno PolicyReport ingest
- [ ] ClusterRole / ClusterRoleBinding in graph
- [ ] `cluster_id` on all nodes for multi-cluster
- [ ] Cilium Hubble runtime edges

---

## Quick reference

| Command | Phase |
|---------|-------|
| `./scripts/phase0-setup-kind.sh` | 0 |
| `./scripts/phase0-verify.sh` | 0 |
| `make run-api` | 0+ |
| `make run-ingest` | 1 |
| `make run-graph` | 2 |
| `./scripts/phase1-verify.sh` | 1 |
| `make run-web` | 3 |

See also: [PLAN.md](./PLAN.md) · [QUICKSTART.md](./QUICKSTART.md) · [EXECUTION_FLOW.md](./EXECUTION_FLOW.md)
