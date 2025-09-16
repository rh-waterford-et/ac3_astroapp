package orchestrator

import (
	"context"
	"fmt"
	"log"
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

	// Create UC3 API client
	apiClient := api.NewUC3Client(cfg.Infrastructure.UC3APIBaseURL)

	return &ExperimentController{
		config:    cfg,
		scaler:    k8sScaler,
		collector: redisCollector,
		apiClient: apiClient,
		outputDir: cfg.Metrics.OutputDirectory,
	}, nil
}

// RunExperiment executes the complete autoscaling experiment
func (e *ExperimentController) RunExperiment() error {
	log.Printf("Starting UC3 Autoscaling Experiment: %s", e.config.Name)
	log.Printf("Duration: %s", e.config.Duration)
	log.Printf("Processor Range: %d-%d", e.config.Scaling.MinProcessors, e.config.Scaling.MaxProcessors)
	log.Printf("Output Directory: %s", e.outputDir)

	// Step 1: Upload dataset to S3
	datasetName, err := e.uploadDataset()
	if err != nil {
		return fmt.Errorf("failed to upload dataset: %w", err)
	}

	// Step 2: Run experiment cycles for each processor count
	for i, processorCount := range e.config.Scaling.ScaleSteps {
		log.Printf("Running cycle %d/%d: %d processors", i+1, len(e.config.Scaling.ScaleSteps), processorCount)

		err := e.runSingleCycle(datasetName, processorCount)
		if err != nil {
			log.Printf("Failed cycle for %d processors: %v", processorCount, err)
			continue // Skip failed cycles, continue with remaining
		}

		log.Printf("Completed cycle %d/%d successfully", i+1, len(e.config.Scaling.ScaleSteps))
	}

	log.Printf("Experiment completed successfully! Data saved to: %s", e.outputDir)
	return nil
}

// uploadDataset uploads the local dataset to S3
func (e *ExperimentController) uploadDataset() (string, error) {
	log.Printf("Uploading dataset from: %s", e.config.Workload.Dataset)

	// Create dataset manager
	datasetManager, err := dataset.NewDatasetManager(
		e.config.Name, // experiment ID
		e.config.Workload.ProcessorType,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create dataset manager: %w", err)
	}

	// Scan local dataset
	localDataset, err := datasetManager.ScanLocalDataset(e.config.Workload.Dataset)
	if err != nil {
		return "", fmt.Errorf("failed to scan local dataset: %w", err)
	}

	// Upload to S3
	err = datasetManager.UploadDataset(localDataset)
	if err != nil {
		return "", fmt.Errorf("failed to upload dataset: %w", err)
	}

	log.Printf("Dataset uploaded successfully: %s", localDataset.DatasetName)
	return localDataset.DatasetName, nil
}

// runSingleCycle executes one complete processor count cycle
func (e *ExperimentController) runSingleCycle(datasetName string, processorCount int) error {
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
	log.Printf("Triggering processing for dataset: %s", datasetName)
	err = e.apiClient.TriggerProcessing(datasetName, e.config.Workload.ProcessorType)
	if err != nil {
		return fmt.Errorf("failed to trigger processing: %w", err)
	}

	// Step 4: Wait for completion (we'll need to detect batch ID somehow)
	batchID, err := e.waitForProcessingCompletion(ctx, datasetName)
	if err != nil {
		return fmt.Errorf("failed waiting for completion: %w", err)
	}

	// Step 5: Collect and export data
	err = e.collectAndExportData(ctx, processorCount, batchID)
	if err != nil {
		return fmt.Errorf("failed to collect data: %w", err)
	}

	return nil
}

// waitForProcessingCompletion waits for processing to complete and returns batch ID
func (e *ExperimentController) waitForProcessingCompletion(ctx context.Context, datasetName string) (string, error) {
	// The batch ID is the dataset name (confirmed from UC3 code analysis)
	batchID := datasetName

	log.Printf("Waiting for batch completion: %s", batchID)

	// Use a reasonable timeout for processing (e.g., 2 hours)
	timeout := 2 * time.Hour
	err := e.collector.WaitForBatchCompletion(ctx, batchID, timeout)
	if err != nil {
		return "", fmt.Errorf("batch %s failed to complete: %w", batchID, err)
	}

	log.Printf("Batch %s completed successfully", batchID)
	return batchID, nil
}

// collectAndExportData collects metrics and exports to CSV files
func (e *ExperimentController) collectAndExportData(ctx context.Context, processorCount int, batchID string) error {
	log.Printf("Collecting and exporting data for batch: %s", batchID)

	// Export individual job metrics
	jobMetricsPath := filepath.Join(e.outputDir, fmt.Sprintf("job_records_%d_processors.csv", processorCount))
	err := e.collector.ExportJobMetricsCSV(ctx, batchID, jobMetricsPath)
	if err != nil {
		return fmt.Errorf("failed to export job metrics: %w", err)
	}

	// Export training data point
	trainingDataPath := filepath.Join(e.outputDir, "training_data.csv")
	err = e.collector.ExportTrainingDataPoint(ctx, batchID, processorCount, trainingDataPath)
	if err != nil {
		return fmt.Errorf("failed to export training data: %w", err)
	}

	log.Printf("Data exported successfully for %d processors", processorCount)
	return nil
}

// Close cleans up resources
func (e *ExperimentController) Close() error {
	if e.collector != nil {
		return e.collector.Close()
	}
	return nil
}
