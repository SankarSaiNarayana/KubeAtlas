#!/usr/bin/env bash
# Phase 0 — verify local platform baseline
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="/opt/homebrew/bin:${PATH:-}"
FAIL=0

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; FAIL=1; }

echo "==> Phase 0 verification"
echo ""

echo "Tools"
command -v go >/dev/null && pass "go $(go version | awk '{print $3}')" || fail "go not installed (brew install go)"
command -v docker >/dev/null && pass "docker" || fail "docker not installed"
command -v kubectl >/dev/null && pass "kubectl" || fail "kubectl not installed"

echo ""
echo "Local data layer"
if docker compose -f "${ROOT}/deploy/docker-compose.yml" ps postgres 2>/dev/null | grep -q "running\|Up"; then
  pass "PostgreSQL container running"
else
  fail "PostgreSQL not running — run: make docker-up"
fi

echo ""
echo "API"
if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
  pass "API health http://localhost:8080/health"
  curl -sf http://localhost:8080/api/v1/status | head -c 200
  echo ""
else
  fail "API not reachable — run: make run-api"
fi

echo ""
echo "Kubernetes (optional)"
if kubectl config current-context >/dev/null 2>&1; then
  pass "kubeconfig context: $(kubectl config current-context)"
  if kubectl get deploy demo-api -n demo >/dev/null 2>&1; then
    pass "demo-api deployment in namespace demo"
  else
    echo "  · demo app not deployed — run: ./scripts/phase0-setup-kind.sh"
  fi
else
  echo "  · no kubeconfig — skip cluster checks or run phase0-setup-kind.sh"
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
  echo "Phase 0 checks passed. Start Phase 1: make run-ingest && make run-graph"
else
  echo "Fix failures above before Phase 1."
  exit 1
fi
