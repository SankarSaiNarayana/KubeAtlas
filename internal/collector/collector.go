package collector

import (
	"context"
	"encoding/json"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
	"github.com/kube-dashboard/kube_dashboard/internal/ports"
)

type Notifier interface {
	Publish(eventType string, payload any)
}

type Collector struct {
	clusterID string
	client    kubernetes.Interface
	store     ports.ResourceRepository
	notify    Notifier
	stopCh    chan struct{}
}

func New(clusterID string, client kubernetes.Interface, store ports.ResourceRepository, notify Notifier) *Collector {
	return &Collector{
		clusterID: clusterID,
		client:    client,
		store:     store,
		notify:    notify,
		stopCh:    make(chan struct{}),
	}
}

func (c *Collector) Start(ctx context.Context) error {
	factory := informers.NewSharedInformerFactoryWithOptions(c.client, 0)
	informersList := []cache.SharedIndexInformer{
		factory.Core().V1().Pods().Informer(),
		factory.Apps().V1().Deployments().Informer(),
		factory.Apps().V1().ReplicaSets().Informer(),
		factory.Apps().V1().StatefulSets().Informer(),
		factory.Apps().V1().DaemonSets().Informer(),
		factory.Core().V1().Services().Informer(),
		factory.Networking().V1().Ingresses().Informer(),
		factory.Core().V1().Nodes().Informer(),
		factory.Core().V1().Namespaces().Informer(),
	}
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) { c.process(ctx, obj) },
		UpdateFunc: func(old, newObj any) {
			if sameRV(old, newObj) {
				return
			}
			c.process(ctx, newObj)
		},
		DeleteFunc: func(obj any) { c.handleDelete(obj) },
	}
	for _, inf := range informersList {
		if _, err := inf.AddEventHandler(handler); err != nil {
			return err
		}
	}
	factory.Start(c.stopCh)
	synced := make([]cache.InformerSynced, len(informersList))
	for i, inf := range informersList {
		synced[i] = inf.HasSynced
	}
	if !cache.WaitForCacheSync(ctx.Done(), synced...) {
		return context.Canceled
	}
	log.Printf("collector: watching cluster %s", c.clusterID)
	<-ctx.Done()
	close(c.stopCh)
	return ctx.Err()
}

func (c *Collector) process(ctx context.Context, obj any) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			runtimeObj, _ = tomb.Obj.(runtime.Object)
		}
	}
	if runtimeObj == nil {
		return
	}
	r, err := objectToResource(c.clusterID, runtimeObj)
	if err != nil || r == nil {
		return
	}
	if err := c.store.UpsertResource(ctx, r); err != nil {
		log.Printf("collector upsert: %v", err)
		return
	}
	if c.notify != nil {
		c.notify.Publish("resource.updated", map[string]string{
			"id": r.ID, "kind": r.Kind, "name": r.Name, "namespace": r.Namespace,
		})
	}
}

func (c *Collector) handleDelete(obj any) {
	meta, err := metaAccessor(obj)
	if err != nil {
		return
	}
	ctx := context.Background()
	_ = c.store.MarkResourceDeleted(ctx, c.clusterID, string(meta.GetUID()))
	if c.notify != nil {
		c.notify.Publish("resource.deleted", map[string]string{
			"namespace": meta.GetNamespace(), "name": meta.GetName(),
		})
	}
}

func objectToResource(clusterID string, obj runtime.Object) (*domain.ClusterResource, error) {
	switch o := obj.(type) {
	case *corev1.Pod:
		return fromPod(clusterID, o), nil
	case *appsv1.Deployment:
		return fromMeta(clusterID, o.APIVersion, "Deployment", o.Namespace, o.Name, o.UID, o.Labels, o.Spec, o.Status, "", "", ""), nil
	case *appsv1.ReplicaSet:
		ok, on := ownerOf(o.OwnerReferences)
		return fromMeta(clusterID, o.APIVersion, "ReplicaSet", o.Namespace, o.Name, o.UID, o.Labels, o.Spec, o.Status, "", ok, on), nil
	case *appsv1.StatefulSet:
		return fromMeta(clusterID, o.APIVersion, "StatefulSet", o.Namespace, o.Name, o.UID, o.Labels, o.Spec, o.Status, "", "", ""), nil
	case *appsv1.DaemonSet:
		return fromMeta(clusterID, o.APIVersion, "DaemonSet", o.Namespace, o.Name, o.UID, o.Labels, o.Spec, o.Status, "", "", ""), nil
	case *corev1.Service:
		return fromMeta(clusterID, o.APIVersion, "Service", o.Namespace, o.Name, o.UID, o.Labels, o.Spec, o.Status, "", "", ""), nil
	case *networkingv1.Ingress:
		return fromMeta(clusterID, o.APIVersion, "Ingress", o.Namespace, o.Name, o.UID, o.Labels, o.Spec, o.Status, "", "", ""), nil
	case *corev1.Node:
		return fromMeta(clusterID, o.APIVersion, "Node", "", o.Name, o.UID, o.Labels, o.Spec, o.Status, o.Name, "", ""), nil
	case *corev1.Namespace:
		return fromMeta(clusterID, o.APIVersion, "Namespace", "", o.Name, o.UID, o.Labels, o.Spec, o.Status, "", "", ""), nil
	default:
		return nil, nil
	}
}

func fromPod(clusterID string, p *corev1.Pod) *domain.ClusterResource {
	ok, on := ownerOf(p.OwnerReferences)
	return fromMeta(clusterID, p.APIVersion, "Pod", p.Namespace, p.Name, p.UID, p.Labels, p.Spec, p.Status, p.Spec.NodeName, ok, on)
}

func fromMeta(clusterID, apiVersion, kind, ns, name string, uid types.UID, labels map[string]string, spec, status any, node, ownerK, ownerN string) *domain.ClusterResource {
	lb, _ := json.Marshal(labels)
	sp, _ := json.Marshal(spec)
	st, _ := json.Marshal(status)
	return &domain.ClusterResource{
		ClusterID:      clusterID,
		ResourceUID:    string(uid),
		APIVersion:     apiVersion,
		Kind:           kind,
		Namespace:      ns,
		Name:           name,
		Labels:         lb,
		SpecSnapshot:   sp,
		StatusSnapshot: st,
		NodeName:       node,
		OwnerKind:      ownerK,
		OwnerName:      ownerN,
	}
}

func ownerOf(refs []metav1.OwnerReference) (kind, name string) {
	if len(refs) == 0 {
		return "", ""
	}
	return refs[0].Kind, refs[0].Name
}

func sameRV(old, newObj any) bool {
	o1, ok1 := old.(metav1.Object)
	o2, ok2 := newObj.(metav1.Object)
	if !ok1 || !ok2 {
		return false
	}
	return o1.GetResourceVersion() == o2.GetResourceVersion() && o1.GetResourceVersion() != ""
}

func metaAccessor(obj any) (metav1.Object, error) {
	if t, ok := obj.(metav1.Object); ok {
		return t, nil
	}
	if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if t, ok := tomb.Obj.(metav1.Object); ok {
			return t, nil
		}
	}
	return nil, context.Canceled
}
