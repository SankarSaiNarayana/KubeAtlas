# Phase 0 Scope

## Cluster

- **Cluster ID:** `local`
- **Environment:** kind (`kube-dashboard-dev`) or existing dev cluster

## Namespaces

| Namespace | Purpose |
|-----------|---------|
| `demo` | Sample app for Phase 0–2 testing |

## Target Services

| Service | Namespace | Notes |
|---------|-----------|-------|
| `demo-api` | `demo` | nginx Deployment + Service + Ingress |

## Development phase status

| Phase | Status | Verified |
|-------|--------|----------|
| 0 — Baseline | In progress | `./scripts/phase0-verify.sh` |
| 1 — Changes | Started | `./scripts/phase1-verify.sh` |
| 2 — Graph | Partial | `make run-graph` |
| 3 — Dashboard | Partial | UI at :5173 |
| 4 — Incidents | Not started | — |
| 5 — RBAC/multi-cluster | Not started | — |

## GitOps (Phase 1+)

- [ ] Argo CD installed
- [ ] Flux installed
- **Chosen tool:** _none yet_ (webhook ready at `POST /api/v1/webhooks/gitops`)

## Exit checklist — Phase 0

- [ ] `make docker-up` succeeds
- [ ] `make run-api` → `GET /health` returns OK
- [ ] Dashboard connected at http://localhost:5173
- [ ] (Optional) `./scripts/phase0-setup-kind.sh` → demo-api running

## Exit checklist — Phase 1

- [ ] `make run-ingest` → cluster watch active
- [ ] `kubectl rollout restart deploy/demo-api -n demo` → change in UI
- [ ] `./scripts/phase1-verify.sh` passes

See [DEVELOPMENT.md](./DEVELOPMENT.md) for step-by-step instructions.
