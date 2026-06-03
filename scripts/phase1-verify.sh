#!/usr/bin/env bash
# Phase 1 — trigger a change and verify it appears in the API
set -euo pipefail

export PATH="/opt/homebrew/bin:${PATH:-}"
API="${API_URL:-http://localhost:8080}"

echo "==> Phase 1 smoke test"
echo "Prerequisites: make docker-up, make run-api, make run-ingest (separate terminal)"

if ! curl -sf "${API}/health" >/dev/null; then
  echo "API not running at ${API}"
  exit 1
fi

BEFORE=$(curl -sf "${API}/api/v1/changes?since=24h" | grep -o '"id"' | wc -l | tr -d ' ')
echo "Changes before: ${BEFORE}"

echo "Posting test change..."
curl -sf -X POST "${API}/api/v1/changes" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "Deployment",
    "namespace": "demo",
    "name": "demo-api",
    "verb": "update",
    "actor": "phase1-test",
    "source": "api",
    "diff_summary": "phase1 smoke test change"
  }' >/dev/null

AFTER=$(curl -sf "${API}/api/v1/changes?since=24h" | grep -o '"id"' | wc -l | tr -d ' ')
echo "Changes after: ${AFTER}"

if [ "${AFTER}" -gt "${BEFORE}" ]; then
  echo "✓ Phase 1 change ingest works"
  curl -sf "${API}/api/v1/changes?since=24h&actor=phase1-test" | head -c 300
  echo ""
else
  echo "✗ Change was not recorded"
  exit 1
fi

if kubectl get deploy demo-api -n demo >/dev/null 2>&1; then
  echo ""
  echo "Optional: patch demo deployment to trigger cluster-watch ingest:"
  echo "  kubectl annotate deployment demo-api -n demo phase1/test=$(date +%s) --overwrite"
  echo "  kubectl rollout restart deployment demo-api -n demo"
fi
