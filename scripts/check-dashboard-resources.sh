#!/usr/bin/env bash
set -euo pipefail

API_URL=${API_URL:-http://localhost:8080}

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required. install it and rerun."
  exit 1
fi

cluster_pods=$(mktemp)
dashboard_pods=$(mktemp)
cluster_deployments=$(mktemp)
dashboard_deployments=$(mktemp)
trap 'rm -f "$cluster_pods" "$dashboard_pods" "$cluster_deployments" "$dashboard_deployments"' EXIT

echo "Checking cluster pods against dashboard graph nodes..."
kubectl get pods --all-namespaces -o json | jq -r '.items[] | "Pod/\(.metadata.namespace)/\(.metadata.name) \(.status.phase)"' | sort > "$cluster_pods"
curl -sf "$API_URL/api/v1/graph" | jq -r '.nodes[] | select(.kind == "Pod") | "Pod/\(.namespace)/\(.name) \(.status)"' | sort > "$dashboard_pods"

echo "--- pods present in cluster but not in dashboard ---"
comm -23 "$cluster_pods" "$dashboard_pods" || true

echo "--- pods present in dashboard but not in cluster ---"
comm -13 "$cluster_pods" "$dashboard_pods" || true

echo

echo "Checking cluster deployments against dashboard graph nodes..."
kubectl get deploy --all-namespaces -o json | jq -r '.items[] | "Deployment/\(.metadata.namespace)/\(.metadata.name)"' | sort > "$cluster_deployments"
curl -sf "$API_URL/api/v1/graph" | jq -r '.nodes[] | select(.kind == "Deployment") | "Deployment/\(.namespace)/\(.name)"' | sort > "$dashboard_deployments"

echo "--- deployments present in cluster but not in dashboard ---"
comm -23 "$cluster_deployments" "$dashboard_deployments" || true

echo "--- deployments present in dashboard but not in cluster ---"
comm -13 "$cluster_deployments" "$dashboard_deployments" || true

echo

echo "To compare failed resources, run: kubectl get pods --all-namespaces | grep -E 'CrashLoopBackOff|Error|ImagePullBackOff|Pending'"