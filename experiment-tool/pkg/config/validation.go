package config

import (
	"fmt"
	"time"
)

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

	// Validate timing parameters - removed 30s minimum requirement
	if config.Scaling.StabilizeTime < 0*time.Second {
		return fmt.Errorf("stabilize_time cannot be negative")
	}

	return nil
}
