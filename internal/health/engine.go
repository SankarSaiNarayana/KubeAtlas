package health

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Evaluator interface {
	Evaluate(resource *domain.ClusterResource) *domain.ResourceHealth
}

type Engine struct {
	clusterID string
	store     ports.HealthRepository
	resources ports.ResourceRepository
	notify    Notifier
}

type Notifier interface {
	Publish(eventType string, payload any)
}

type Transition struct {
	Resource   *domain.ClusterResource
	Previous   domain.HealthState
	Current    domain.HealthState
	Health     *domain.ResourceHealth
}

type TransitionHandler func(ctx context.Context, t Transition) error

func NewEngine(clusterID string, store ports.HealthRepository, resources ports.ResourceRepository, notify Notifier) *Engine {
	return &Engine{clusterID: clusterID, store: store, resources: resources, notify: notify}
}

func (e *Engine) EvaluateResource(ctx context.Context, resource *domain.ClusterResource, onTransition TransitionHandler) error {
	h := Evaluate(resource)
	h.ClusterID = e.clusterID
	h.ResourceID = resource.ID
	prev, err := e.store.UpsertHealth(ctx, h)
	if err != nil {
		return err
	}
	if onTransition != nil && prev != h.Health {
		if err := onTransition(ctx, Transition{Resource: resource, Previous: prev, Current: h.Health, Health: h}); err != nil {
			return err
		}
	}
	if e.notify != nil {
		e.notify.Publish("health.updated", map[string]any{
			"resource_id": resource.ID, "health": h.Health, "reason": h.Reason,
		})
	}
	return nil
}

func Evaluate(r *domain.ClusterResource) *domain.ResourceHealth {
	switch r.Kind {
	case "Pod":
		return evaluatePod(r)
	case "Deployment":
		return evaluateDeployment(r)
	case "Node":
		return evaluateNode(r)
	case "ReplicaSet":
		return evaluateReplicaSet(r)
	case "StatefulSet", "DaemonSet":
		return evaluateWorkload(r)
	default:
		return healthy(r, "no health rules for kind")
	}
}

func evaluatePod(r *domain.ClusterResource) *domain.ResourceHealth {
	var status corev1.PodStatus
	_ = json.Unmarshal(r.StatusSnapshot, &status)
	details := map[string]any{"phase": status.Phase}
	for _, cs := range status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			switch reason {
			case "CrashLoopBackOff", "Error", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError":
				return critical(r, fmt.Sprintf("container %s: %s", cs.Name, reason), details)
			case "ContainerCreating", "PodInitializing":
				return warning(r, fmt.Sprintf("container %s: %s", cs.Name, reason), details)
			}
		}
	}
	if status.Phase == corev1.PodPending {
		return warning(r, "pod pending", details)
	}
	for _, cond := range status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionFalse && status.Phase == corev1.PodRunning {
			return warning(r, "pod not ready: "+cond.Message, details)
		}
	}
	if status.Phase == corev1.PodFailed {
		return critical(r, "pod failed", details)
	}
	return healthy(r, "pod running")
}

func evaluateDeployment(r *domain.ClusterResource) *domain.ResourceHealth {
	var status appsv1.DeploymentStatus
	_ = json.Unmarshal(r.StatusSnapshot, &status)
	var spec appsv1.DeploymentSpec
	_ = json.Unmarshal(r.SpecSnapshot, &spec)
	details := map[string]any{
		"replicas":          status.Replicas,
		"ready_replicas":    status.ReadyReplicas,
		"updated_replicas":  status.UpdatedReplicas,
		"available_replicas": status.AvailableReplicas,
		"desired":           deref32(spec.Replicas),
	}
	desired := deref32(spec.Replicas)
	if desired > 0 && status.ReadyReplicas != desired {
		return critical(r, fmt.Sprintf("replica mismatch: ready=%d desired=%d", status.ReadyReplicas, desired), details)
	}
	if status.UnavailableReplicas > 0 {
		return warning(r, fmt.Sprintf("%d unavailable replicas", status.UnavailableReplicas), details)
	}
	return healthy(r, "deployment healthy")
}

func evaluateReplicaSet(r *domain.ClusterResource) *domain.ResourceHealth {
	var status appsv1.ReplicaSetStatus
	_ = json.Unmarshal(r.StatusSnapshot, &status)
	var spec appsv1.ReplicaSetSpec
	_ = json.Unmarshal(r.SpecSnapshot, &spec)
	desired := deref32(spec.Replicas)
	if desired > 0 && status.ReadyReplicas != desired {
		return critical(r, fmt.Sprintf("replicaset ready mismatch: %d/%d", status.ReadyReplicas, desired), nil)
	}
	return healthy(r, "replicaset healthy")
}

func evaluateWorkload(r *domain.ClusterResource) *domain.ResourceHealth {
	var status struct {
		Replicas        int32 `json:"replicas"`
		ReadyReplicas   int32 `json:"readyReplicas"`
		AvailableReplicas int32 `json:"availableReplicas"`
	}
	_ = json.Unmarshal(r.StatusSnapshot, &status)
	if status.Replicas > 0 && status.ReadyReplicas < status.Replicas {
		return critical(r, fmt.Sprintf("ready %d/%d", status.ReadyReplicas, status.Replicas), nil)
	}
	return healthy(r, "workload healthy")
}

func evaluateNode(r *domain.ClusterResource) *domain.ResourceHealth {
	var status corev1.NodeStatus
	_ = json.Unmarshal(r.StatusSnapshot, &status)
	for _, cond := range status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
			return critical(r, "node not ready: "+cond.Reason, map[string]any{"message": cond.Message})
		}
		if cond.Type == corev1.NodeDiskPressure && cond.Status == corev1.ConditionTrue {
			return warning(r, "disk pressure", nil)
		}
		if cond.Type == corev1.NodeMemoryPressure && cond.Status == corev1.ConditionTrue {
			return warning(r, "memory pressure", nil)
		}
	}
	return healthy(r, "node ready")
}

func healthy(r *domain.ClusterResource, reason string) *domain.ResourceHealth {
	return &domain.ResourceHealth{Health: domain.HealthHealthy, Reason: reason, Details: detailsJSON(map[string]any{"kind": r.Kind})}
}

func warning(r *domain.ClusterResource, reason string, d map[string]any) *domain.ResourceHealth {
	if d == nil {
		d = map[string]any{}
	}
	d["kind"] = r.Kind
	return &domain.ResourceHealth{Health: domain.HealthWarning, Reason: reason, Details: detailsJSON(d)}
}

func critical(r *domain.ClusterResource, reason string, d map[string]any) *domain.ResourceHealth {
	if d == nil {
		d = map[string]any{}
	}
	d["kind"] = r.Kind
	return &domain.ResourceHealth{Health: domain.HealthCritical, Reason: reason, Details: detailsJSON(d)}
}

func detailsJSON(d map[string]any) json.RawMessage {
	b, _ := json.Marshal(d)
	return b
}

func deref32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
