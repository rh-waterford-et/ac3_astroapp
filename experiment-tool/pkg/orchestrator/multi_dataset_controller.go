package orchestrator

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
	"github.com/uc3/experiment-tool/pkg/api"
	"github.com/uc3/experiment-tool/pkg/collector"
	"github.com/uc3/experiment-tool/pkg/config"
	"github.com/uc3/experiment-tool/pkg/dataset"
	"github.com/uc3/experiment-tool/pkg/scaler"
)

// MultiDatasetController orchestrates concurrent multi-dataset experiments
type MultiDatasetController struct {
	config     *config.ExperimentConfig
	scaler     scaler.Scaler
	collector  *collector.RedisCollector
	s3Monitor  *collector.S3Monitor
	apiClient  *api.UC3Client
	outputDir  string
	experiment *collector.MultiDatasetExperiment

	// Concurrency control
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMultiDatasetController creates a new multi-dataset experiment controller
func NewMultiDatasetController(cfg *config.ExperimentConfig) (*MultiDatasetController, error) {
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

	// Convert config datasets to collector datasets
	datasets := cfg.Workload.GetDatasets()
	datasetConfigs := make([]collector.DatasetConfig, len(datasets))
	for i, ds := range datasets {
		datasetConfigs[i] = collector.DatasetConfig{
			Name:          ds.Name,
			ProcessorType: ds.ProcessorType,
		}
	}

	// Create multi-dataset experiment
	experiment := collector.NewMultiDatasetExperiment(datasetConfigs)

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	return &MultiDatasetController{
		config:     cfg,
		scaler:     k8sScaler,
		collector:  redisCollector,
		s3Monitor:  s3Monitor,
		apiClient:  apiClient,
		outputDir:  cfg.Metrics.OutputDirectory,
		experiment: experiment,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Run executes the multi-dataset experiment
func (mdc *MultiDatasetController) Run() error {
	defer mdc.cancel() // Ensure cleanup on exit

	log.Printf("[EXPERIMENT] Starting with %d datasets", mdc.experiment.GetTotalDatasetCount())

	// Step 1: Scale to configured processor count (if enabled)
	processorCount := mdc.config.Scaling.ProcessorCount
	var err error
	if mdc.config.Scaling.ShouldScale() {
		log.Printf("[EXPERIMENT] Scaling to %d processors", processorCount)
		err = mdc.scaler.Scale(mdc.ctx, int32(processorCount))
		if err != nil {
			return fmt.Errorf("failed to scale to %d processors: %w", processorCount, err)
		}

		// Wait for stabilization
		if mdc.config.Scaling.StabilizeTime > 0 {
			log.Printf("[EXPERIMENT] Waiting for stabilization (%s)", mdc.config.Scaling.StabilizeTime)
			time.Sleep(mdc.config.Scaling.StabilizeTime)
		}
	} else {
		log.Printf("[EXPERIMENT] Scaling disabled - using existing/HPA-managed pod count")
		// Optionally query current scale for reference
		currentScale, err := mdc.scaler.GetCurrentScale(mdc.ctx)
		if err == nil {
			log.Printf("[EXPERIMENT] Current processor count: %d", currentScale)
		}
	}

	// Step 2: Upload all datasets simultaneously (new optimized approach)
	err = mdc.uploadAllDatasets()
	if err != nil {
		return fmt.Errorf("failed to upload datasets: %w", err)
	}

	// Step 3: Launch dataset processing with intervals
	err = mdc.launchDatasetProcessing()
	if err != nil {
		return fmt.Errorf("failed to launch dataset processing: %w", err)
	}

	// Step 4: Wait for all datasets to complete
	mdc.wg.Wait()

	// Step 5: Generate final reports
	err = mdc.generateFinalReports()
	if err != nil {
		return fmt.Errorf("failed to generate final reports: %w", err)
	}

	log.Printf("[EXPERIMENT] Completed! Data: %s", mdc.outputDir)
	return nil
}

// uploadAllDatasets uploads all datasets to S3 with staggered starts to avoid overwhelming S3
func (mdc *MultiDatasetController) uploadAllDatasets() error {
	datasetsMap := mdc.experiment.GetAllDatasets()
	startInterval := mdc.config.Workload.GetDatasetStartInterval()

	// Convert map to slice for ordered iteration
	datasets := make([]*collector.DatasetExecution, 0, len(datasetsMap))
	for _, dataset := range datasetsMap {
		datasets = append(datasets, dataset)
	}

	log.Printf("[EXPERIMENT] Uploading %d datasets with %s interval between starts...", len(datasets), startInterval)

	// Use a channel to collect upload results
	type uploadResult struct {
		datasetExec   *collector.DatasetExecution
		experimentRun *collector.ExperimentRun
		err           error
	}

	resultChan := make(chan uploadResult, len(datasets))

	// Start uploads with staggered delays to avoid overwhelming S3
	for i, datasetExec := range datasets {
		go func(de *collector.DatasetExecution) {
			datasetName := de.Config.Name
			processorType := de.Config.ProcessorType

			log.Printf("[%s] Starting dataset upload...", datasetName)
			de.SetStatus(collector.StatusUploading)

			experimentRun, err := mdc.uploadDataset(datasetName, processorType)
			resultChan <- uploadResult{
				datasetExec:   de,
				experimentRun: experimentRun,
				err:           err,
			}
		}(datasetExec)

		// Wait before starting next upload (except for the last one)
		if i < len(datasets)-1 && startInterval > 0 {
			log.Printf("[EXPERIMENT] Waiting %s before starting next upload...", startInterval)
			time.Sleep(startInterval)
		}
	}

	// Collect all upload results
	uploadedCount := 0
	failedCount := 0

	for i := 0; i < len(datasets); i++ {
		result := <-resultChan

		if result.err != nil {
			log.Printf("[%s] Upload failed: %v", result.datasetExec.Config.Name, result.err)
			result.datasetExec.SetError(fmt.Errorf("upload failed: %w", result.err))
			mdc.experiment.IncrementFailedCount()
			failedCount++
		} else {
			log.Printf("[%s] Upload completed successfully", result.datasetExec.Config.Name)
			result.datasetExec.ExperimentRun = result.experimentRun
			result.datasetExec.SetStatus(collector.StatusReady) // New status: ready for processing
			uploadedCount++
		}
	}

	log.Printf("[EXPERIMENT] Upload complete: %d ok, %d failed", uploadedCount, failedCount)

	if uploadedCount == 0 {
		return fmt.Errorf("all dataset uploads failed")
	}

	return nil
}

// launchDatasetProcessing launches all ready datasets concurrently (uploads already done)
func (mdc *MultiDatasetController) launchDatasetProcessing() error {
	// Get only successfully uploaded datasets
	datasets := mdc.experiment.GetAllDatasets()
	readyDatasets := make([]*collector.DatasetExecution, 0)

	for _, datasetExec := range datasets {
		if datasetExec.GetStatus() == collector.StatusReady {
			readyDatasets = append(readyDatasets, datasetExec)
		}
	}

	if len(readyDatasets) == 0 {
		return fmt.Errorf("no datasets ready for processing")
	}

	// Get staggering configuration
	startInterval := mdc.config.Workload.GetDatasetStartInterval()

	log.Printf("[EXPERIMENT] Launching %d datasets with %s interval between starts",
		len(readyDatasets), startInterval)

	// Launch datasets with simple time delay between starts
	for i, datasetExec := range readyDatasets {
		mdc.wg.Add(1)
		go mdc.processDatasetFromReady(datasetExec)

		log.Printf("[%s] Dataset processing triggered (%d/%d)", datasetExec.Config.Name, i+1, len(readyDatasets))

		// Wait before triggering next dataset (except for the last one)
		// This prevents overwhelming the producer pod with simultaneous HTTP requests
		if i < len(readyDatasets)-1 && startInterval > 0 {
			log.Printf("[EXPERIMENT] Waiting %s before triggering next dataset...", startInterval)
			time.Sleep(startInterval)
		}
	}

	return nil
}

// processDatasetFromReady handles processing of a dataset that's already uploaded
func (mdc *MultiDatasetController) processDatasetFromReady(datasetExec *collector.DatasetExecution) {
	defer mdc.wg.Done()

	datasetName := datasetExec.Config.Name
	log.Printf("[%s] Starting processing", datasetName)

	mdc.experiment.IncrementActiveCount()
	defer mdc.experiment.DecrementActiveCount()

	// Dataset is already uploaded, go straight to processing
	datasetExec.SetStatus(collector.StatusProcessing)

	// Process dataset through all scaling steps
	err := mdc.runDatasetCycles(datasetExec)
	if err != nil {
		datasetExec.SetError(fmt.Errorf("processing failed: %w", err))
		mdc.experiment.IncrementFailedCount()
		mdc.handleDatasetFailure(datasetExec)
		return
	}

	// Check if already completed (from runDatasetCycles)
	if datasetExec.GetStatus() != collector.StatusCompleted {
		// This should not happen with our new logic, but safety check
		datasetExec.SetStatus(collector.StatusCompleted)
	}
	mdc.experiment.IncrementCompletedCount()

	log.Printf("[%s] Completed (%v)", datasetName, datasetExec.Duration())
}

// uploadDataset uploads a single dataset and returns the experiment run
func (mdc *MultiDatasetController) uploadDataset(datasetName, processorType string) (*collector.ExperimentRun, error) {
	log.Printf("[%s] Uploading...", datasetName)

	// Dynamic path resolution for containerized environments
	datasetPath := datasetName
	if datasetPath != "" {
		// Check if the provided path exists
		if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
			// Try container dataset paths
			containerPaths := []string{
				"/app/datasets/" + datasetName,
				"/app/datasets/NGC7025_short",
				"/app/datasets/NGC7025_full",
			}

			found := false
			for _, containerPath := range containerPaths {
				if _, err := os.Stat(containerPath); err == nil {
					log.Printf("[%s] Using container dataset path: %s", datasetName, containerPath)
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
	datasetManager, err := dataset.NewDatasetManager("experiment", processorType)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataset manager: %w", err)
	}

	localDataset, err := datasetManager.ScanLocalDataset(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan local dataset: %w", err)
	}

	log.Printf("[%s] Uploading dataset from: %s", datasetName, datasetPath)
	err = datasetManager.UploadDataset(localDataset)
	if err != nil {
		return nil, fmt.Errorf("failed to upload dataset: %w", err)
	}

	log.Printf("[%s] Uploaded %d files", datasetName, len(localDataset.Files))

	// Create ExperimentRun with uploaded file count
	// Use the passed-in datasetName parameter to ensure correct naming
	experimentRun, err := collector.NewExperimentRun(
		datasetName,
		processorType,
		len(localDataset.Files),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create experiment run: %w", err)
	}

	return experimentRun, nil
}

// runDatasetCycles processes a dataset once with the configured processor count
func (mdc *MultiDatasetController) runDatasetCycles(datasetExec *collector.DatasetExecution) error {
	datasetName := datasetExec.Config.Name
	experimentRun := datasetExec.ExperimentRun
	processorCount := mdc.config.Scaling.ProcessorCount

	// Check if dataset is already completed (should not happen, but safety check)
	if datasetExec.GetStatus() == collector.StatusCompleted {
		log.Printf("[%s] Dataset already completed, skipping", datasetName)
		return nil
	}

	log.Printf("[%s] Processing with %d processors", datasetName, processorCount)

	datasetExec.ProcessorCount = processorCount

	err := mdc.runSingleDatasetCycle(datasetExec, experimentRun, processorCount)
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}

	// Mark dataset as completed
	datasetExec.SetStatus(collector.StatusCompleted)
	log.Printf("[%s] Processing completed successfully", datasetName)

	return nil
}

// runSingleDatasetCycle executes processing for a dataset
func (mdc *MultiDatasetController) runSingleDatasetCycle(datasetExec *collector.DatasetExecution, experimentRun *collector.ExperimentRun, processorCount int) error {
	ctx := mdc.ctx
	datasetName := experimentRun.DatasetName

	// Step 1: Trigger UC3 processing
	log.Printf("[%s] Triggering", datasetName)
	err := mdc.apiClient.TriggerProcessing(datasetName, experimentRun.ProcessorType)
	if err != nil {
		return fmt.Errorf("failed to trigger processing: %w", err)
	}

	// Step 2: Wait for S3 completion
	err = mdc.s3Monitor.WaitForCompletionWithStatus(ctx, experimentRun, datasetExec)
	if err != nil {
		return fmt.Errorf("failed waiting for S3 completion: %w", err)
	}

	// Step 3: Wait for metrics aggregation
	log.Printf("[%s] Waiting for metrics", datasetName)
	err = mdc.waitForMetricsAggregation(ctx, experimentRun, processorCount)
	if err != nil {
		return fmt.Errorf("failed waiting for metrics aggregation: %w", err)
	}

	log.Printf("[%s] Processing done", datasetName)

	// Step 4: Collect and export data for this dataset
	err = mdc.collectAndExportDatasetData(ctx, processorCount, datasetName)
	if err != nil {
		return fmt.Errorf("failed to collect and export data: %w", err)
	}

	// Step 5: Sort the dataset's job records CSV by queue_start_time
	jobRecordsPath := fmt.Sprintf("%s/%s_job_records_detailed.csv", mdc.outputDir, datasetName)
	err = mdc.collector.SortJobRecordsCSV(jobRecordsPath)
	if err != nil {
		log.Printf("[%s] Warning: failed to sort job records CSV: %v", datasetName, err)
		// Don't fail the dataset for sorting issues
	}

	return nil
}

// waitForMetricsAggregation waits for the aggregation cycle to complete
func (mdc *MultiDatasetController) waitForMetricsAggregation(ctx context.Context, experimentRun *collector.ExperimentRun, processorCount int) error {
	// UC3 aggregates every 5 minutes, wait 5m30s to ensure we capture everything
	waitTime := 5*time.Minute + 30*time.Second

	log.Printf("[%s] Waiting for aggregation (%v)", experimentRun.DatasetName, waitTime)

	waitStartTime := time.Now()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	jobRecordsPath := fmt.Sprintf("%s/%s_job_records_detailed.csv", mdc.outputDir, experimentRun.DatasetName)

	// Initialize the job records CSV file with headers
	err := mdc.initializeJobRecordsCSV(jobRecordsPath)
	if err != nil {
		log.Printf("[%s] Warning: failed to initialize job records CSV: %v", experimentRun.DatasetName, err)
	}

	seenJobIDs := make(map[string]bool)

	for range ticker.C {
		timeWaited := time.Since(waitStartTime)

		newJobCount, err := mdc.collector.AppendJobRecordsToCSV(ctx, experimentRun.DatasetName, jobRecordsPath, seenJobIDs, processorCount)
		if err != nil {
			log.Printf("[%s] Failed to collect job records: %v", experimentRun.DatasetName, err)
		} else if newJobCount > 0 {
			log.Printf("[%s] Collected %d new jobs (total: %d)", experimentRun.DatasetName, newJobCount, len(seenJobIDs))
		}

		// Wait the full time for aggregation
		if timeWaited >= waitTime {
			if len(seenJobIDs) > 0 {
				log.Printf("[%s] Aggregation complete: %d jobs after %v", experimentRun.DatasetName, len(seenJobIDs), timeWaited.Round(time.Second))
			} else {
				log.Printf("[%s] No job records found after %v", experimentRun.DatasetName, timeWaited.Round(time.Second))
			}
			return nil
		}

		remainingWait := waitTime - timeWaited
		log.Printf("[%s] Waiting for aggregation: %d jobs, %v remaining", experimentRun.DatasetName, len(seenJobIDs), remainingWait.Round(time.Second))
	}

	return nil // Should never reach here, but satisfy return requirement
}

// initializeJobRecordsCSV creates a job records CSV file with proper headers
func (mdc *MultiDatasetController) initializeJobRecordsCSV(filename string) error {
	// Check if file already exists
	if _, err := os.Stat(filename); err == nil {
		return nil // File already exists, don't overwrite
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create job records file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header for individual job records
	header := []string{
		"batch_id",
		"job_id",
		"queue_start_time",
		"queue_receive_time",
		"job_end_time",
		"queue_duration_seconds",
		"processing_duration_seconds",
		"total_duration_seconds",
		"job_size_mb",
		"processor_count",
		"queue_ahead_length",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	return nil
}

// collectAndExportDatasetData collects and exports data for a single dataset
func (mdc *MultiDatasetController) collectAndExportDatasetData(ctx context.Context, processorCount int, datasetName string) error {
	log.Printf("[%s] Collecting and exporting data...", datasetName)

	// Export batch summaries for this dataset
	summaries, err := mdc.collector.GetBatchSummariesForDataset(ctx, datasetName)
	if err != nil {
		return fmt.Errorf("failed to get batch summaries: %w", err)
	}

	// Export to dataset-specific file
	summaryFile := fmt.Sprintf("%s/%s_batch_summaries_%d_processors.csv", mdc.outputDir, datasetName, processorCount)
	err = mdc.exportBatchSummariesToCSV(summaries, summaryFile)
	if err != nil {
		return fmt.Errorf("failed to export batch summaries: %w", err)
	}

	log.Printf("[%s] Exported %d batch summaries to %s", datasetName, len(summaries), summaryFile)

	// Append to unified training data file
	trainingFile := fmt.Sprintf("%s/training_data.csv", mdc.outputDir)
	err = mdc.appendToTrainingData(summaries, processorCount, datasetName, trainingFile)
	if err != nil {
		return fmt.Errorf("failed to append to training data: %w", err)
	}

	log.Printf("[%s] Data exported successfully for %d processors", datasetName, processorCount)
	return nil
}

// exportBatchSummariesToCSV exports batch summaries to CSV (local implementation)
func (mdc *MultiDatasetController) exportBatchSummariesToCSV(summaries map[string]*metrics.BatchSummary, filename string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header for batch summary data
	header := []string{
		"batch_id",
		"job_count",
		"complete_job_count",
		"first_job_queue_start_time",
		"last_job_end_time",
		"total_batch_duration_seconds",
		"avg_queue_time_seconds",
		"avg_processing_time_seconds",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write batch summary records
	for batchID, summary := range summaries {
		record := []string{
			batchID,
			fmt.Sprintf("%d", summary.JobCount),
			fmt.Sprintf("%d", summary.CompleteJobCount),
			summary.FirstJobQueueStartTime.Format(time.RFC3339Nano),
			summary.LastJobEndTime.Format(time.RFC3339Nano),
			fmt.Sprintf("%.6f", summary.TotalBatchDuration.Seconds()),
			fmt.Sprintf("%.6f", summary.AvgJobQueueTime.Seconds()),
			fmt.Sprintf("%.6f", summary.AvgJobProcessingTime.Seconds()),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write batch summary record: %w", err)
		}
	}

	return nil
}

// appendToTrainingData appends dataset results to the unified training data file
func (mdc *MultiDatasetController) appendToTrainingData(summaries map[string]*metrics.BatchSummary, processorCount int, datasetName, filename string) error {
	// Calculate aggregated statistics for this dataset
	totalJobs := 0
	totalBatches := len(summaries)
	var totalQueueTime, totalProcessingTime time.Duration

	for _, summary := range summaries {
		totalJobs += summary.JobCount
		totalQueueTime += summary.AvgJobQueueTime * time.Duration(summary.JobCount)
		totalProcessingTime += summary.AvgJobProcessingTime * time.Duration(summary.JobCount)
	}

	if totalJobs == 0 {
		return fmt.Errorf("no jobs found for dataset %s", datasetName)
	}

	avgQueueTime := totalQueueTime.Seconds() / float64(totalJobs)
	avgProcessingTime := totalProcessingTime.Seconds() / float64(totalJobs)

	// Check if file exists to determine if we need to write header
	fileExists := true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fileExists = false
	}

	// Open file for appending
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open training data file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header if file is new
	if !fileExists {
		header := []string{
			"processors",
			"dataset",
			"batch_count",
			"job_count",
			"avg_queue_time_seconds",
			"avg_processing_time_seconds",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Write training data record
	record := []string{
		fmt.Sprintf("%d", processorCount),
		datasetName,
		fmt.Sprintf("%d", totalBatches),
		fmt.Sprintf("%d", totalJobs),
		fmt.Sprintf("%.6f", avgQueueTime),
		fmt.Sprintf("%.6f", avgProcessingTime),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write training data record: %w", err)
	}

	return nil
}

// handleDatasetFailure handles failure of a single dataset
func (mdc *MultiDatasetController) handleDatasetFailure(datasetExec *collector.DatasetExecution) {
	datasetName := datasetExec.Config.Name
	err := datasetExec.GetError()

	log.Printf("[%s] Dataset failed: %v", datasetName, err)

	failureStrategy := mdc.config.Workload.GetFailureStrategy()
	if failureStrategy == "abort_all" {
		log.Printf("[EXPERIMENT] Failure strategy is 'abort_all', cancelling all datasets")
		mdc.cancel() // Cancel all other datasets
	} else {
		log.Printf("[%s] Failure strategy is 'continue', other datasets will continue", datasetName)
	}
}

// generateFinalReports creates final experiment reports
func (mdc *MultiDatasetController) generateFinalReports() error {
	log.Printf("[EXPERIMENT] Generating final reports...")

	// Generate experiment summary
	err := mdc.generateExperimentSummary()
	if err != nil {
		return fmt.Errorf("failed to generate experiment summary: %w", err)
	}

	// Generate dataset timeline
	err = mdc.generateDatasetTimeline()
	if err != nil {
		return fmt.Errorf("failed to generate dataset timeline: %w", err)
	}

	// Merge all job records into consolidated file
	err = mdc.mergeAllJobRecords()
	if err != nil {
		return fmt.Errorf("failed to merge job records: %w", err)
	}

	return nil
}

// generateExperimentSummary creates an overall experiment summary
func (mdc *MultiDatasetController) generateExperimentSummary() error {
	summaryFile := fmt.Sprintf("%s/experiment_summary.csv", mdc.outputDir)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(summaryFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(summaryFile)
	if err != nil {
		return fmt.Errorf("failed to create experiment summary file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"experiment_name",
		"total_datasets",
		"completed_datasets",
		"failed_datasets",
		"total_duration_seconds",
		"failure_strategy",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Calculate experiment statistics
	totalDatasets := mdc.experiment.GetTotalDatasetCount()
	completedDatasets := mdc.experiment.GetCompletedCount()
	failedDatasets := mdc.experiment.GetFailedCount()

	// Calculate total experiment duration
	var experimentStart, experimentEnd time.Time
	datasets := mdc.experiment.GetAllDatasets()
	first := true

	for _, dataset := range datasets {
		if first {
			experimentStart = dataset.StartTime
			experimentEnd = dataset.StartTime
			first = false
		}

		if dataset.StartTime.Before(experimentStart) {
			experimentStart = dataset.StartTime
		}

		if !dataset.CompletionTime.IsZero() && dataset.CompletionTime.After(experimentEnd) {
			experimentEnd = dataset.CompletionTime
		}
	}

	totalDuration := experimentEnd.Sub(experimentStart)

	// Write experiment summary record
	record := []string{
		mdc.config.Name,
		fmt.Sprintf("%d", totalDatasets),
		fmt.Sprintf("%d", completedDatasets),
		fmt.Sprintf("%d", failedDatasets),
		fmt.Sprintf("%.2f", totalDuration.Seconds()),
		mdc.config.Workload.GetFailureStrategy(),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write experiment summary record: %w", err)
	}

	log.Printf("[EXPERIMENT] Generated experiment summary: %s", summaryFile)
	return nil
}

// generateDatasetTimeline creates a timeline of dataset execution
func (mdc *MultiDatasetController) generateDatasetTimeline() error {
	timelineFile := fmt.Sprintf("%s/dataset_timeline.csv", mdc.outputDir)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(timelineFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(timelineFile)
	if err != nil {
		return fmt.Errorf("failed to create dataset timeline file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"dataset",
		"processor_type",
		"start_time",
		"completion_time",
		"duration_seconds",
		"status",
		"processor_count",
		"error_message",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write dataset timeline records
	datasets := mdc.experiment.GetAllDatasets()
	for _, dataset := range datasets {
		completionTimeStr := ""
		durationStr := "0"
		errorMsg := ""

		if !dataset.CompletionTime.IsZero() {
			completionTimeStr = dataset.CompletionTime.Format(time.RFC3339Nano)
			durationStr = fmt.Sprintf("%.2f", dataset.Duration().Seconds())
		}

		if dataset.GetError() != nil {
			errorMsg = dataset.GetError().Error()
		}

		record := []string{
			dataset.Config.Name,
			dataset.Config.ProcessorType,
			dataset.StartTime.Format(time.RFC3339Nano),
			completionTimeStr,
			durationStr,
			string(dataset.GetStatus()),
			fmt.Sprintf("%d", dataset.ProcessorCount),
			errorMsg,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write dataset timeline record: %w", err)
		}
	}

	log.Printf("[EXPERIMENT] Generated dataset timeline: %s", timelineFile)
	return nil
}

// mergeAllJobRecords merges all per-dataset job_records_detailed.csv files into one consolidated file
func (mdc *MultiDatasetController) mergeAllJobRecords() error {
	consolidatedFile := fmt.Sprintf("%s/all_jobs_detailed.csv", mdc.outputDir)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(consolidatedFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Collect all records from all datasets
	var allRecords [][]string
	var header []string

	// Get all datasets
	datasets := mdc.experiment.GetAllDatasets()

	// Iterate through each dataset and collect its job records
	for _, dataset := range datasets {
		datasetJobFile := fmt.Sprintf("%s/%s_job_records_detailed.csv", mdc.outputDir, dataset.Config.Name)

		// Check if file exists
		if _, err := os.Stat(datasetJobFile); os.IsNotExist(err) {
			log.Printf("[EXPERIMENT] Skipping %s (file not found)", datasetJobFile)
			continue
		}

		// Open dataset-specific job records file
		inFile, err := os.Open(datasetJobFile)
		if err != nil {
			log.Printf("[EXPERIMENT] Warning: failed to open %s: %v", datasetJobFile, err)
			continue
		}

		reader := csv.NewReader(inFile)

		// Read all records
		records, err := reader.ReadAll()
		inFile.Close()

		if err != nil {
			log.Printf("[EXPERIMENT] Warning: failed to read %s: %v", datasetJobFile, err)
			continue
		}

		if len(records) == 0 {
			continue
		}

		// Save header from first file
		if len(header) == 0 {
			header = records[0]
		}

		// Collect all data records (skip header)
		allRecords = append(allRecords, records[1:]...)

		log.Printf("[EXPERIMENT] Collected %d records from %s", len(records)-1, filepath.Base(datasetJobFile))
	}

	if len(allRecords) == 0 {
		log.Printf("[EXPERIMENT] No job records to merge")
		return nil
	}

	// Sort all records by queue_start_time (column index 2)
	sort.Slice(allRecords, func(i, j int) bool {
		timeI, errI := time.Parse(time.RFC3339Nano, allRecords[i][2])
		timeJ, errJ := time.Parse(time.RFC3339Nano, allRecords[j][2])

		if errI != nil || errJ != nil {
			return false // Keep original order if parse fails
		}

		return timeI.Before(timeJ)
	})

	// Write sorted consolidated file
	outFile, err := os.Create(consolidatedFile)
	if err != nil {
		return fmt.Errorf("failed to create consolidated job records file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	// Write header
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write all sorted records
	for _, record := range allRecords {
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	log.Printf("[EXPERIMENT] Generated consolidated job records: %s (%d total records, sorted by queue_start_time)", consolidatedFile, len(allRecords))
	return nil
}

// Close cleans up resources
func (mdc *MultiDatasetController) Close() error {
	mdc.cancel()
	return nil
}
