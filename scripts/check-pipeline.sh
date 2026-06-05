#!/usr/bin/env bash
# Quick health check for KubeAtlas pipeline
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export DATABASE_URL="${DATABASE_URL:-postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable}"
_raw_api="${API_ADDR:-:8080}"
case "$_raw_api" in
  :*) API="http://localhost${_raw_api}" ;;
  http*) API="$_raw_api" ;;
  *) API="http://$_raw_api" ;;
esac
AI="${AI_SERVICE_URL:-http://localhost:8090}"

ok=0
fail=0

check() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "OK   $name"
    ok=$((ok + 1))
  else
    echo "FAIL $name"
    fail=$((fail + 1))
  fi
}

echo "==> KubeAtlas pipeline check"
echo ""

check "Postgres" docker compose -f deploy/docker-compose.yml exec -T postgres pg_isready -U kube -d kubedashboard
check "Go API /health" curl -sf "${API%/}/health"
check "Atlas overview" curl -sf "${API%/}/api/v1/atlas/overview"
check "Python AI /health" curl -sf "${AI%/}/health"
check "kubectl context" kubectl config current-context
check "cluster reachable" kubectl get nodes --request-timeout=5s

echo ""
echo "Passed: $ok  Failed: $fail"
if [ "$fail" -gt 0 ]; then
  echo ""
  echo "Tips:"
  echo "  make up && make migrate"
  echo "  make run-api    (terminal 1)"
  echo "  make run-ai     (terminal 2)"
  echo "  make run-worker (terminal 3, needs kubeconfig)"
  exit 1
fi
