package ingest

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kube-dashboard/kube_dashboard/internal/config"
	"github.com/kube-dashboard/kube_dashboard/internal/models"
	"github.com/kube-dashboard/kube_dashboard/internal/store"
)

// Watcher records cluster mutations via Kubernetes informers (dev-friendly; no audit log required).
type Watcher struct {
	clusterID string
	clientset *kubernetes.Clientset
	store     *store.Store
}

func NewWatcher(clusterID, kubeconfig string, st *store.Store) (*Watcher, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}
	return &Watcher{clusterID: clusterID, clientset: clientset, store: st}, nil
}

func (w *Watcher) Run(ctx context.Context) error {
	// Important: resync causes periodic Update events for *all* objects, which will spam the
	// change timeline. For Phase 1 we only want real watch-driven deltas.
	factory := informers.NewSharedInformerFactory(w.clientset, 0)

	handlers := []struct {
		inf     cache.SharedIndexInformer
		kind    string
		verbAdd func(any) (models.ChangeEventInput, bool)
	}{
		{factory.Apps().V1().Deployments().Informer(), "Deployment", w.fromDeployment},
		{factory.Apps().V1().StatefulSets().Informer(), "StatefulSet", w.fromStatefulSet},
		{factory.Apps().V1().DaemonSets().Informer(), "DaemonSet", w.fromDaemonSet},
		{factory.Core().V1().Services().Informer(), "Service", w.fromService},
		{factory.Core().V1().ConfigMaps().Informer(), "ConfigMap", w.fromConfigMap},
		{factory.Networking().V1().Ingresses().Informer(), "Ingress", w.fromIngress},
		{factory.Rbac().V1().Roles().Informer(), "Role", w.fromRole},
		{factory.Rbac().V1().RoleBindings().Informer(), "RoleBinding", w.fromRoleBinding},
	}

	for _, h := range handlers {
		kind := h.kind
		parse := h.verbAdd
		if _, err := h.inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				w.record(ctx, "create", kind, obj, parse)
			},
			UpdateFunc: func(oldObj, newObj any) {
				// Avoid duplicates when the informer sends update callbacks without a real change.
				if sameResourceVersion(oldObj, newObj) {
					return
				}
				w.record(ctx, "update", kind, newObj, parse)
			},
			DeleteFunc: func(obj any) {
				w.record(ctx, "delete", kind, obj, parse)
			},
		}); err != nil {
			return err
		}
	}

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	log.Printf("ingest: cluster watch active for cluster %s", w.clusterID)
	<-ctx.Done()
	return ctx.Err()
}

func sameResourceVersion(oldObj, newObj any) bool {
	om, ok1 := oldObj.(metav1.Object)
	nm, ok2 := newObj.(metav1.Object)
	if !ok1 || !ok2 {
		return false
	}
	return om.GetResourceVersion() != "" && om.GetResourceVersion() == nm.GetResourceVersion()
}

func (w *Watcher) record(ctx context.Context, verb, kind string, obj any, parse func(any) (models.ChangeEventInput, bool)) {
	in, ok := parse(obj)
	if !ok {
		return
	}
	in.ClusterID = w.clusterID
	in.Verb = verb
	in.Kind = kind
	in.Source = "cluster-watch"
	if in.Actor == "" {
		in.Actor = "unknown"
	}
	if in.DiffSummary == "" {
		in.DiffSummary = fmt.Sprintf("%s %s/%s", verb, kind, in.Name)
	}
	now := time.Now().UTC()
	in.OccurredAt = &now

	if _, err := w.store.InsertChangeEvent(ctx, in); err != nil {
		log.Printf("ingest: record change: %v", err)
	}
}

func actorFromMeta(obj metav1.Object) string {
	for _, mf := range obj.GetManagedFields() {
		if mf.Manager != "" && !strings.HasPrefix(mf.Manager, "kube-") {
			return mf.Manager
		}
	}
	for _, mf := range obj.GetManagedFields() {
		if mf.Manager != "" {
			return mf.Manager
		}
	}
	return "kubernetes"
}

func (w *Watcher) fromDeployment(obj any) (models.ChangeEventInput, bool) {
	d, ok := obj.(*appsv1.Deployment)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion:  "apps/v1",
		Namespace:   d.Namespace,
		Name:        d.Name,
		ResourceUID: string(d.UID),
		Actor:       actorFromMeta(d),
		DiffSummary: fmt.Sprintf("deployment %s replicas=%d", d.Name, d.Spec.Replicas),
	}, true
}

func (w *Watcher) fromStatefulSet(obj any) (models.ChangeEventInput, bool) {
	s, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion: "apps/v1", Namespace: s.Namespace, Name: s.Name,
		ResourceUID: string(s.UID), Actor: actorFromMeta(s),
		DiffSummary: fmt.Sprintf("statefulset %s", s.Name),
	}, true
}

func (w *Watcher) fromDaemonSet(obj any) (models.ChangeEventInput, bool) {
	d, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion: "apps/v1", Namespace: d.Namespace, Name: d.Name,
		ResourceUID: string(d.UID), Actor: actorFromMeta(d),
		DiffSummary: fmt.Sprintf("daemonset %s", d.Name),
	}, true
}

func (w *Watcher) fromService(obj any) (models.ChangeEventInput, bool) {
	s, ok := obj.(*corev1.Service)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion: "v1", Namespace: s.Namespace, Name: s.Name,
		ResourceUID: string(s.UID), Actor: actorFromMeta(s),
		DiffSummary: fmt.Sprintf("service %s type=%s", s.Name, s.Spec.Type),
	}, true
}

func (w *Watcher) fromConfigMap(obj any) (models.ChangeEventInput, bool) {
	c, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion: "v1", Namespace: c.Namespace, Name: c.Name,
		ResourceUID: string(c.UID), Actor: actorFromMeta(c),
		DiffSummary: fmt.Sprintf("configmap %s keys=%d", c.Name, len(c.Data)),
	}, true
}

func (w *Watcher) fromIngress(obj any) (models.ChangeEventInput, bool) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	hosts := 0
	for _, r := range ing.Spec.Rules {
		if r.Host != "" {
			hosts++
		}
	}
	return models.ChangeEventInput{
		APIVersion: "networking.k8s.io/v1", Namespace: ing.Namespace, Name: ing.Name,
		ResourceUID: string(ing.UID), Actor: actorFromMeta(ing),
		DiffSummary: fmt.Sprintf("ingress %s hosts=%d", ing.Name, hosts),
	}, true
}

func (w *Watcher) fromRole(obj any) (models.ChangeEventInput, bool) {
	r, ok := obj.(*rbacv1.Role)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion: "rbac.authorization.k8s.io/v1", Namespace: r.Namespace, Name: r.Name,
		ResourceUID: string(r.UID), Actor: actorFromMeta(r),
		DiffSummary: fmt.Sprintf("role %s rules=%d", r.Name, len(r.Rules)),
	}, true
}

func (w *Watcher) fromRoleBinding(obj any) (models.ChangeEventInput, bool) {
	rb, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		return models.ChangeEventInput{}, false
	}
	return models.ChangeEventInput{
		APIVersion: "rbac.authorization.k8s.io/v1", Namespace: rb.Namespace, Name: rb.Name,
		ResourceUID: string(rb.UID), Actor: actorFromMeta(rb),
		DiffSummary: fmt.Sprintf("rolebinding %s subjects=%d", rb.Name, len(rb.Subjects)),
	}, true
}

func kubeconfigPath() string {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return home + "/.kube/config"
}

func RunIngest(ctx context.Context, cfg config.Config, st *store.Store) error {
	if path := os.Getenv("AUDIT_LOG_PATH"); path != "" {
		svc := NewService(cfg, st)
		return svc.Run(ctx)
	}

	log.Println("ingest: AUDIT_LOG_PATH not set — starting cluster watch mode")
	w, err := NewWatcher(cfg.ClusterID, kubeconfigPath(), st)
	if err != nil {
		return err
	}
	return w.Run(ctx)
}
