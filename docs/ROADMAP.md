# kube_dashboard — Forward Roadmap to Production

> Use this file as context for future development sessions. It captures:
> 1. Where the project stands today
> 2. The remaining phases (2 → 5) with concrete steps
> 3. Production-readiness work that runs **alongside** phases
> 4. My architectural opinions, trade-offs, and risks

**Repository:** `/Users/yadlapallisankarsainarayana/kube_dashboard`
**Last updated:** 2026-05-28

---

## 1. Current Status Snapshot

| Phase | Status | What works |
|-------|--------|------------|
| **0 – Baseline** | Complete | Postgres (5433), API on :8080, Web on :5173, health/status endpoints, demo seed |
| **1 – Change capture** | Complete (dev path) | `cluster-watch` ingester, audit log parser, GitOps/Robusta webhooks, dedup, filters (`source`, `verb`, `actor`, `kind`, `namespace`, `name`) |
| **2 – Operational graph** | Partial | Deployments / Services / Ingress → graph nodes + edges; ConfigMap/Secret mounts |
| **3 – Dashboard MVP** | Partial | Overview, Graph, Changes, Incidents pages; SVG graph; connection banner |
| **4 – Incident intelligence** | Not started | (Alertmanager webhook handler exists, no UX) |
| **5 – RBAC / multi-cluster** | Not started | – |

**Built artifacts** (verified compiles):
- `cmd/api`, `cmd/graph`, `cmd/ingest`
- `internal/{api,store,graph,ingest,models,config}`
- `web/` React + Vite + TS (4 pages, hooks/context, polling)
- `migrations/001_init.sql`, `deploy/docker-compose.yml`, `deploy/k8s/*`

---

## 2. Phase 2 — Operational Graph (next priority)

**Goal:** Every cluster resource is a node, every dependency is an edge, every change is **attached to its node**. This is what turns the Changes timeline into a *graph-aware* incident tool.

### 2.A. Schema additions (migration `002`)

```sql
ALTER TABLE change_events ADD COLUMN IF NOT EXISTS graph_node_id UUID;
CREATE INDEX IF NOT EXISTS idx_change_events_node ON change_events (graph_node_id);

CREATE TABLE IF NOT EXISTS node_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id     UUID NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,
    reason      TEXT,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 2.B. Code tasks

1. **Resolve `graph_node_id` on insert** in `InsertChangeEvent`:
   - Look up `graph_nodes` by `(cluster_id, kind, namespace, name)` and link.
2. **Resource detail API:**
   - `GET /api/v1/resources/{id}` → node + neighbors + last 50 changes.
   - `GET /api/v1/resources/{id}/neighbors?depth=2&direction=both`.
   - `GET /api/v1/resources/{id}/changes?since=1h`.
3. **More resource kinds in graph builder:** Pod, StatefulSet, DaemonSet, ReplicaSet, NetworkPolicy.
4. **Owner-references walker:** use `metadata.ownerReferences` to build `owns` edges between controllers and Pods.
5. **Garbage collection:** when an object is deleted in K8s, mark node `status='deleted'` (don't hard-delete — preserves history).

### 2.C. UX

- Click a node in the SVG graph → open **Resource Detail** drawer with:
  - Current spec summary
  - Last 10 changes (timeline)
  - Direct upstream/downstream links

### 2.D. Exit criteria

- Click any service → see all changes within 1s
- Graph reflects `kubectl apply` within ~30s
- Owner-reference chains render correctly (e.g. Deployment → ReplicaSet → Pod)

### 2.E. Open decision: graph backend

For < 5k nodes Postgres adjacency is fine. **Switch to Memgraph only when:**
- Cypher queries > 100ms p95, OR
- Multi-hop blast-radius queries are common in the UI

Until then keep it boring (Postgres).

---

## 3. Phase 3 — Dashboard MVP (parallel with Phase 2)

**Goal:** SRE opens **one URL during an incident** and sees everything they need.

### 3.A. Pages to add / improve

| Page | Status | What's missing |
|------|--------|----------------|
| Overview | Exists | Trend lines (changes/hour), top noisy actors |
| Graph | Exists (SVG) | Zoom, pan, click-to-detail, filter by namespace |
| Changes | Exists | Faceted filters (source, actor, verb, namespace) + URL query sync |
| Incidents | Exists | Detail view with subgraph + correlated changes |
| **Resource detail** | Missing | Build in Phase 2 |
| **Search** | Missing | Global search bar (kind/name/actor) |

### 3.B. Front-end stack additions

- `react-flow` or `cytoscape.js` for interactive graph (only if SVG hits limits).
- `tanstack/react-query` to replace ad-hoc fetch (caching, retries, polling consistency).
- `zod` for response validation (fail loud on schema drift).
- `tailwindcss` + `radix-ui` (optional, replace handwritten CSS if velocity matters more than file size).

### 3.C. Real-time

- v1: 12s polling (already done).
- v2: **SSE** from `/api/v1/changes/stream` (cheap on the server side, no WebSocket complexity).

### 3.D. Exit criteria

- Service detail page renders in < 500ms
- Filters persist in URL (shareable links)
- Mobile-readable (responsive grid already started)

---

## 4. Phase 4 — Incident Intelligence

**Goal:** Alert fires → dashboard opens an incident with **changes in last 1h on the affected subgraph**. This is the core product moment.

### 4.A. Code tasks

1. **Alertmanager webhook** (handler exists) — finish the resource-resolution:
   - `alert.labels.namespace + pod|deployment` → graph node lookup.
2. **Incident detail API & page:**
   - `GET /api/v1/incidents/{id}` → incident + subgraph (depth=2 from impacted node) + changes in window `[startsAt-1h, now]`.
3. **Blast-radius calculator:**
   - Recursive CTE in Postgres, depth-limited (default 3).
   - Score nodes by `severity = log(downstream_count + 1) × recency_weight`.
4. **Correlation rules:**
   - "Change occurred within X minutes before alert on same resource" → mark as **likely cause**.
   - Persist correlation: new table `incident_correlations(incident_id, change_id, score, reason)`.
5. **Optional: HolmesGPT integration** — call its read-only endpoints for an explanation; surface as `incident.ai_summary`.

### 4.B. Operational

- Webhook auth: shared secret header (`X-Alert-Token`) required for `POST /api/v1/incidents`.
- Rate limit incidents: collapse repeated alerts (same labels) within 5 min into one incident, increment `alert_count`.

### 4.C. Exit criteria

- Synthetic alert → incident page shows: subgraph + 3 ranked likely-cause changes + actor names
- p95 time-to-render incident < 1s after webhook hits

---

## 5. Phase 5 — RBAC, Policy, Multi-Cluster, Runtime

This is when **kube_dashboard** stops being a single-cluster tool and becomes a platform.

### 5.A. RBAC as graph

- Ingest: ClusterRole / Role / RoleBinding / ClusterRoleBinding / ServiceAccount.
- Edges:
  - `RoleBinding -[binds]-> Subject`
  - `RoleBinding -[grants]-> Role`
  - `Role -[allows{verb,resource}]-> RuleNode`
- Use case: "Who can delete Deployments in `payments`?" → graph query.

### 5.B. Policy

- Ingest **Kyverno PolicyReport / ClusterPolicyReport** every 30s.
- Render policy violations on the affected resource node (red badge).
- Tie to changes: "This change violates policy `require-image-tag`".

### 5.C. Runtime dependencies

- Optional: Cilium Hubble flow ingest → `Pod -[communicates_with]-> Pod/Service` edges.
- Cost: heavy; only enable in clusters with Cilium.

### 5.D. Multi-cluster

- Every model already has `cluster_id`. Need:
  - One `cmd/graph` + `cmd/ingest` process per cluster (or a single agent deployed per cluster pushing to API).
  - UI cluster selector + cross-cluster search.
  - Federated graph: same resource across clusters appears as separate nodes with a soft `same_as` link.

### 5.E. Exit criteria

- Two clusters render in one UI
- "What changed in RBAC for `payments` last 7 days?" answerable in one query
- Policy violations show up on graph nodes within 30s

---

## 6. Production-Readiness (runs in parallel with Phases 2–5)

This is the work that turns "demo on laptop" into "service we trust in production." Do **not** wait until Phase 5.

### 6.A. Security

- [ ] **Auth on the dashboard:** OIDC (Google/Okta/Keycloak) via `oauth2-proxy` in front of the API.
- [ ] **Email allowlist:** API enforces `API_ALLOWED_EMAILS` using `X-Forwarded-Email` from the auth proxy.
- [ ] **API tokens** for webhooks (Alertmanager, GitOps, Robusta) — never accept unauthenticated POST in prod.
- [ ] **RBAC for the dashboard itself:** read-only viewer vs. operator (who can seed/dismiss incidents).
- [ ] **Secret handling:** never log secret data; redact `Secret` resource payloads (only metadata).
- [ ] **CORS:** restrict to known origins in prod (currently `*` for dev).
- [ ] **Audit our own actions:** every dashboard mutation logged with user identity.

### 6.B. Deployment

- [ ] Dockerfile for `api`, `graph`, `ingest`, and `web` (multi-stage, distroless).
- [ ] Helm chart `deploy/helm/kube-dashboard/`:
  - `Deployment` per binary
  - `Service` + `Ingress`
  - `ConfigMap` for non-secret config, `Secret` for DB creds + webhook tokens
  - `ServiceAccount` + RBAC: read-only on `*` for graph/ingest
- [ ] Database: external managed Postgres in prod (not Docker Compose).
- [ ] Migration tool: switch from "run SQL on startup" to **`goose`** or **`golang-migrate`** with versioned files.

### 6.C. Observability of itself

- [ ] **Prometheus metrics** on each binary:
  - `kube_dashboard_changes_ingested_total{source,verb}`
  - `kube_dashboard_graph_nodes`
  - `kube_dashboard_http_requests_total{path,status}`
  - `kube_dashboard_db_query_duration_seconds`
- [ ] **Structured logs** (zap or zerolog) — JSON in prod.
- [ ] **/healthz** + **/readyz** distinct: ready only when DB ping + first informer cache sync done.
- [ ] **Tracing** (OpenTelemetry, optional): trace an alert → incident render.

### 6.D. Performance / scale

- [ ] **Connection pooling**: pgxpool sized via env (default 10).
- [ ] **Pagination** on `/api/v1/changes` (we have `limit`, add `cursor` based on `occurred_at`).
- [ ] **Backpressure on ingester:** bounded channel between informer event → DB insert; drop with metric if overloaded.
- [ ] **Partition `change_events` by month** when > 10M rows.
- [ ] **Compress old payloads** (`payload jsonb` → `bytea` zstd) after 30 days.

### 6.E. Reliability

- [ ] **Graph rebuild loop**: every 1h re-list all resources from K8s and reconcile DB. Drift kills informer-only systems.
- [ ] **Idempotent writes everywhere** (we already upsert nodes/edges; verify changes too if a UID + revision is available).
- [ ] **DLQ for webhooks:** if Postgres is down, drop webhook body to disk; replay on recovery.
- [ ] **Backup**: pg_dump nightly; keep 30 days.

### 6.F. CI / quality gates

- [ ] GitHub Actions:
  - `go vet`, `staticcheck`, `go test ./...`
  - `golangci-lint`
  - `npm run build` + `tsc --noEmit`
  - Build & push Docker images on tag
- [ ] **Integration test**: spin Postgres via testcontainers, run ingest + graph builder against a kind cluster, assert API contract.

### 6.G. Documentation

- [ ] **OpenAPI spec** (`docs/openapi.yaml`) generated from handler annotations.
- [ ] **Runbook**: "what to do when graph builder lags", "how to reset change_events".
- [ ] **Operator guide**: env vars, K8s RBAC needed, scaling tips.

---

## 7. Suggested Calendar (assuming 1 engineer, full time)

| Weeks | Work |
|-------|------|
| W1 | Phase 2.A–2.C — graph deepening + resource detail API |
| W2 | Phase 3 — service detail page + filters + interactive graph |
| W3 | Phase 4.A–4.B — incidents + correlation MVP |
| W4 | Phase 4.C — blast radius + UI |
| W5 | Production block: Dockerfiles + Helm chart + Prom metrics |
| W6 | Production block: auth (OIDC) + webhook tokens + structured logs |
| W7 | Phase 5.A — RBAC ingest + graph edges |
| W8 | Phase 5.B — Kyverno policy reports |
| W9 | Phase 5.D — multi-cluster (federation + UI selector) |
| W10 | Hardening: integration tests, runbooks, backups, partitioning |
| W11 | Closed beta with 1–2 internal teams |
| W12 | Iterate based on incident feedback |

Two engineers parallelizing brings this to ~6–7 weeks.

---

## 8. Architectural Opinions (the "why")

1. **Postgres before Memgraph.** Recursive CTEs handle blast-radius up to depth 4–5 on graphs with a few thousand nodes. Only move to a graph DB when you can prove a real query is slow. Avoid premature complexity.

2. **Informers over audit logs (for dev).** Audit logs are gold in prod but need cluster-admin and a log pipeline. The cluster-watch ingester gets you 80% of "who changed it" via `managedFields` and runs against any kubeconfig.

3. **GitOps is the source of truth for *intent*, the cluster is the source of truth for *state*. Show both.** A change banner that says "GitOps says v1.3 but cluster has v1.2" is more valuable than either alone.

4. **Don't build alerting. Consume it.** Alertmanager + Keep + Robusta exist. Kube_dashboard's job is to *enrich* alerts with graph + change context, not to detect them.

5. **Incident-first UX.** The home screen should answer "is anything on fire?" in 2 seconds. Everything else (graph explorer, change browsing) is secondary.

6. **One process per concern.** `api`, `graph`, `ingest` as separate binaries means each scales/restarts independently, and the failure of one (e.g., graph builder lagging) doesn't block the API.

7. **Schema migrations are sacred.** Use `golang-migrate` from Phase 2 onwards. Avoid the "run SQL on startup" pattern in prod.

8. **Multi-cluster on day one in the data model, day 30 in the UI.** Every table has `cluster_id`. UI can ignore it until you actually have a second cluster.

---

## 9. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Graph builder misses events on restart | High | Medium | 1h full re-list reconciliation loop (6.E) |
| `change_events` table grows unboundedly | High | High | Partition by month + cold storage after 30 days |
| Alertmanager floods incidents | Medium | High | Collapse by label set, rate limit |
| Webhook abuse | Medium | High | Shared-secret token, rate limit |
| K8s API version drift breaks informers | Medium | Medium | Pin `client-go` and CI test against multiple K8s versions |
| Single Postgres becomes bottleneck | Medium | Medium | Read replicas first, then partitioning |
| User loses trust due to stale data | Low | High | Surface "last sync" timestamp prominently in UI |

---

## 10. How to use this document in a future session

When opening a new chat, paste this prompt:

> Continue work on kube_dashboard. Read `docs/ROADMAP.md` and `docs/DEVELOPMENT.md`. We are currently at Phase **X** step **Y**. Start with task **Z**.

Replace `X/Y/Z` with the most recent thing you finished. The roadmap section IDs (2.A, 4.B, 6.C, …) are stable and safe to reference.

---

## Related docs

- `docs/PLAN.md` — original full plan (do not edit; historical)
- `docs/DEVELOPMENT.md` — per-phase how-to-run guide
- `docs/QUICKSTART.md` — fastest path to seeing the dashboard
- `docs/EXECUTION_FLOW.md` — runtime details of each binary
- `docs/ARCHITECTURE.md` — component diagram
- `docs/SCOPE.md` — current cluster + namespace target
