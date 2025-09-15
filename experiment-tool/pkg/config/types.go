package config

import "time"

// ExperimentConfig holds all configuration for autoscaling experiments
type ExperimentConfig struct {
	// Experiment Parameters
	Name        string        `mapstructure:"name"`
	Duration    time.Duration `mapstructure:"duration"`
	Description string        `mapstructure:"description"`

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

// WorkloadConfig defines job submission patterns
type WorkloadConfig struct {
	Dataset           string        `mapstructure:"dataset"`
	ProcessorType     string        `mapstructure:"processor_type"`
	BatchSize         int           `mapstructure:"batch_size"`
	SubmissionRate    time.Duration `mapstructure:"submission_rate"`
	JobSizeVariation  string        `mapstructure:"job_size_variation"` // "small", "medium", "large", "mixed"
	PausesBetweenJobs bool          `mapstructure:"pauses_between_jobs"`
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
