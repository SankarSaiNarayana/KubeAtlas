# KubeAtlas

AI-powered Kubernetes incident investigation and self-healing platform.

## Architecture

```
Cluster → collector → health → incidents → context → investigator → remediation
                                                              ↓
Human approval → execution → dashboard (SSE)
```

Packages: `internal/collector`, `health`, `incidents`, `context`, `investigator`, `remediation`, `execution`, `pipeline`, `realtime`.

## Architecture split

| Layer | Runtime | Responsibility |
|-------|---------|----------------|
| Control plane | **Go** | Discovery, health, incidents, context, approval, K8s execution |
| AI plane | **Python/FastAPI** | Investigation + remediation (LangChain when API keys set) |

Go calls `AI_SERVICE_URL` after context is collected. If the AI service is down, the worker falls back to in-process Go rules.

## Run

```bash
docker compose -f deploy/docker-compose.yml up -d postgres ai
export DATABASE_URL=postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable
export API_ALLOWED_EMAILS=
export AI_SERVICE_URL=http://localhost:8090

# Optional: enable LangChain LLM (otherwise rules engine in Python)
export OPENAI_API_KEY=sk-...

# Terminal 1 — REST + SSE
go run ./cmd/api

# Terminal 2 — Python AI (or: make run-ai)
cd services/ai && pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8090

# Terminal 3 — cluster watch + pipeline (requires kubeconfig)
go run ./cmd/worker

# Terminal 4 — UI
cd web && npm run dev -- --host 0.0.0.0 --port 5180
```

Open http://localhost:5180/

## API (Atlas)

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/atlas/overview` | Resource health + incident counts |
| `GET /api/v1/atlas/resources` | Discovered resources |
| `GET /api/v1/atlas/incidents` | Incidents (`?status=open\|resolved\|all`) |
| `GET /api/v1/atlas/investigations` | AI investigations |
| `GET /api/v1/atlas/remediations` | Pending/approved actions |
| `POST /api/v1/atlas/remediations/{id}/approve` | Human approval |
| `POST /api/v1/atlas/remediations/{id}/execute` | Run after approval |
| `GET /api/v1/events/stream` | SSE real-time updates |

## Workflow

1. Worker watches Pods, Deployments, ReplicaSets, StatefulSets, DaemonSets, Services, Ingresses, Nodes, Namespaces.
2. Health engine sets `HEALTHY` / `WARNING` / `CRITICAL` in `resource_health`.
3. Incidents open on `HEALTHY → WARNING/CRITICAL`, auto-resolve when healthy again.
4. Context builder collects logs, events, describe, YAML, node info.
5. Python AI service investigates (LangChain + OpenAI/Anthropic if configured, else rules).
6. Python proposes remediations; Go stores them; **never auto-executes**.
7. Operator approves in UI; execution runs via Kubernetes API.

## Multi-cluster

Set `CLUSTER_ID` per worker/API deployment; all tables include `cluster_id`.
