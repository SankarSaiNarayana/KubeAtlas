# Quick Start — Run the Dashboard Locally

## Prerequisites

| Tool | Install (macOS) |
|------|-----------------|
| **Go 1.22+** | `brew install go` |
| **Docker** | Docker Desktop |
| **Node.js 18+** | nvm or `brew install node` |

```bash
go version && docker version && node --version
```

---

## Fastest path (one script)

```bash
cd /Users/yadlapallisankarsainarayana/kube_dashboard
chmod +x scripts/*.sh
./scripts/start-dev.sh
```

Opens the Vite UI on an available port (usually **http://localhost:5173**).
If 5173 is busy, Vite will automatically pick another port (e.g. **5176**).

---

## Manual steps (3 terminals)

### Terminal 1 — Database

```bash
cd /Users/yadlapallisankarsainarayana/kube_dashboard

docker compose -f deploy/docker-compose.yml up -d postgres

cp -n .env.example .env   # first time only
```

Postgres runs on **host port 5433** (avoids conflict if you already have Postgres on 5432).

### Terminal 2 — API

```bash
cd /Users/yadlapallisankarsainarayana/kube_dashboard

export PATH="/opt/homebrew/bin:$PATH"
go mod tidy
go run ./cmd/api
```

Verify:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/status
```

#### (Optional) Email allowlist auth

If you set `API_ALLOWED_EMAILS`, all API requests must include an email header.

Dev (disable auth):

```bash
export API_ALLOWED_EMAILS=
```

Prod-style (enable allowlist):

```bash
export API_ALLOWED_EMAILS="you@company.com"
export API_EMAIL_HEADER="X-Forwarded-Email"

curl -H "X-Forwarded-Email: you@company.com" http://localhost:8080/api/v1/status
```

Seed demo data (optional):

```bash
chmod +x scripts/seed-demo.sh
./scripts/seed-demo.sh
```

### Terminal 3 — Dashboard UI

```bash
cd /Users/yadlapallisankarsainarayana/kube_dashboard/web
npm install
npm run dev -- --host 0.0.0.0 --port 5173
```

Open: the URL printed by Vite (e.g. **http://localhost:5173**). If ports are in use, it may print **http://localhost:5176**.

---

## Optional — Graph from real cluster

Requires `~/.kube/config`:

```bash
export PATH="/opt/homebrew/bin:$PATH"
cd /Users/yadlapallisankarsainarayana/kube_dashboard
go run ./cmd/graph
```

Refresh the **Graph** page to see Deployments, Services, Ingress.

---

## Make targets

```bash
make docker-up    # Postgres + Redis
make migrate      # Run SQL migration
make run-api      # API server
make run-web      # React dev server
make run-graph    # K8s graph builder
make build        # Build all binaries to bin/
```

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `go: command not found` | `brew install go` |
| Port 5432 in use | We use **5433** — see `.env` |
| UI shows API unreachable | Start API first (`go run ./cmd/api`) |
| UI is blank / black | Open DevTools Console; ensure API auth isn’t blocking (see allowlist section). |
| Graph page empty | Run `go run ./cmd/graph` with valid kubeconfig |
| Build error on graph | Run `go mod tidy` then `go build ./...` |
| API returns 401/403 | Set `API_ALLOWED_EMAILS=` empty for dev, or send `X-User-Email` header |
| Vite port changed (not 5173) | Use the printed URL (e.g. `http://localhost:5176`). |

---

## What you'll see

| Page | Content |
|------|---------|
| **Overview** | API health, node/change/incident counts |
| **Graph** | K8s resources table (after graph builder) |
| **Changes** | Who changed what, when, from which source |
| **Incidents** | Alert-driven incidents (after seed or Alertmanager) |
