package metrics

import (
	"context"
	"fmt"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sClient wraps the Kubernetes client for querying cluster state
type K8sClient struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewK8sClient creates a new Kubernetes client using in-cluster configuration
func NewK8sClient() (*K8sClient, error) {
	// Use in-cluster config (for pods running inside Kubernetes)
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Get namespace from environment or use default
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "uc3-applications"
	}

	return &K8sClient{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

// GetPodCount returns the number of running pods in the namespace
func (k *K8sClient) GetPodCount(ctx context.Context) (int, error) {
	pods, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list pods: %w", err)
	}

	return len(pods.Items), nil
}

// GetPodCountByLabel returns the number of pods matching a specific label selector
func (k *K8sClient) GetPodCountByLabel(ctx context.Context, labelSelector string) (int, error) {
	pods, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list pods with selector %s: %w", labelSelector, err)
	}

	return len(pods.Items), nil
}

// GetPodCountsByApp returns pod counts for specific application labels
func (k *K8sClient) GetPodCountsByApp(ctx context.Context) (map[string]int, error) {
	appLabels := []string{
		"ucm-producer",
		"ucm-processor",
		"prometheus-server",
		"grafana",
		"redis",
		"rabbitmq",
	}

	counts := make(map[string]int)

	for _, app := range appLabels {
		count, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", app),
		})
		if err != nil {
			log.Printf("Warning: failed to get pod count for app=%s: %v", app, err)
			counts[app] = 0
		} else {
			counts[app] = len(count.Items)
		}
	}

	return counts, nil
}

// GetRunningPodCount returns only the count of pods in Running phase
func (k *K8sClient) GetRunningPodCount(ctx context.Context) (int, error) {
	pods, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list running pods: %w", err)
	}

	return len(pods.Items), nil
}
