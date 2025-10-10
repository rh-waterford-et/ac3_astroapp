package config

import (
	"fmt"
	"time"
)

// validateConfig validates the loaded configuration
func validateConfig(config *ExperimentConfig) error {
	// Validate scaling parameters
	if config.Scaling.ProcessorCount < 1 {
		return fmt.Errorf("processor_count must be at least 1")
	}
	if config.Scaling.ProcessorCount > 50 {
		return fmt.Errorf("processor_count should not exceed 50 for safety")
	}

	// Validate workload parameters
	if config.Workload.ProcessorType != "starlight" && config.Workload.ProcessorType != "ppxf" && config.Workload.ProcessorType != "" {
		return fmt.Errorf("processor_type must be 'starlight' or 'ppxf'")
	}

	// Validate timing parameters
	if config.Scaling.StabilizeTime < 0*time.Second {
		return fmt.Errorf("stabilize_time cannot be negative")
	}

	// Validate dataset staggering parameters
	if config.Workload.DatasetStartInterval < 0*time.Second {
		return fmt.Errorf("dataset_start_interval cannot be negative")
	}

	return nil
}
