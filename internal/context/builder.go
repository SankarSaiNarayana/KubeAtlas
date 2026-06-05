package context

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Builder struct {
	client kubernetes.Interface
	store  ports.ContextRepository
}

func NewBuilder(client kubernetes.Interface, store ports.ContextRepository) *Builder {
	return &Builder{client: client, store: store}
}

func (b *Builder) Collect(ctx context.Context, inc *domain.AtlasIncident, resource *domain.ClusterResource) (*domain.IncidentContext, error) {
	c := &domain.IncidentContext{IncidentID: inc.ID}

	switch resource.Kind {
	case "Pod":
		if err := b.collectPod(ctx, resource, c); err != nil {
			return nil, err
		}
	case "Deployment":
		if err := b.collectDeployment(ctx, resource, c); err != nil {
			return nil, err
		}
	default:
		if err := b.collectGeneric(ctx, resource, c); err != nil {
			return nil, err
		}
	}

	if err := b.store.SaveContext(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (b *Builder) collectPod(ctx context.Context, r *domain.ClusterResource, c *domain.IncidentContext) error {
	pod, err := b.client.CoreV1().Pods(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	c.RestartCount = totalRestarts(pod)
	c.ImageDetails, _ = json.Marshal(imagesFromPod(pod))
	c.EnvVars, _ = json.Marshal(envFromPod(pod))
	c.VolumeMounts, _ = json.Marshal(mountsFromPod(pod))
	c.DescribeData, _ = json.Marshal(describePod(pod))
	c.Events, _ = json.Marshal(b.fetchEvents(ctx, r.Namespace, pod.Name))
	c.Logs, _ = json.Marshal(b.fetchPodLogs(ctx, r.Namespace, pod.Name))
	if pod.Spec.NodeName != "" {
		node, err := b.client.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		if err == nil {
			c.NodeInfo, _ = json.Marshal(nodeSummary(node))
		}
	}
	if r.OwnerKind == "ReplicaSet" && r.OwnerName != "" {
		rs, err := b.client.AppsV1().ReplicaSets(r.Namespace).Get(ctx, r.OwnerName, metav1.GetOptions{})
		if err == nil {
			c.ReplicaSetInfo, _ = json.Marshal(rs.Status)
			if len(rs.OwnerReferences) > 0 && rs.OwnerReferences[0].Kind == "Deployment" {
				dep, err := b.client.AppsV1().Deployments(r.Namespace).Get(ctx, rs.OwnerReferences[0].Name, metav1.GetOptions{})
				if err == nil {
					y, _ := yaml.Marshal(dep)
					c.DeploymentYAML = string(y)
				}
			}
		}
	}
	return nil
}

func (b *Builder) collectDeployment(ctx context.Context, r *domain.ClusterResource, c *domain.IncidentContext) error {
	dep, err := b.client.AppsV1().Deployments(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	y, _ := yaml.Marshal(dep)
	c.DeploymentYAML = string(y)
	c.DescribeData, _ = json.Marshal(map[string]any{
		"conditions": dep.Status.Conditions,
		"replicas":   dep.Status.Replicas,
		"ready":      dep.Status.ReadyReplicas,
	})
	c.Events, _ = json.Marshal(b.fetchEvents(ctx, r.Namespace, r.Name))
	rss, _ := b.client.AppsV1().ReplicaSets(r.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(dep.Spec.Selector),
	})
	if rss != nil && len(rss.Items) > 0 {
		c.ReplicaSetInfo, _ = json.Marshal(rss.Items[0].Status)
	}
	pods, _ := b.client.CoreV1().Pods(r.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(dep.Spec.Selector),
	})
	if pods != nil && len(pods.Items) > 0 {
		p := pods.Items[0]
		c.RestartCount = totalRestarts(&p)
		c.Logs, _ = json.Marshal(b.fetchPodLogs(ctx, r.Namespace, p.Name))
		c.ImageDetails, _ = json.Marshal(imagesFromPod(&p))
	}
	return nil
}

func (b *Builder) collectGeneric(ctx context.Context, r *domain.ClusterResource, c *domain.IncidentContext) error {
	c.DescribeData = r.StatusSnapshot
	ns := r.Namespace
	if ns == "" {
		ns = "default"
	}
	c.Events, _ = json.Marshal(b.fetchEvents(ctx, ns, r.Name))
	return nil
}

func (b *Builder) fetchEvents(ctx context.Context, namespace, name string) []map[string]string {
	field := fmt.Sprintf("involvedObject.name=%s", name)
	list, err := b.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: field,
	})
	if err != nil {
		return nil
	}
	out := make([]map[string]string, 0, len(list.Items))
	for _, e := range list.Items {
		out = append(out, map[string]string{
			"type": e.Type, "reason": e.Reason, "message": e.Message,
			"count": fmt.Sprintf("%d", e.Count), "last": e.LastTimestamp.String(),
		})
	}
	return out
}

func (b *Builder) fetchPodLogs(ctx context.Context, namespace, pod string) []map[string]string {
	opts := &corev1.PodLogOptions{TailLines: int64Ptr(100)}
	req := b.client.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return []map[string]string{{"error": err.Error()}}
	}
	defer stream.Close()
	data, _ := io.ReadAll(stream)
	lines := strings.Split(string(data), "\n")
	out := make([]map[string]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, map[string]string{"line": line, "ts": time.Now().UTC().Format(time.RFC3339)})
	}
	if len(out) > 50 {
		out = out[len(out)-50:]
	}
	return out
}

func int64Ptr(n int64) *int64 { return &n }

func totalRestarts(pod *corev1.Pod) int {
	n := 0
	for _, cs := range pod.Status.ContainerStatuses {
		n += int(cs.RestartCount)
	}
	return n
}

func imagesFromPod(pod *corev1.Pod) []map[string]string {
	var out []map[string]string
	for _, c := range pod.Spec.Containers {
		out = append(out, map[string]string{"container": c.Name, "image": c.Image})
	}
	return out
}

func envFromPod(pod *corev1.Pod) []map[string]string {
	var out []map[string]string
	for _, c := range pod.Spec.Containers {
		for _, e := range c.Env {
			val := e.Value
			if e.ValueFrom != nil {
				val = "(from source)"
			}
			out = append(out, map[string]string{"container": c.Name, "name": e.Name, "value": val})
		}
	}
	return out
}

func mountsFromPod(pod *corev1.Pod) []map[string]string {
	var out []map[string]string
	for _, c := range pod.Spec.Containers {
		for _, m := range c.VolumeMounts {
			out = append(out, map[string]string{"container": c.Name, "name": m.Name, "mountPath": m.MountPath})
		}
	}
	return out
}

func describePod(pod *corev1.Pod) map[string]any {
	return map[string]any{
		"phase":      pod.Status.Phase,
		"conditions": pod.Status.Conditions,
		"message":    pod.Status.Message,
		"reason":     pod.Status.Reason,
		"node":       pod.Spec.NodeName,
		"qos":        pod.Status.QOSClass,
	}
}

func nodeSummary(node *corev1.Node) map[string]any {
	return map[string]any{
		"name":       node.Name,
		"conditions": node.Status.Conditions,
		"addresses":  node.Status.Addresses,
		"capacity":   node.Status.Capacity,
	}
}
