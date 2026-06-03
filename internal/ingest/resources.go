package ingest

// resourceKind maps Kubernetes audit log resource (plural) to Kind name.
var resourceKind = map[string]string{
	"deployments":         "Deployment",
	"statefulsets":        "StatefulSet",
	"daemonsets":          "DaemonSet",
	"replicasets":         "ReplicaSet",
	"pods":                "Pod",
	"services":            "Service",
	"configmaps":          "ConfigMap",
	"secrets":             "Secret",
	"ingresses":           "Ingress",
	"networkpolicies":     "NetworkPolicy",
	"roles":               "Role",
	"rolebindings":        "RoleBinding",
	"clusterroles":        "ClusterRole",
	"clusterrolebindings": "ClusterRoleBinding",
}

func kindFromResource(resource string) string {
	if k, ok := resourceKind[resource]; ok {
		return k
	}
	// fallback: capitalize first letter, trim trailing s
	if resource == "" {
		return "Unknown"
	}
	return resource
}
