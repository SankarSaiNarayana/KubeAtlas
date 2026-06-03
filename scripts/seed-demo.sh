#!/usr/bin/env bash
# Seed demo change events and an incident for local UI testing.
set -euo pipefail
API="${API_URL:-http://localhost:8080}"

echo "Seeding demo data to $API ..."

curl -sf -X POST "$API/api/v1/changes" -H "Content-Type: application/json" -d '{
  "kind": "Deployment",
  "namespace": "demo",
  "name": "payments-api",
  "verb": "update",
  "actor": "alice@company.com",
  "source": "gitops",
  "diff_summary": "image: payments-api:v1.2.0 -> v1.3.0"
}'

curl -sf -X POST "$API/api/v1/changes" -H "Content-Type: application/json" -d '{
  "kind": "Ingress",
  "namespace": "demo",
  "name": "payments-ingress",
  "verb": "patch",
  "actor": "bob@company.com",
  "source": "kubectl",
  "diff_summary": "tls secret reference updated"
}'

curl -sf -X POST "$API/api/v1/incidents" -H "Content-Type: application/json" -d '{
  "status": "firing",
  "alerts": [{
    "status": "firing",
    "labels": {
      "alertname": "KubePodCrashLooping",
      "namespace": "demo",
      "kind": "Pod",
      "pod": "payments-api-abc123"
    },
    "annotations": {
      "summary": "Pod payments-api is crash looping"
    },
    "startsAt": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"
  }]
}'

echo ""
echo "Done. Open http://localhost:5173"
