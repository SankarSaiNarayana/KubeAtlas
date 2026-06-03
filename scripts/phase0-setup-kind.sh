#!/usr/bin/env bash
# Phase 0 — create kind cluster and deploy demo workload
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLUSTER_NAME="${KIND_CLUSTER_NAME:-kube-dashboard-dev}"

echo "==> Phase 0.1: Kind cluster '${CLUSTER_NAME}'"

if ! command -v kind >/dev/null 2>&1; then
  echo "Install kind: brew install kind"
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  kind create cluster --name "${CLUSTER_NAME}"
else
  echo "Cluster ${CLUSTER_NAME} already exists"
fi

kubectl cluster-info --context "kind-${CLUSTER_NAME}"

echo "==> Phase 0.2: Deploy demo namespace + app"
kubectl apply -f "${ROOT}/deploy/k8s/00-namespace.yaml" --context "kind-${CLUSTER_NAME}"
kubectl apply -f "${ROOT}/deploy/k8s/sample-demo.yaml" --context "kind-${CLUSTER_NAME}"

echo "==> Waiting for demo-api deployment"
kubectl rollout status deployment/demo-api -n demo --context "kind-${CLUSTER_NAME}" --timeout=120s

echo ""
echo "Phase 0 cluster setup complete."
echo "  kubectl --context kind-${CLUSTER_NAME} get all -n demo"
echo ""
echo "Next: ./scripts/phase0-verify.sh"
