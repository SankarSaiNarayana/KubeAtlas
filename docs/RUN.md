# KubeAtlas — How to Run

Run everything from the **repo root**.

## What runs where

```
Kubernetes cluster
       │
       ▼
  Go worker (: no port) ──watch──► Postgres (:5433)
       │                              ▲
       │ context collected            │ read/write
       ▼                              │
  Python AI (:8090) ◄──HTTP───────────┤
       │                              │
       ▼                              │
  Go API (:8080) ─────────────────────┘
       │
       ▼
  React UI (:5180)
```

| Process | Command | Needs |
|---------|---------|--------|
| Postgres | `make up` | Docker |
| Go API | `make run-api` | Postgres |
| Python AI | `make run-ai` | Python 3.11+ |
| Go worker | `make run-worker` | Postgres + kubectl + cluster |
| UI | `make run-web` | Go API running |

## One-time setup

```bash
make up
make migrate        # only if API/worker haven't run yet (they also migrate on start)
make setup-ai       # optional; run-ai does this automatically
```

Copy env if needed:

```bash
cp .env.example .env
```

Add to `.env` or export:

```bash
export DATABASE_URL=postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable
export AI_SERVICE_URL=http://localhost:8090
export API_ALLOWED_EMAILS=          # empty = no auth in dev
```

Optional LLM (Python uses LangChain):

```bash
export OPENAI_API_KEY=sk-...
```

## Start (4 terminals)

```bash
# 1 — API
make run-api

# 2 — Python AI
make run-ai

# 3 — Worker (watches your cluster)
make run-worker

# 4 — UI
make run-web
```

Open **http://localhost:5180**

## Verify

```bash
make check
```

## Pipeline flow

1. **Worker** watches Pods, Deployments, etc. → saves to `cluster_resources`
2. **Health engine** sets HEALTHY / WARNING / CRITICAL
3. **Incidents** stay visible until resource is healthy again
4. **Context** collected → **Python AI** investigates → remediations suggested
5. **Incidents page**: click **Run investigation** → see suggestions → **Verify & close** when done

## Common issues

| Problem | Fix |
|---------|-----|
| `address already in use :8080` | `export API_ADDR=:8081` or stop other process on 8080 |
| Worker: `kubernetes: kubeconfig` | `kubectl config use-context <your-cluster>` |
| Worker: `ai service unreachable` | Start `make run-ai` or unset `AI_SERVICE_URL` for Go fallback |
| UI disconnected | API not running; run `make run-api` |
| No resources in dashboard | Worker not running or not connected to cluster |

## Optional: Docker AI service

```bash
docker compose -f deploy/docker-compose.yml up -d ai
export AI_SERVICE_URL=http://localhost:8090
```
