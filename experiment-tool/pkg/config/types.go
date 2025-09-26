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
	MinProcessors     int           `mapstructure:"min_processors"`
	MaxProcessors     int           `mapstructure:"max_processors"`
	ScaleSteps        []int         `mapstructure:"scale_steps"`
	StabilizeTime     time.Duration `mapstructure:"stabilize_time"`
	ScaleUpInterval   time.Duration `mapstructure:"scale_up_interval"`
	ScaleDownInterval time.Duration `mapstructure:"scale_down_interval"`
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
	Datasets              []DatasetConfig `mapstructure:"datasets"`
	DatasetStartInterval  time.Duration   `mapstructure:"dataset_start_interval"`
	MaxConcurrentDatasets int             `mapstructure:"max_concurrent_datasets"`
	FailureStrategy       string          `mapstructure:"failure_strategy"` // "continue" | "abort_all"

	// Common fields
	BatchSize         int           `mapstructure:"batch_size"`
	SubmissionRate    time.Duration `mapstructure:"submission_rate"`
	JobSizeVariation  string        `mapstructure:"job_size_variation"` // "small", "medium", "large", "mixed"
	PausesBetweenJobs bool          `mapstructure:"pauses_between_jobs"`
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

// GetMaxConcurrentDatasets returns the max concurrent datasets with a sensible default
func (w *WorkloadConfig) GetMaxConcurrentDatasets() int {
	if w.MaxConcurrentDatasets <= 0 {
		if w.IsMultiDataset() {
			return 3 // Default for multi-dataset mode
		}
		return 1 // Single dataset mode
	}
	return w.MaxConcurrentDatasets
}

// GetDatasetStartInterval returns the dataset start interval with a sensible default
func (w *WorkloadConfig) GetDatasetStartInterval() time.Duration {
	if w.DatasetStartInterval <= 0 {
		return 1 * time.Minute // Default 1 minute between dataset starts
	}
	return w.DatasetStartInterval
}

// MetricsConfig defines data collection parameters
type MetricsConfig struct {
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	ExportFormat       string        `mapstructure:"export_format"`
	OutputDirectory    string        `mapstructure:"output_directory"`
	IncludeJobLevel    bool          `mapstructure:"include_job_level"`
	IncludeSummary     bool          `mapstructure:"include_summary"`
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

	// RabbitMQ Exporter
	RabbitMQExporterURL string `mapstructure:"rabbitmq_exporter_url"`
}
