package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

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

// LoadConfig loads configuration from file, environment, and flags
func LoadConfig(configFile string) (*ExperimentConfig, error) {
	// Set defaults
	setDefaults()

	// Set config file if provided
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Look for config in common locations
		viper.SetConfigName("experiment")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("$HOME/.uc3-experiment")
	}

	// Environment variable support
	viper.SetEnvPrefix("UC3_EXP")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// Config file not required, continue with defaults and env vars
		fmt.Printf("Warning: Config file not found, using defaults: %v\n", err)
	}

	// Unmarshal into struct
	var config ExperimentConfig
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets reasonable defaults for all configuration values
func setDefaults() {
	// Experiment defaults
	viper.SetDefault("name", "uc3-autoscaling-experiment")
	viper.SetDefault("duration", "30m")
	viper.SetDefault("description", "UC3 autoscaling training data generation")

	// Scaling defaults
	viper.SetDefault("scaling.min_processors", 1)
	viper.SetDefault("scaling.max_processors", 10)
	viper.SetDefault("scaling.scale_steps", []int{1, 2, 3, 5, 7, 10})
	viper.SetDefault("scaling.stabilize_time", "2m")
	viper.SetDefault("scaling.scale_up_interval", "5m")
	viper.SetDefault("scaling.scale_down_interval", "3m")

	// Workload defaults
	viper.SetDefault("workload.dataset", "NGC7025")
	viper.SetDefault("workload.processor_type", "starlight")
	viper.SetDefault("workload.batch_size", 10)
	viper.SetDefault("workload.submission_rate", "30s")
	viper.SetDefault("workload.job_size_variation", "mixed")
	viper.SetDefault("workload.pauses_between_jobs", false)

	// Metrics defaults
	viper.SetDefault("metrics.collection_interval", "30s")
	viper.SetDefault("metrics.export_format", "csv")
	viper.SetDefault("metrics.output_directory", "./experiment-data")
	viper.SetDefault("metrics.include_job_level", true)
	viper.SetDefault("metrics.include_summary", true)

	// Infrastructure defaults
	viper.SetDefault("infrastructure.kube_config", "") // Use in-cluster config
	viper.SetDefault("infrastructure.namespace", "ac3-astroapp")
	viper.SetDefault("infrastructure.deployment_name", "ucm-starlight-processor")
	viper.SetDefault("infrastructure.uc3_api_base_url", "http://localhost:8080")
	viper.SetDefault("infrastructure.trigger_url", "http://localhost:8081")
	viper.SetDefault("infrastructure.redis_host", "localhost")
	viper.SetDefault("infrastructure.redis_port", 6379)
	viper.SetDefault("infrastructure.redis_password", "")
	viper.SetDefault("infrastructure.rabbitmq_exporter_url", "http://localhost:9419/metrics")
}

// validateConfig validates the loaded configuration
func validateConfig(config *ExperimentConfig) error {
	// Validate scaling parameters
	if config.Scaling.MinProcessors < 1 {
		return fmt.Errorf("min_processors must be at least 1")
	}
	if config.Scaling.MaxProcessors < config.Scaling.MinProcessors {
		return fmt.Errorf("max_processors must be >= min_processors")
	}
	if config.Scaling.MaxProcessors > 50 {
		return fmt.Errorf("max_processors should not exceed 50 for safety")
	}

	// Validate workload parameters
	if config.Workload.BatchSize < 1 {
		return fmt.Errorf("batch_size must be at least 1")
	}
	if config.Workload.ProcessorType != "starlight" && config.Workload.ProcessorType != "ppxf" {
		return fmt.Errorf("processor_type must be 'starlight' or 'ppxf'")
	}

	// Validate job size variation
	validSizes := map[string]bool{"small": true, "medium": true, "large": true, "mixed": true}
	if !validSizes[config.Workload.JobSizeVariation] {
		return fmt.Errorf("job_size_variation must be one of: small, medium, large, mixed")
	}

	// Validate metrics configuration
	if config.Metrics.ExportFormat != "csv" && config.Metrics.ExportFormat != "json" {
		return fmt.Errorf("export_format must be 'csv' or 'json'")
	}

	// Validate durations
	if config.Duration < time.Minute {
		return fmt.Errorf("experiment duration must be at least 1 minute")
	}
	if config.Scaling.StabilizeTime < 30*time.Second {
		return fmt.Errorf("stabilize_time must be at least 30 seconds")
	}

	return nil
}

// GetString is a convenience method for getting string values
func GetString(key string) string {
	return viper.GetString(key)
}

// GetInt is a convenience method for getting int values
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetBool is a convenience method for getting bool values
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetDuration is a convenience method for getting duration values
func GetDuration(key string) time.Duration {
	return viper.GetDuration(key)
}
