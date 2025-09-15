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

	// Validate durations
	if config.Duration < time.Minute {
		return fmt.Errorf("experiment duration must be at least 1 minute")
	}
	if config.Scaling.StabilizeTime < 30*time.Second {
		return fmt.Errorf("stabilize_time must be at least 30 seconds")
	}

	return nil
}
