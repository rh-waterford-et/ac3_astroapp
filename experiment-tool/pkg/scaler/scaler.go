package scaler

import (
	"context"
	"time"
)

// Scaler defines the interface for scaling operations
type Scaler interface {
	// Scale sets the number of replicas for the target deployment
	Scale(ctx context.Context, replicas int32) error

	// GetCurrentScale returns the current number of replicas
	GetCurrentScale(ctx context.Context) (int32, error)

	// WaitForScale waits until the deployment reaches the desired replica count
	WaitForScale(ctx context.Context, replicas int32, timeout time.Duration) error

	// GetDeploymentInfo returns deployment metadata for validation
	GetDeploymentInfo(ctx context.Context) (*DeploymentInfo, error)
}

// DeploymentInfo contains metadata about the target deployment
type DeploymentInfo struct {
	Name      string
	Namespace string
	Replicas  int32
	Ready     int32
	Available int32
	Labels    map[string]string
}

// ScaleResult contains the result of a scaling operation
type ScaleResult struct {
	PreviousReplicas int32
	NewReplicas      int32
	Duration         time.Duration
	Success          bool
	Error            error
}

// ScalerConfig holds configuration for the scaler
type ScalerConfig struct {
	Namespace      string
	DeploymentName string
	KubeConfig     string // Path to kubeconfig file, empty for in-cluster
	Timeout        time.Duration
}

// ValidationError represents configuration or deployment validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
