package config

import (
	"fmt"

	"github.com/spf13/viper"
)

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
		// Only show warning if no specific config file was provided
		if configFile == "" {
			fmt.Printf("Warning: Config file not found, using defaults: %v\n", err)
		}
		// If a specific config file was provided but not found, that's an error
		if configFile != "" {
			return nil, fmt.Errorf("specified config file not found: %s (%v)", configFile, err)
		}
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
	viper.SetDefault("description", "UC3 autoscaling training data generation")

	// Scaling defaults
	viper.SetDefault("scaling.processor_count", 3)
	viper.SetDefault("scaling.stabilize_time", "0s")
	viper.SetDefault("scaling.enable_scaling", true) // Default to enabled for backward compatibility

	// Workload defaults
	viper.SetDefault("workload.dataset", "NGC7025")
	viper.SetDefault("workload.processor_type", "starlight")
	viper.SetDefault("workload.failure_strategy", "continue")
	viper.SetDefault("workload.dataset_start_interval", "30s")

	// Metrics defaults
	viper.SetDefault("metrics.output_directory", "./experiment-data")

	// Infrastructure defaults
	viper.SetDefault("infrastructure.kube_config", "") // Use in-cluster config
	viper.SetDefault("infrastructure.namespace", "uc3-applications")
	viper.SetDefault("infrastructure.deployment_name", "ucm-processor-deployment")
	viper.SetDefault("infrastructure.uc3_api_base_url", "https://uc3-backend-api-uc3-applications.apps.ac3.rh-horizon.eu")
	viper.SetDefault("infrastructure.trigger_url", "http://localhost:8081")
	viper.SetDefault("infrastructure.redis_host", "localhost")
	viper.SetDefault("infrastructure.redis_port", 6379)
	viper.SetDefault("infrastructure.redis_password", "")
}
