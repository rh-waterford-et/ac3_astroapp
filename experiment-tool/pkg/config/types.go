package config

import "time"

// ExperimentConfig holds all configuration for autoscaling experiments
type ExperimentConfig struct {
	// Experiment Parameters
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`

	// Scaling Configuration
	Scaling ScalingConfig `mapstructure:"scaling"`

	// Workload Configuration
	Workload WorkloadConfig `mapstructure:"workload"`

	// Data Collection
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Infrastructure
	Infrastructure InfrastructureConfig `mapstructure:"infrastructure"`
}

// ScalingConfig defines processor scaling parameters
type ScalingConfig struct {
	ProcessorCount int           `mapstructure:"processor_count"` // Number of processor pods to run
	StabilizeTime  time.Duration `mapstructure:"stabilize_time"`  // Time to wait after scaling before triggering processing
	EnableScaling  bool          `mapstructure:"enable_scaling"`  // Enable/disable scaling (set false when using HPA)
}

// ShouldScale returns whether the experiment tool should perform scaling operations
// Defaults to true for backward compatibility (set in loader.go)
func (s *ScalingConfig) ShouldScale() bool {
	return s.EnableScaling
}

// DatasetConfig defines a single dataset configuration
type DatasetConfig struct {
	Name          string `mapstructure:"name" yaml:"name"`
	ProcessorType string `mapstructure:"processor_type" yaml:"processor_type"`
}

// WorkloadConfig defines job submission patterns
// Supports both single dataset (backward compatible) and multi-dataset modes
type WorkloadConfig struct {
	// Single Dataset Mode (backward compatible)
	Dataset       string `mapstructure:"dataset"`
	ProcessorType string `mapstructure:"processor_type"`

	// Multi-Dataset Mode (new)
	Datasets        []DatasetConfig `mapstructure:"datasets"`
	FailureStrategy string          `mapstructure:"failure_strategy"` // "continue" | "abort_all"

	// Dataset Staggering (for multi-dataset mode)
	DatasetStartInterval time.Duration `mapstructure:"dataset_start_interval"` // Delay between starting each dataset (prevents overwhelming producer pod)
}

// IsMultiDataset returns true if this is a multi-dataset configuration
func (w *WorkloadConfig) IsMultiDataset() bool {
	return len(w.Datasets) > 0
}

// GetDatasets returns the datasets to process, handling both single and multi-dataset modes
func (w *WorkloadConfig) GetDatasets() []DatasetConfig {
	if w.IsMultiDataset() {
		return w.Datasets
	}

	// Backward compatibility: convert single dataset to multi-dataset format
	if w.Dataset != "" {
		return []DatasetConfig{
			{
				Name:          w.Dataset,
				ProcessorType: w.ProcessorType,
			},
		}
	}

	return []DatasetConfig{}
}

// GetFailureStrategy returns the failure strategy with a sensible default
func (w *WorkloadConfig) GetFailureStrategy() string {
	if w.FailureStrategy == "" {
		return "continue" // Default to continue on failure
	}
	return w.FailureStrategy
}

// GetDatasetStartInterval returns the dataset start interval with a sensible default
func (w *WorkloadConfig) GetDatasetStartInterval() time.Duration {
	if w.DatasetStartInterval == 0 {
		return 30 * time.Second // Default to 30 seconds between dataset starts
	}
	return w.DatasetStartInterval
}

// MetricsConfig defines data collection parameters
type MetricsConfig struct {
	OutputDirectory string `mapstructure:"output_directory"`
}

// InfrastructureConfig defines UC3 system connection details
type InfrastructureConfig struct {
	// Kubernetes
	KubeConfig     string `mapstructure:"kube_config"`
	Namespace      string `mapstructure:"namespace"`
	DeploymentName string `mapstructure:"deployment_name"`

	// UC3 API
	UC3APIBaseURL string `mapstructure:"uc3_api_base_url"`
	TriggerURL    string `mapstructure:"trigger_url"`

	// Redis
	RedisHost     string `mapstructure:"redis_host"`
	RedisPort     int    `mapstructure:"redis_port"`
	RedisPassword string `mapstructure:"redis_password"`
}
