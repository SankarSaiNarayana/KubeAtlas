#!/usr/bin/env bash
# Start local dev stack: Postgres + API + Web UI
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="/opt/homebrew/bin:${PATH:-}"

if ! command -v go >/dev/null; then
  echo "Go is required. Install: brew install go"
  exit 1
fi

if ! command -v docker >/dev/null; then
  echo "Docker is required for PostgreSQL."
  exit 1
fi

echo "==> Starting PostgreSQL (port 5433)..."
docker compose -f deploy/docker-compose.yml up -d postgres

echo "==> Waiting for Postgres..."
sleep 3

if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

export $(grep -v '^#' .env | xargs)

echo "==> Building API..."
go build -o bin/api ./cmd/api

echo "==> Starting API on ${API_ADDR:-:8080}..."
./bin/api &
API_PID=$!

trap 'kill $API_PID 2>/dev/null || true' EXIT

sleep 2
if curl -sf "http://localhost:8080/health" >/dev/null; then
  echo "API healthy"
  bash scripts/seed-demo.sh || true
else
  echo "Warning: API not responding yet"
fi

echo ""
echo "==> Starting web UI..."
echo "    Dashboard: http://localhost:5173"
echo "    API:       http://localhost:8080/health"
echo ""
cd web && npm run dev
