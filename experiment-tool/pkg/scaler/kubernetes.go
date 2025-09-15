package scaler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// KubernetesScaler implements the Scaler interface using Kubernetes client-go
type KubernetesScaler struct {
	client     kubernetes.Interface
	config     ScalerConfig
	restConfig *rest.Config
}

// NewKubernetesScaler creates a new Kubernetes scaler instance
func NewKubernetesScaler(config ScalerConfig) (*KubernetesScaler, error) {
	// Validate configuration
	if err := validateScalerConfig(config); err != nil {
		return nil, err
	}

	// Build Kubernetes client configuration
	restConfig, err := buildKubeConfig(config.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &KubernetesScaler{
		client:     clientset,
		config:     config,
		restConfig: restConfig,
	}, nil
}

// Scale sets the number of replicas for the target deployment
func (k *KubernetesScaler) Scale(ctx context.Context, replicas int32) error {
	if replicas < 0 {
		return &ValidationError{Field: "replicas", Message: "cannot be negative"}
	}

	deploymentsClient := k.client.AppsV1().Deployments(k.config.Namespace)

	// Get current deployment
	deployment, err := deploymentsClient.Get(ctx, k.config.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", k.config.Namespace, k.config.DeploymentName, err)
	}

	// Update replica count
	deployment.Spec.Replicas = &replicas

	// Apply the update
	_, err = deploymentsClient.Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale deployment %s/%s to %d replicas: %w",
			k.config.Namespace, k.config.DeploymentName, replicas, err)
	}

	return nil
}

// GetCurrentScale returns the current number of replicas
func (k *KubernetesScaler) GetCurrentScale(ctx context.Context) (int32, error) {
	deploymentsClient := k.client.AppsV1().Deployments(k.config.Namespace)

	deployment, err := deploymentsClient.Get(ctx, k.config.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get deployment %s/%s: %w", k.config.Namespace, k.config.DeploymentName, err)
	}

	if deployment.Spec.Replicas == nil {
		return 0, nil
	}

	return *deployment.Spec.Replicas, nil
}

// WaitForScale waits until the deployment reaches the desired replica count
func (k *KubernetesScaler) WaitForScale(ctx context.Context, replicas int32, timeout time.Duration) error {
	deploymentsClient := k.client.AppsV1().Deployments(k.config.Namespace)

	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(timeoutCtx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := deploymentsClient.Get(ctx, k.config.DeploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if desired replicas match ready replicas
		return deployment.Status.ReadyReplicas == replicas, nil
	})
}

// GetDeploymentInfo returns deployment metadata for validation
func (k *KubernetesScaler) GetDeploymentInfo(ctx context.Context) (*DeploymentInfo, error) {
	deploymentsClient := k.client.AppsV1().Deployments(k.config.Namespace)

	deployment, err := deploymentsClient.Get(ctx, k.config.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", k.config.Namespace, k.config.DeploymentName, err)
	}

	replicas := int32(0)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	return &DeploymentInfo{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
		Replicas:  replicas,
		Ready:     deployment.Status.ReadyReplicas,
		Available: deployment.Status.AvailableReplicas,
		Labels:    deployment.Labels,
	}, nil
}

// ValidateClusterConnection checks if we can connect to the cluster and provides user-friendly error messages
func (k *KubernetesScaler) ValidateClusterConnection(ctx context.Context) error {
	// Try to get server version as a connectivity test
	discoveryClient := k.client.Discovery()
	_, err := discoveryClient.ServerVersion()
	if err != nil {
		return &ValidationError{
			Field:   "cluster_connection",
			Message: formatConnectionError(err),
		}
	}
	return nil
}

// formatConnectionError provides user-friendly error messages for common connection issues
func formatConnectionError(err error) string {
	errStr := err.Error()

	// Common OpenShift/Kubernetes connection errors
	if strings.Contains(errStr, "connection refused") {
		return "cannot connect to cluster - ensure you're logged in with 'oc login' or 'kubectl'"
	}
	if strings.Contains(errStr, "no such host") {
		return "cluster endpoint not found - check your cluster connection"
	}
	if strings.Contains(errStr, "certificate") || strings.Contains(errStr, "x509") {
		return "certificate validation failed - try 'oc login' to refresh your credentials"
	}
	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "forbidden") {
		return "authentication failed - run 'oc login' to authenticate with your cluster"
	}
	if strings.Contains(errStr, "token") {
		return "authentication token expired - run 'oc login' to refresh your session"
	}

	// Generic fallback
	return fmt.Sprintf("cluster connection failed: %s", errStr)
}

// buildKubeConfig builds Kubernetes client configuration
func buildKubeConfig(kubeConfigPath string) (*rest.Config, error) {
	// If kubeConfigPath is provided, use it
	if kubeConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}

	// Try in-cluster config first
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Fall back to default kubeconfig location
	if home := homedir.HomeDir(); home != "" {
		kubeConfigPath = filepath.Join(home, ".kube", "config")
		if config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath); err == nil {
			return config, nil
		}
	}

	return nil, fmt.Errorf("unable to build kubernetes config: no kubeconfig found")
}

// validateScalerConfig validates the scaler configuration
func validateScalerConfig(config ScalerConfig) error {
	if config.Namespace == "" {
		return &ValidationError{Field: "namespace", Message: "cannot be empty"}
	}
	if config.DeploymentName == "" {
		return &ValidationError{Field: "deployment_name", Message: "cannot be empty"}
	}
	if config.Timeout <= 0 {
		return &ValidationError{Field: "timeout", Message: "must be positive"}
	}
	return nil
}

// ScaleWithResult performs scaling and returns detailed results
func (k *KubernetesScaler) ScaleWithResult(ctx context.Context, replicas int32) *ScaleResult {
	start := time.Now()

	// Get current scale
	previousReplicas, err := k.GetCurrentScale(ctx)
	if err != nil {
		return &ScaleResult{
			PreviousReplicas: 0,
			NewReplicas:      replicas,
			Duration:         time.Since(start),
			Success:          false,
			Error:            err,
		}
	}

	// Perform scaling
	err = k.Scale(ctx, replicas)
	if err != nil {
		return &ScaleResult{
			PreviousReplicas: previousReplicas,
			NewReplicas:      replicas,
			Duration:         time.Since(start),
			Success:          false,
			Error:            err,
		}
	}

	return &ScaleResult{
		PreviousReplicas: previousReplicas,
		NewReplicas:      replicas,
		Duration:         time.Since(start),
		Success:          true,
		Error:            nil,
	}
}
