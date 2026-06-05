package k8sclient

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func New() (kubernetes.Interface, *rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		client, err := kubernetes.NewForConfig(cfg)
		return client, cfg, err
	}
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		loading.ExplicitPath = kubeconfig
	} else {
		home, _ := os.UserHomeDir()
		loading.ExplicitPath = filepath.Join(home, ".kube", "config")
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("kubeconfig: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	return client, cfg, err
}
