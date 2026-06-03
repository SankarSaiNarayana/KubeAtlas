# Kube Dashboard

Real-time operational intelligence for Kubernetes: live dependency graph, change attribution, and incident context.

## Quick Start

See **[docs/QUICKSTART.md](docs/QUICKSTART.md)** for full run instructions.

```bash
# One-command dev (Postgres + API + UI + demo data)
chmod +x scripts/*.sh && ./scripts/start-dev.sh

# Or manual:
docker compose -f deploy/docker-compose.yml up -d postgres
cp .env.example .env
go run ./cmd/api          # Terminal 1 → http://localhost:8080
cd web && npm install && npm run dev   # Terminal 2 → http://localhost:5173
```

Postgres uses host port **5433** (not 5432) to avoid conflicts.

## Project Structure

```
cmd/api/       REST API server
cmd/graph/     Kubernetes graph builder (informers)
cmd/ingest/    Audit log tail + change ingestion
internal/      Shared packages (models, store, handlers)
web/           React dashboard
migrations/    PostgreSQL schema
deploy/        Docker Compose + K8s manifests
docs/PLAN.md   Full OSS stack and phased plan
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/graph?namespace=` | Operational graph |
| GET | `/api/v1/changes?since=24h&actor=` | Change timeline |
| POST | `/api/v1/changes` | Ingest change event |
| GET | `/api/v1/incidents` | List incidents |
| POST | `/api/v1/incidents` | Alertmanager webhook |
| POST | `/api/v1/webhooks/robusta` | Robusta change webhook |

## Documentation

See [docs/PLAN.md](docs/PLAN.md) for the complete open-source tech stack, tool usage, and step-by-step phases.

## License

MIT
