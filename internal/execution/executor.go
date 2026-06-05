package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Executor struct {
	clusterID string
	client    kubernetes.Interface
	store     ports.ExecutionRepository
	rem       ports.RemediationRepository
	approval  ports.ApprovalRepository
	notify    Notifier
}

type Notifier interface {
	Publish(eventType string, payload any)
}

func NewExecutor(clusterID string, client kubernetes.Interface, store ports.ExecutionRepository, rem ports.RemediationRepository, approval ports.ApprovalRepository, notify Notifier) *Executor {
	return &Executor{clusterID: clusterID, client: client, store: store, rem: rem, approval: approval, notify: notify}
}

func (e *Executor) ExecuteApproved(ctx context.Context, recommendationID, approvedBy string) (*domain.ExecutionRecord, error) {
	ok, by, err := e.approval.IsApproved(ctx, recommendationID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("recommendation %s is not approved", recommendationID)
	}
	if approvedBy == "" {
		approvedBy = by
	}
	rec, err := e.rem.GetRecommendation(ctx, recommendationID)
	if err != nil {
		return nil, err
	}
	_ = e.rem.UpdateRecommendationStatus(ctx, recommendationID, domain.ActionExecuting)

	start := time.Now()
	var params map[string]string
	_ = json.Unmarshal(rec.Parameters, &params)

	execErr := e.runAction(ctx, rec.ActionType, params)
	completed := time.Now()
	record := &domain.ExecutionRecord{
		ClusterID:        e.clusterID,
		RecommendationID: recommendationID,
		ApprovedBy:       approvedBy,
		ActionType:       rec.ActionType,
		Parameters:       rec.Parameters,
		Success:          execErr == nil,
		StartedAt:        start,
		CompletedAt:      &completed,
		DurationMs:       completed.Sub(start).Milliseconds(),
	}
	if execErr != nil {
		record.FailureReason = execErr.Error()
	}
	if err := e.store.SaveExecution(ctx, record); err != nil {
		return record, err
	}
	if e.notify != nil {
		e.notify.Publish("execution.completed", map[string]any{"id": record.ID, "success": record.Success})
	}
	return record, nil
}

func (e *Executor) runAction(ctx context.Context, actionType string, p map[string]string) error {
	ns := p["namespace"]
	name := p["name"]
	switch actionType {
	case domain.ActionRestartPod, domain.ActionDeleteFailedPod:
		return e.client.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
	case domain.ActionRestartDeployment:
		dep, err := e.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dep.Spec.Template.Annotations == nil {
			dep.Spec.Template.Annotations = map[string]string{}
		}
		dep.Spec.Template.Annotations["kubeatlas.io/restartedAt"] = time.Now().Format(time.RFC3339)
		_, err = e.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
		return err
	case domain.ActionRollbackDeployment:
		return e.rollbackDeployment(ctx, ns, name)
	case domain.ActionScaleDeployment:
		dep, err := e.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dep.Spec.Replicas == nil {
			r := int32(1)
			dep.Spec.Replicas = &r
		}
		*dep.Spec.Replicas++
		_, err = e.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported action: %s", actionType)
	}
}

func (e *Executor) rollbackDeployment(ctx context.Context, ns, name string) error {
	dep, err := e.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	rsList, err := e.client.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(dep.Spec.Selector),
	})
	if err != nil || len(rsList.Items) < 2 {
		return fmt.Errorf("no previous replicaset to rollback to")
	}
	var prev *appsv1.ReplicaSet
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		if rs.Annotations["deployment.kubernetes.io/revision"] != "" && prev == nil {
			prev = rs
		}
	}
	if prev == nil {
		prev = &rsList.Items[1]
	}
	dep.Spec.Template = prev.Spec.Template
	_, err = e.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (e *Executor) Reject(ctx context.Context, recommendationID string) error {
	return e.rem.UpdateRecommendationStatus(ctx, recommendationID, domain.ActionRejected)
}
