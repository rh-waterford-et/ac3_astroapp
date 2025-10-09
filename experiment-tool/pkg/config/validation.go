package config

import (
	"fmt"
	"time"
)

// validateConfig validates the loaded configuration
func validateConfig(config *ExperimentConfig) error {
	// Validate scaling parameters
	if len(config.Scaling.ProcessorCounts) == 0 {
		return fmt.Errorf("processor_counts must have at least one value")
	}
	for _, count := range config.Scaling.ProcessorCounts {
		if count < 1 {
			return fmt.Errorf("processor_counts must all be >= 1")
		}
		if count > 50 {
			return fmt.Errorf("processor_counts should not exceed 50 for safety")
		}
	}

	// Validate workload parameters
	if config.Workload.ProcessorType != "starlight" && config.Workload.ProcessorType != "ppxf" && config.Workload.ProcessorType != "" {
		return fmt.Errorf("processor_type must be 'starlight' or 'ppxf'")
	}

	// Validate timing parameters
	if config.Scaling.StabilizeTime < 0*time.Second {
		return fmt.Errorf("stabilize_time cannot be negative")
	}

	return nil
}
