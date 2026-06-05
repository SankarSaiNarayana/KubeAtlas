from __future__ import annotations

from typing import Any

from kubernetes import client, config
from kubernetes.client import ApiException


def _load_kube_config() -> None:
    try:
        config.load_incluster_config()
    except Exception:
        config.load_kube_config()


def _normalize(obj: Any, kind: str) -> dict[str, Any]:
    metadata = getattr(obj, "metadata", None)
    return {
        "kind": kind,
        "name": getattr(metadata, "name", "") or "",
        "namespace": getattr(metadata, "namespace", "") or "",
        "uid": getattr(metadata, "uid", "") or "",
        "labels": getattr(metadata, "labels", {}) or {},
        "api_version": getattr(obj, "api_version", "") or "",
    }


def _list_resources(api: Any, method: str, kind: str) -> list[dict[str, Any]]:
    call = getattr(api, method)
    items = call().items
    return [_normalize(item, kind) for item in items]


def list_cluster_resources() -> dict[str, list[dict[str, Any]]]:
    _load_kube_config()
    core = client.CoreV1Api()
    apps = client.AppsV1Api()
    networking = client.NetworkingV1Api()

    try:
        return {
            "pods": _list_resources(core, "list_pod_for_all_namespaces", "Pod"),
            "services": _list_resources(core, "list_service_for_all_namespaces", "Service"),
            "namespaces": _list_resources(core, "list_namespace", "Namespace"),
            "nodes": _list_resources(core, "list_node", "Node"),
            "deployments": _list_resources(apps, "list_deployment_for_all_namespaces", "Deployment"),
            "replicasets": _list_resources(apps, "list_replica_set_for_all_namespaces", "ReplicaSet"),
            "statefulsets": _list_resources(apps, "list_stateful_set_for_all_namespaces", "StatefulSet"),
            "daemonsets": _list_resources(apps, "list_daemon_set_for_all_namespaces", "DaemonSet"),
            "ingresses": _list_resources(networking, "list_ingress_for_all_namespaces", "Ingress"),
        }
    except ApiException as exc:
        raise RuntimeError(f"kubernetes API error: {exc}") from exc
