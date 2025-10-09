package orchestrator

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/uc3/experiment-tool/pkg/api"
	"github.com/uc3/experiment-tool/pkg/collector"
	"github.com/uc3/experiment-tool/pkg/config"
	"github.com/uc3/experiment-tool/pkg/dataset"
	"github.com/uc3/experiment-tool/pkg/scaler"
)

// ExperimentController orchestrates the complete autoscaling experiment
type ExperimentController struct {
	config    *config.ExperimentConfig
	scaler    scaler.Scaler
	collector *collector.RedisCollector
	s3Monitor *collector.S3Monitor
	apiClient *api.UC3Client
	outputDir string
}

// NewExperimentController creates a new experiment controller
func NewExperimentController(cfg *config.ExperimentConfig) (*ExperimentController, error) {
	// Create Kubernetes scaler
	scalerConfig := scaler.ScalerConfig{
		KubeConfig:     cfg.Infrastructure.KubeConfig,
		Namespace:      cfg.Infrastructure.Namespace,
		DeploymentName: cfg.Infrastructure.DeploymentName,
		Timeout:        5 * time.Minute,
	}

	k8sScaler, err := scaler.NewKubernetesScaler(scalerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create scaler: %w", err)
	}

	// Create Redis collector
	redisCollector, err := collector.NewRedisCollector()
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis collector: %w", err)
	}

	// Create S3 monitor
	s3Monitor, err := collector.NewS3Monitor()
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 monitor: %w", err)
	}

	// Create UC3 API client
	apiClient := api.NewUC3Client(cfg.Infrastructure.UC3APIBaseURL)

	return &ExperimentController{
		config:    cfg,
		scaler:    k8sScaler,
		collector: redisCollector,
		s3Monitor: s3Monitor,
		apiClient: apiClient,
		outputDir: cfg.Metrics.OutputDirectory,
	}, nil
}

// RunExperiment executes the complete autoscaling experiment
func (e *ExperimentController) RunExperiment() error {
	log.Printf("Starting UC3 Autoscaling Experiment: %s", e.config.Name)
	log.Printf("Description: %s", e.config.Description)
	log.Printf("Dataset: %s", e.config.Workload.Dataset)
	log.Printf("Processor Type: %s", e.config.Workload.ProcessorType)
	log.Printf("Scale Steps: %v", e.config.Scaling.ScaleSteps)
	log.Printf("Output Directory: %s", e.outputDir)

	// Step 1: Upload dataset to S3
	experimentRun, err := e.uploadDataset(e.config.Workload.Dataset)
	if err != nil {
		return fmt.Errorf("failed to upload dataset: %w", err)
	}

	// Step 2: Run experiment cycles for each processor count
	for i, processorCount := range e.config.Scaling.ScaleSteps {
		log.Printf("Running cycle %d/%d: %d processors", i+1, len(e.config.Scaling.ScaleSteps), processorCount)

		err := e.runSingleCycle(experimentRun, processorCount)
		if err != nil {
			log.Printf("Failed cycle for %d processors: %v", processorCount, err)
			continue // Skip failed cycles, continue with remaining
		}

		log.Printf("Completed cycle %d/%d successfully", i+1, len(e.config.Scaling.ScaleSteps))
	}

	log.Printf("Experiment completed successfully! Data saved to: %s", e.outputDir)
	return nil
}

// uploadDataset uploads the dataset and returns experiment run information
func (e *ExperimentController) uploadDataset(datasetPath string) (*collector.ExperimentRun, error) {
	// Dynamic path resolution for containerized environments
	if datasetPath != "" {
		// Check if the provided path exists
		if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
			// Try container dataset paths
			containerPaths := []string{
				"/app/datasets/NGC7025_short",
				"/app/datasets/NGC7025_full",
				"/app/datasets/" + datasetPath,
			}

			found := false
			for _, containerPath := range containerPaths {
				if _, err := os.Stat(containerPath); err == nil {
					log.Printf("Using container dataset path: %s", containerPath)
					datasetPath = containerPath
					found = true
					break
				}
			}

			if !found {
				return nil, fmt.Errorf("dataset path not found: %s", datasetPath)
			}
		}
	}

	// Create dataset manager and upload
	datasetManager, err := dataset.NewDatasetManager("experiment", e.config.Workload.ProcessorType)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataset manager: %w", err)
	}

	localDataset, err := datasetManager.ScanLocalDataset(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan local dataset: %w", err)
	}

	log.Printf("Uploading dataset from: %s", datasetPath)
	err = datasetManager.UploadDataset(localDataset)
	if err != nil {
		return nil, fmt.Errorf("failed to upload dataset: %w", err)
	}

	log.Printf("Dataset uploaded successfully: %s", localDataset.DatasetName)

	// Create ExperimentRun with uploaded file count
	experimentRun, err := collector.NewExperimentRun(
		localDataset.DatasetName,
		e.config.Workload.ProcessorType,
		len(localDataset.Files), // Track uploaded file count
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create experiment run: %w", err)
	}

	return experimentRun, nil
}

// runSingleCycle executes one complete processor count cycle
func (e *ExperimentController) runSingleCycle(experimentRun *collector.ExperimentRun, processorCount int) error {
	ctx := context.Background()

	// Step 1: Scale to processor count
	log.Printf("Scaling to %d processors...", processorCount)
	err := e.scaler.Scale(ctx, int32(processorCount))
	if err != nil {
		return fmt.Errorf("failed to scale to %d processors: %w", processorCount, err)
	}

	// Step 2: Wait for stabilization
	log.Printf("Waiting for stabilization (%s)...", e.config.Scaling.StabilizeTime)
	time.Sleep(e.config.Scaling.StabilizeTime)

	// Step 3: Trigger UC3 processing
	log.Printf("Triggering processing for dataset: %s", experimentRun.DatasetName)
	err = e.apiClient.TriggerProcessing(experimentRun.DatasetName, e.config.Workload.ProcessorType)
	if err != nil {
		return fmt.Errorf("failed to trigger processing: %w", err)
	}

	// Step 4: Wait for completion using S3 monitoring
	err = e.waitForS3Completion(ctx, experimentRun)
	if err != nil {
		return fmt.Errorf("failed waiting for completion: %w", err)
	}

	// Step 5: Collect and export data
	err = e.collectAndExportData(ctx, processorCount, experimentRun.DatasetName)
	if err != nil {
		return fmt.Errorf("failed to collect and export data: %w", err)
	}

	// Step 6: Clean up S3 output files
	err = e.cleanupS3OutputFiles(ctx, experimentRun)
	if err != nil {
		log.Printf("Warning: failed to cleanup S3 output files: %v", err)
		// Don't fail the experiment for cleanup issues
	}

	return nil
}

// waitForS3Completion waits for processing to complete using S3 output monitoring
func (e *ExperimentController) waitForS3Completion(ctx context.Context, experimentRun *collector.ExperimentRun) error {
	log.Printf("Waiting for dataset processing completion: %s", experimentRun.DatasetName)

	// Use S3Monitor for reliable completion detection
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	err := e.s3Monitor.WaitForCompletion(timeoutCtx, experimentRun)
	if err != nil {
		return fmt.Errorf("dataset %s failed to complete: %w", experimentRun.DatasetName, err)
	}

	log.Printf("Dataset %s processing completed (S3 files ready)", experimentRun.DatasetName)

	// Now wait for metrics aggregation cycle
	log.Printf("Waiting for metrics aggregation cycle...")
	err = e.waitForMetricsAggregation(ctx, experimentRun)
	if err != nil {
		return fmt.Errorf("failed waiting for metrics aggregation: %w", err)
	}

	log.Printf("Dataset %s fully completed (metrics aggregated)", experimentRun.DatasetName)
	return nil
}

// waitForMetricsAggregation waits for the next aggregation cycle after processing completes
func (e *ExperimentController) waitForMetricsAggregation(ctx context.Context, experimentRun *collector.ExperimentRun) error {
	// UC3 aggregation runs every 5 minutes
	aggregationInterval := 5 * time.Minute
	maxWaitTime := 6 * time.Minute // 5 min + 1 min buffer

	log.Printf("Waiting for next aggregation cycle to capture all jobs...")
	log.Printf("UC3 aggregates every %v, waiting up to %v", aggregationInterval, maxWaitTime)

	// Create CSV file for job records at the start
	jobRecordsPath := filepath.Join(e.outputDir, "job_records_detailed.csv")
	err := e.createJobRecordsCSV(jobRecordsPath)
	if err != nil {
		return fmt.Errorf("failed to create job records CSV: %w", err)
	}

	// Track seen job IDs to avoid duplicates
	seenJobIDs := make(map[string]bool)

	// Record when we started waiting (after S3 completion)
	waitStartTime := time.Now()

	// We must wait at least 5 minutes to ensure the next aggregation cycle runs
	minimumWaitTime := aggregationInterval

	ticker := time.NewTicker(15 * time.Second) // Check more frequently for new jobs
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, maxWaitTime)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			log.Printf("Timeout reached, proceeding with available job records")
			return nil // Don't fail, just proceed with what we have
		case <-ticker.C:
			timeWaited := time.Since(waitStartTime)

			// Dynamically collect new job records
			newJobCount, err := e.collector.AppendJobRecordsToCSV(ctx, experimentRun.DatasetName, jobRecordsPath, seenJobIDs)
			if err != nil {
				log.Printf("Error collecting job records: %v", err)
			} else if newJobCount > 0 {
				log.Printf("Collected %d new job records (total seen: %d)", newJobCount, len(seenJobIDs))
			}

			// Check current summaries for job count reference
			summaries, err := e.collector.GetBatchSummariesForDataset(ctx, experimentRun.DatasetName)
			if err != nil {
				log.Printf("Error checking summaries: %v", err)
				continue
			}

			// Count current jobs
			totalJobs := 0

			for batchID, summary := range summaries {
				totalJobs += summary.JobCount
				log.Printf("Batch %s: %d jobs", batchID, summary.JobCount)
			}

			log.Printf("Current state: %d total jobs across %d summaries, %d individual records collected (waited %v/%v)",
				totalJobs, len(summaries), len(seenJobIDs),
				timeWaited.Round(time.Second), minimumWaitTime)

			// Only exit if we've waited at least 5 minutes AND have job records
			if timeWaited >= minimumWaitTime {
				if len(seenJobIDs) > 0 {
					log.Printf("Minimum wait time elapsed and job records collected! (%d individual jobs after %v)",
						len(seenJobIDs), timeWaited.Round(time.Second))
					return nil
				} else {
					log.Printf("Minimum wait time elapsed but no job records found - this may indicate an issue")
					return nil
				}
			} else {
				remainingWait := minimumWaitTime - timeWaited
				log.Printf("Must wait at least %v more for next aggregation cycle...",
					remainingWait.Round(time.Second))
			}
		}
	}
}

// createJobRecordsCSV creates the CSV file with headers for individual job records
func (e *ExperimentController) createJobRecordsCSV(csvPath string) error {
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV headers
	headers := []string{
		"batch_id",
		"job_id",
		"queue_start_time",
		"queue_receive_time",
		"job_end_time",
		"queue_duration_seconds",
		"processing_duration_seconds",
		"total_duration_seconds",
		"job_size_mb",
		"queue_ahead_length",
	}

	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	log.Printf("Created job records CSV: %s", csvPath)
	return nil
}

// collectAndExportData collects metrics and exports to CSV files
func (e *ExperimentController) collectAndExportData(ctx context.Context, processorCount int, datasetName string) error {
	log.Printf("Collecting and exporting data for dataset: %s", datasetName)

	// Export individual job metrics
	jobMetricsPath := filepath.Join(e.outputDir, fmt.Sprintf("job_records_%d_processors.csv", processorCount))
	err := e.collector.ExportJobMetricsCSV(ctx, datasetName, jobMetricsPath)
	if err != nil {
		return fmt.Errorf("failed to export job metrics: %w", err)
	}

	// Export training data point
	trainingDataPath := filepath.Join(e.outputDir, "training_data.csv")
	err = e.collector.ExportTrainingDataPoint(ctx, datasetName, processorCount, trainingDataPath)
	if err != nil {
		return fmt.Errorf("failed to export training data: %w", err)
	}

	log.Printf("Data exported successfully for %d processors", processorCount)
	return nil
}

// cleanupS3OutputFiles removes the input, processed, and output files from S3 after successful data collection
func (e *ExperimentController) cleanupS3OutputFiles(ctx context.Context, experimentRun *collector.ExperimentRun) error {
	log.Printf("Cleaning up S3 files (input, processed, output) for dataset: %s", experimentRun.DatasetName)

	// Clean up input files for this specific dataset
	inputPath := fmt.Sprintf("%s/input/%s/", experimentRun.ProcessorType, experimentRun.DatasetName)
	log.Printf("Deleting S3 input directory: %s", inputPath)
	err := e.s3Monitor.GetS3Client().DeleteDirectory(inputPath)
	if err != nil {
		log.Printf("Warning: failed to delete input directory %s: %v", inputPath, err)
	}

	// Clean up processed files for this specific dataset
	processedPath := fmt.Sprintf("%s/processed/%s/", experimentRun.ProcessorType, experimentRun.DatasetName)
	log.Printf("Deleting S3 processed directory: %s", processedPath)
	err = e.s3Monitor.GetS3Client().DeleteDirectory(processedPath)
	if err != nil {
		log.Printf("Warning: failed to delete processed directory %s: %v", processedPath, err)
	}

	// Delete the output directory for this specific dataset
	outputPath := fmt.Sprintf("%s/output/%s/", experimentRun.ProcessorType, experimentRun.DatasetName)
	log.Printf("Deleting S3 output directory: %s", outputPath)
	err = e.s3Monitor.GetS3Client().DeleteDirectory(outputPath)
	if err != nil {
		return fmt.Errorf("failed to delete output directory %s: %w", outputPath, err)
	}

	log.Printf("Successfully deleted S3 files (input, processed, output) for dataset: %s", experimentRun.DatasetName)
	return nil
}

// Close cleans up resources
func (e *ExperimentController) Close() error {
	if e.collector != nil {
		return e.collector.Close()
	}
	return nil
}
