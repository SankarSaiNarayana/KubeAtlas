package graph

import (
	"context"
	"fmt"
	"log"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kube-dashboard/kube_dashboard/internal/models"
	"github.com/kube-dashboard/kube_dashboard/internal/store"
)

type Builder struct {
	clusterID string
	clientset *kubernetes.Clientset
	store     *store.Store
}

func NewBuilder(clusterID, kubeconfig string, st *store.Store) (*Builder, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &Builder{clusterID: clusterID, clientset: clientset, store: st}, nil
}

func (b *Builder) Run(ctx context.Context) error {
	factory := informers.NewSharedInformerFactory(b.clientset, 0)

	deployInf := factory.Apps().V1().Deployments().Informer()
	rsInf := factory.Apps().V1().ReplicaSets().Informer()
	dsInf := factory.Apps().V1().DaemonSets().Informer()
	ssInf := factory.Apps().V1().StatefulSets().Informer()
	svcInf := factory.Core().V1().Services().Informer()
	podInf := factory.Core().V1().Pods().Informer()
	cmInf := factory.Core().V1().ConfigMaps().Informer()
	secretInf := factory.Core().V1().Secrets().Informer()
	nsInf := factory.Core().V1().Namespaces().Informer()
	nodeInf := factory.Core().V1().Nodes().Informer()
	saInf := factory.Core().V1().ServiceAccounts().Informer()
	pvcInf := factory.Core().V1().PersistentVolumeClaims().Informer()
	pvInf := factory.Core().V1().PersistentVolumes().Informer()
	ingInf := factory.Networking().V1().Ingresses().Informer()
	netpolInf := factory.Networking().V1().NetworkPolicies().Informer()
	jobInf := factory.Batch().V1().Jobs().Informer()
	cronInf := factory.Batch().V1().CronJobs().Informer()
	hpaInf := factory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()
	roleInf := factory.Rbac().V1().Roles().Informer()
	rbInf := factory.Rbac().V1().RoleBindings().Informer()
	crInf := factory.Rbac().V1().ClusterRoles().Informer()
	crbInf := factory.Rbac().V1().ClusterRoleBindings().Informer()
	scInf := factory.Storage().V1().StorageClasses().Informer()

	handlers := []struct {
		inf cache.SharedIndexInformer
		fn  func(context.Context, any) error
	}{
		{deployInf, b.onDeployment},
		{rsInf, b.onReplicaSet},
		{dsInf, b.onDaemonSet},
		{ssInf, b.onStatefulSet},
		{svcInf, b.onService},
		{podInf, b.onPod},
		{cmInf, b.onConfigMap},
		{secretInf, b.onSecret},
		{nsInf, b.onNamespace},
		{nodeInf, b.onNode},
		{saInf, b.onServiceAccount},
		{pvcInf, b.onPVC},
		{pvInf, b.onPV},
		{ingInf, b.onIngress},
		{netpolInf, b.onNetworkPolicy},
		{jobInf, b.onJob},
		{cronInf, b.onCronJob},
		{hpaInf, b.onHPA},
		{roleInf, b.onRole},
		{rbInf, b.onRoleBinding},
		{crInf, b.onClusterRole},
		{crbInf, b.onClusterRoleBinding},
		{scInf, b.onStorageClass},
	}

	for _, h := range handlers {
		if _, err := h.inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj any) { _ = h.fn(ctx, obj) },
			UpdateFunc: func(_, obj any) { _ = h.fn(ctx, obj) },
		}); err != nil {
			return err
		}
	}

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	log.Printf("graph builder running for cluster %s", b.clusterID)
	<-ctx.Done()
	return ctx.Err()
}

func (b *Builder) onDeployment(ctx context.Context, obj any) error {
	d, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil
	}
	node := models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(d.UID),
		APIVersion:  "apps/v1",
		Kind:        "Deployment",
		Namespace:   d.Namespace,
		Name:        d.Name,
		Labels:      d.Labels,
		Status:      deploymentStatus(d),
	}
	saved, err := b.store.UpsertGraphNode(ctx, node)
	if err != nil {
		return err
	}

	for _, ref := range d.Spec.Template.Spec.Volumes {
		if ref.ConfigMap != nil {
			if err := b.linkByName(ctx, saved, "ConfigMap", d.Namespace, ref.ConfigMap.Name, "mounts"); err != nil {
				log.Printf("link configmap: %v", err)
			}
		}
		if ref.Secret != nil {
			if err := b.linkByName(ctx, saved, "Secret", d.Namespace, ref.Secret.SecretName, "mounts"); err != nil {
				log.Printf("link secret: %v", err)
			}
		}
	}
	return nil
}

func (b *Builder) onService(ctx context.Context, obj any) error {
	s, ok := obj.(*corev1.Service)
	if !ok {
		return nil
	}
	node := models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(s.UID),
		APIVersion:  "v1",
		Kind:        "Service",
		Namespace:   s.Namespace,
		Name:        s.Name,
		Labels:      s.Labels,
		Status:      "active",
	}
	saved, err := b.store.UpsertGraphNode(ctx, node)
	if err != nil {
		return err
	}

	deployments, err := b.clientset.AppsV1().Deployments(s.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, d := range deployments.Items {
		if labelMapMatches(s.Spec.Selector, d.Spec.Template.Labels) {
			if err := b.linkByName(ctx, saved, "Deployment", d.Namespace, d.Name, "selects"); err != nil {
				log.Printf("link deployment: %v", err)
			}
		}
	}
	return nil
}

func (b *Builder) onIngress(ctx context.Context, obj any) error {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil
	}
	node := models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(ing.UID),
		APIVersion:  "networking.k8s.io/v1",
		Kind:        "Ingress",
		Namespace:   ing.Namespace,
		Name:        ing.Name,
		Labels:      ing.Labels,
		Status:      "active",
	}
	saved, err := b.store.UpsertGraphNode(ctx, node)
	if err != nil {
		return err
	}

	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil {
				continue
			}
			svcName := path.Backend.Service.Name
			if err := b.linkByName(ctx, saved, "Service", ing.Namespace, svcName, "exposes"); err != nil {
				log.Printf("link service from ingress: %v", err)
			}
		}
	}
	return nil
}

func (b *Builder) onPod(ctx context.Context, obj any) error {
	p, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}

	status := podStatus(p)
	node := models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(p.UID),
		APIVersion:  "v1",
		Kind:        "Pod",
		Namespace:   p.Namespace,
		Name:        p.Name,
		Labels:      p.Labels,
		Status:      status,
	}
	saved, err := b.store.UpsertGraphNode(ctx, node)
	if err != nil {
		return err
	}

	for _, owner := range p.OwnerReferences {
		if owner.Controller != nil && *owner.Controller {
			if err := b.linkByName(ctx, saved, owner.Kind, p.Namespace, owner.Name, "owned_by"); err != nil {
				log.Printf("link pod owner: %v", err)
			}
		}
	}
	return nil
}

func podStatus(p *corev1.Pod) string {
	if p.Status.Phase == corev1.PodSucceeded {
		return "succeeded"
	}
	if p.Status.Phase == corev1.PodFailed {
		return "failed"
	}
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			switch cs.State.Waiting.Reason {
			case "CrashLoopBackOff":
				return "crashloop"
			case "ImagePullBackOff", "ErrImagePull", "ImageInspectError":
				return "image_pull_backoff"
			default:
				return strings.ToLower(cs.State.Waiting.Reason)
			}
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return "error"
		}
	}
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
			return "not_ready"
		}
	}
	if p.Status.Phase == corev1.PodPending {
		return "pending"
	}
	if p.Status.Phase == corev1.PodRunning {
		return "ready"
	}
	return strings.ToLower(string(p.Status.Phase))
}

func podReason(p *corev1.Pod) string {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			return cs.State.Terminated.Reason
		}
	}
	return string(p.Status.Phase)
}

// --- Additional resource kinds (store as nodes) ---

func (b *Builder) onNamespace(ctx context.Context, obj any) error {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(ns.UID),
		APIVersion:  "v1",
		Kind:        "Namespace",
		Namespace:   "",
		Name:        ns.Name,
		Labels:      ns.Labels,
		Status:      string(ns.Status.Phase),
	})
	return err
}

func (b *Builder) onNode(ctx context.Context, obj any) error {
	n, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}
	status := "unknown"
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				status = "ready"
			} else {
				status = "not_ready"
			}
		}
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(n.UID),
		APIVersion:  "v1",
		Kind:        "Node",
		Namespace:   "",
		Name:        n.Name,
		Labels:      n.Labels,
		Status:      status,
	})
	return err
}

func (b *Builder) onConfigMap(ctx context.Context, obj any) error {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(cm.UID),
		APIVersion:  "v1",
		Kind:        "ConfigMap",
		Namespace:   cm.Namespace,
		Name:        cm.Name,
		Labels:      cm.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onSecret(ctx context.Context, obj any) error {
	s, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(s.UID),
		APIVersion:  "v1",
		Kind:        "Secret",
		Namespace:   s.Namespace,
		Name:        s.Name,
		Labels:      s.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onServiceAccount(ctx context.Context, obj any) error {
	sa, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(sa.UID),
		APIVersion:  "v1",
		Kind:        "ServiceAccount",
		Namespace:   sa.Namespace,
		Name:        sa.Name,
		Labels:      sa.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onPVC(ctx context.Context, obj any) error {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil
	}
	phase := ""
	if pvc.Status.Phase != "" {
		phase = string(pvc.Status.Phase)
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(pvc.UID),
		APIVersion:  "v1",
		Kind:        "PersistentVolumeClaim",
		Namespace:   pvc.Namespace,
		Name:        pvc.Name,
		Labels:      pvc.Labels,
		Status:      phase,
	})
	return err
}

func (b *Builder) onPV(ctx context.Context, obj any) error {
	pv, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return nil
	}
	phase := ""
	if pv.Status.Phase != "" {
		phase = string(pv.Status.Phase)
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(pv.UID),
		APIVersion:  "v1",
		Kind:        "PersistentVolume",
		Namespace:   "",
		Name:        pv.Name,
		Labels:      pv.Labels,
		Status:      phase,
	})
	return err
}

func (b *Builder) onReplicaSet(ctx context.Context, obj any) error {
	rs, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(rs.UID),
		APIVersion:  "apps/v1",
		Kind:        "ReplicaSet",
		Namespace:   rs.Namespace,
		Name:        rs.Name,
		Labels:      rs.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onDaemonSet(ctx context.Context, obj any) error {
	ds, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(ds.UID),
		APIVersion:  "apps/v1",
		Kind:        "DaemonSet",
		Namespace:   ds.Namespace,
		Name:        ds.Name,
		Labels:      ds.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onStatefulSet(ctx context.Context, obj any) error {
	ss, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(ss.UID),
		APIVersion:  "apps/v1",
		Kind:        "StatefulSet",
		Namespace:   ss.Namespace,
		Name:        ss.Name,
		Labels:      ss.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onNetworkPolicy(ctx context.Context, obj any) error {
	np, ok := obj.(*networkingv1.NetworkPolicy)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(np.UID),
		APIVersion:  "networking.k8s.io/v1",
		Kind:        "NetworkPolicy",
		Namespace:   np.Namespace,
		Name:        np.Name,
		Labels:      np.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onJob(ctx context.Context, obj any) error {
	j, ok := obj.(*batchv1.Job)
	if !ok {
		return nil
	}
	status := "active"
	if j.Status.Succeeded > 0 {
		status = "succeeded"
	} else if j.Status.Failed > 0 {
		status = "failed"
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(j.UID),
		APIVersion:  "batch/v1",
		Kind:        "Job",
		Namespace:   j.Namespace,
		Name:        j.Name,
		Labels:      j.Labels,
		Status:      status,
	})
	return err
}

func (b *Builder) onCronJob(ctx context.Context, obj any) error {
	cj, ok := obj.(*batchv1.CronJob)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(cj.UID),
		APIVersion:  "batch/v1",
		Kind:        "CronJob",
		Namespace:   cj.Namespace,
		Name:        cj.Name,
		Labels:      cj.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onHPA(ctx context.Context, obj any) error {
	h, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(h.UID),
		APIVersion:  "autoscaling/v2",
		Kind:        "HorizontalPodAutoscaler",
		Namespace:   h.Namespace,
		Name:        h.Name,
		Labels:      h.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onRole(ctx context.Context, obj any) error {
	role, ok := obj.(*rbacv1.Role)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(role.UID),
		APIVersion:  "rbac.authorization.k8s.io/v1",
		Kind:        "Role",
		Namespace:   role.Namespace,
		Name:        role.Name,
		Labels:      role.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onRoleBinding(ctx context.Context, obj any) error {
	rb, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(rb.UID),
		APIVersion:  "rbac.authorization.k8s.io/v1",
		Kind:        "RoleBinding",
		Namespace:   rb.Namespace,
		Name:        rb.Name,
		Labels:      rb.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onClusterRole(ctx context.Context, obj any) error {
	cr, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(cr.UID),
		APIVersion:  "rbac.authorization.k8s.io/v1",
		Kind:        "ClusterRole",
		Namespace:   "",
		Name:        cr.Name,
		Labels:      cr.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onClusterRoleBinding(ctx context.Context, obj any) error {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(crb.UID),
		APIVersion:  "rbac.authorization.k8s.io/v1",
		Kind:        "ClusterRoleBinding",
		Namespace:   "",
		Name:        crb.Name,
		Labels:      crb.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) onStorageClass(ctx context.Context, obj any) error {
	sc, ok := obj.(*storagev1.StorageClass)
	if !ok {
		return nil
	}
	_, err := b.store.UpsertGraphNode(ctx, models.GraphNode{
		ClusterID:   b.clusterID,
		ResourceUID: string(sc.UID),
		APIVersion:  "storage.k8s.io/v1",
		Kind:        "StorageClass",
		Namespace:   "",
		Name:        sc.Name,
		Labels:      sc.Labels,
		Status:      "active",
	})
	return err
}

func (b *Builder) linkByName(ctx context.Context, source models.GraphNode, kind, namespace, name, edgeType string) error {
	target := models.GraphNode{
		ClusterID:  b.clusterID,
		APIVersion: apiVersionForKind(kind),
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		Labels:     map[string]string{},
		Status:     "referenced",
	}
	savedTarget, err := b.store.UpsertGraphNode(ctx, target)
	if err != nil {
		return err
	}
	return b.store.UpsertGraphEdge(ctx, models.GraphEdge{
		ClusterID: b.clusterID,
		SourceID:  source.ID,
		TargetID:  savedTarget.ID,
		EdgeType:  edgeType,
	})
}

func labelMapMatches(selector map[string]string, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func deploymentStatus(d *appsv1.Deployment) string {
	if d.Status.ReadyReplicas >= d.Status.Replicas && d.Status.Replicas > 0 {
		return "ready"
	}
	if d.Status.Replicas == 0 {
		return "scaled_down"
	}
	return "not_ready"
}

func apiVersionForKind(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return "apps/v1"
	case "Ingress":
		return "networking.k8s.io/v1"
	default:
		return "v1"
	}
}
