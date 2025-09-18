package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
)

// RedisCollector handles metrics collection from UC3's Redis instance
type RedisCollector struct {
	redisClient  *metrics.RedisClient
	metricsStore *metrics.MetricsStore
}

// NewRedisCollector creates a new Redis collector using UC3's connection pattern
func NewRedisCollector() (*RedisCollector, error) {
	// Use UC3's Redis connection method
	redisClient, err := metrics.NewRedisConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create metrics store with 7-day TTL (same as UC3)
	metricsStore := metrics.NewMetricsStore(redisClient, 168*time.Hour)

	return &RedisCollector{
		redisClient:  redisClient,
		metricsStore: metricsStore,
	}, nil
}

// TestConnection verifies Redis connectivity
func (rc *RedisCollector) TestConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rc.redisClient.Ping(ctx)
	if err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// GetActiveBatches scans for active batch IDs in Redis
func (rc *RedisCollector) GetActiveBatches(ctx context.Context) ([]string, error) {
	return rc.metricsStore.GetAllBatchIDs(ctx)
}

// GetJobMetrics retrieves individual job metrics for a batch
func (rc *RedisCollector) GetJobMetrics(ctx context.Context, batchID string) ([]JobMetrics, error) {
	// Use UC3's existing method to get batch jobs
	ucMetrics, err := rc.metricsStore.GetBatchJobes(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch jobs: %w", err)
	}

	// Convert UC3 metrics to our format
	jobMetrics := make([]JobMetrics, len(ucMetrics))
	for i, ucMetric := range ucMetrics {
		jobMetrics[i] = JobMetrics{
			BatchID:            ucMetric.BatchID,
			JobID:              ucMetric.JobID,
			QueueStartTime:     ucMetric.QueueStartTime,
			QueueReceiveTime:   ucMetric.QueueReceiveTime,
			JobEndTime:         ucMetric.JobEndTime,
			QueueDuration:      ucMetric.QueueDuration,
			ProcessingDuration: ucMetric.ProcessingDuration,
			TotalDuration:      ucMetric.TotalDuration,
			IsComplete:         ucMetric.IsComplete,
			JobSizeMB:          ucMetric.JobSizeMB,
			QueueAheadLength:   ucMetric.JobQueueAheadLength,
		}
	}

	return jobMetrics, nil
}

// GetSystemSnapshot captures current system state
func (rc *RedisCollector) GetSystemSnapshot(ctx context.Context, processorCount int) (*SystemSnapshot, error) {
	activeBatches, err := rc.GetActiveBatches(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active batches: %w", err)
	}

	var allJobMetrics []JobMetrics
	for _, batchID := range activeBatches {
		jobMetrics, err := rc.GetJobMetrics(ctx, batchID)
		if err != nil {
			log.Printf("Warning: failed to get metrics for batch %s: %v", batchID, err)
			continue
		}
		allJobMetrics = append(allJobMetrics, jobMetrics...)
	}

	return &SystemSnapshot{
		Timestamp:      time.Now(),
		ProcessorCount: processorCount,
		ActiveBatches:  activeBatches,
		JobMetrics:     allJobMetrics,
	}, nil
}

// CalculateLiveMetrics aggregates job metrics into live system metrics
func (rc *RedisCollector) CalculateLiveMetrics(snapshot *SystemSnapshot, experimentID string) *LiveMetrics {
	activeJobs := 0
	completedJobs := 0
	totalProcessingTime := 0.0
	completedCount := 0

	for _, job := range snapshot.JobMetrics {
		if job.IsComplete {
			completedJobs++
			totalProcessingTime += job.ProcessingDuration
			completedCount++
		} else {
			activeJobs++
		}
	}

	avgProcessingTime := 0.0
	if completedCount > 0 {
		avgProcessingTime = totalProcessingTime / float64(completedCount)
	}

	// Calculate throughput (jobs per minute) - rough estimate
	throughput := 0.0
	if completedCount > 0 && avgProcessingTime > 0 {
		throughput = 60.0 / avgProcessingTime // jobs per minute per processor
		throughput *= float64(snapshot.ProcessorCount)
	}

	return &LiveMetrics{
		Timestamp:         snapshot.Timestamp,
		ProcessorCount:    snapshot.ProcessorCount,
		ExperimentID:      experimentID,
		ActiveJobs:        activeJobs,
		CompletedJobs:     completedJobs,
		QueueDepth:        activeJobs, // Approximation
		AvgProcessingTime: avgProcessingTime,
		Throughput:        throughput,
	}
}

// GetBatchSummary gets aggregated metrics for a specific batch using UC3's aggregation
func (rc *RedisCollector) GetBatchSummary(ctx context.Context, batchID string, processorCount int, experimentID string) (*BatchMetrics, error) {
	// Use UC3's existing batch aggregation
	ucSummary, err := rc.metricsStore.AggregateBatchMetrics(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate batch metrics: %w", err)
	}

	// Calculate throughput
	throughput := 0.0
	if ucSummary.TotalBatchDuration > 0 {
		throughput = float64(ucSummary.CompleteJobCount) / ucSummary.TotalBatchDuration.Minutes()
	}

	// Calculate total size
	jobs, err := rc.GetJobMetrics(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job metrics for size calculation: %w", err)
	}

	totalSizeMB := 0.0
	for _, job := range jobs {
		totalSizeMB += job.JobSizeMB
	}

	return &BatchMetrics{
		ExperimentID:       experimentID,
		BatchID:            batchID,
		ProcessorCount:     processorCount,
		JobCount:           ucSummary.JobCount,
		CompleteJobCount:   ucSummary.CompleteJobCount,
		AvgQueueTime:       ucSummary.AvgJobQueueTime,
		AvgProcessingTime:  ucSummary.AvgJobProcessingTime,
		TotalBatchDuration: ucSummary.TotalBatchDuration,
		Throughput:         throughput,
		TotalSizeMB:        totalSizeMB,
	}, nil
}

// ExportJobMetricsCSV exports batch summary data to CSV for a dataset (all related batches)
func (rc *RedisCollector) ExportJobMetricsCSV(ctx context.Context, datasetName string, outputPath string) error {
	// Get all batch summaries for the dataset
	summaries, err := rc.findBatchSummariesForDataset(ctx, datasetName)
	if err != nil {
		return fmt.Errorf("failed to get batch summaries for dataset: %w", err)
	}

	if len(summaries) == 0 {
		return fmt.Errorf("no batch summaries found for dataset %s", datasetName)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
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

	log.Printf("Exported %d batch summaries for dataset %s to %s", len(summaries), datasetName, outputPath)
	return nil
}

// ExportTrainingDataPoint exports a single training data point to CSV for a dataset (appends to existing file)
func (rc *RedisCollector) ExportTrainingDataPoint(ctx context.Context, datasetName string, processorCount int, outputPath string) error {
	// Get batch summaries for the dataset to calculate aggregated statistics
	summaries, err := rc.findBatchSummariesForDataset(ctx, datasetName)
	if err != nil {
		return fmt.Errorf("failed to get batch summaries for dataset: %w", err)
	}

	if len(summaries) == 0 {
		return fmt.Errorf("no batch summaries found for dataset %s", datasetName)
	}

	// Calculate aggregated metrics from all batch summaries
	totalJobs := 0
	totalCompleteJobs := 0
	totalQueueTime := 0.0
	totalProcessingTime := 0.0
	validBatches := 0

	for _, summary := range summaries {
		if summary.CompleteJobCount > 0 {
			totalJobs += summary.JobCount
			totalCompleteJobs += summary.CompleteJobCount
			// Weight the averages by the number of complete jobs in each batch
			totalQueueTime += summary.AvgJobQueueTime.Seconds() * float64(summary.CompleteJobCount)
			totalProcessingTime += summary.AvgJobProcessingTime.Seconds() * float64(summary.CompleteJobCount)
			validBatches++
		}
	}

	if validBatches == 0 || totalCompleteJobs == 0 {
		return fmt.Errorf("no completed jobs found in summaries for dataset %s", datasetName)
	}

	// Calculate weighted averages
	avgQueueTime := totalQueueTime / float64(totalCompleteJobs)
	avgProcessingTime := totalProcessingTime / float64(totalCompleteJobs)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if file exists to determine if we need to write header
	fileExists := false
	if _, err := os.Stat(outputPath); err == nil {
		fileExists = true
	}

	// Open file for appending
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header if file is new (only metrics available in summaries)
	if !fileExists {
		header := []string{
			"num_processors",
			"avg_queue_job_time",
			"avg_processed_time",
			"avg_job_total_time",
			"total_jobs",
			"complete_jobs",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Write training data point (only available metrics)
	record := []string{
		fmt.Sprintf("%d", processorCount),
		fmt.Sprintf("%.6f", avgQueueTime),
		fmt.Sprintf("%.6f", avgProcessingTime),
		fmt.Sprintf("%.6f", avgQueueTime+avgProcessingTime),
		fmt.Sprintf("%d", totalJobs),
		fmt.Sprintf("%d", totalCompleteJobs),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write training data record: %w", err)
	}

	log.Printf("Exported training data point: processors=%d, dataset=%s (%d batches, %d jobs) to %s",
		processorCount, datasetName, len(summaries), totalCompleteJobs, outputPath)
	return nil
}

// WaitForBatchCompletion waits for a dataset's processing to complete by monitoring UC3's aggregated summaries
func (rc *RedisCollector) WaitForBatchCompletion(ctx context.Context, datasetName string, timeout time.Duration) error {
	log.Printf("Waiting for dataset completion using aggregated summaries: %s (timeout: %v)", datasetName, timeout)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	var lastJobCount int
	var lastCompleteCount int
	progressStaleCount := 0
	completionStableCount := 0
	maxStaleCount := 10   // If no progress for 5 minutes (10 * 30s), consider it stuck
	minStableCount := 3   // Need 3 consecutive stable completion checks (1.5 minutes)
	minExpectedJobs := 10 // Minimum expected jobs for dataset validation (adjust based on dataset)

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for dataset %s to complete", datasetName)

		case <-ticker.C:
			// Find all batch summaries for this dataset (ignore timestamps since UC3 reuses batch IDs)
			summaries, err := rc.findBatchSummariesForDataset(ctx, datasetName)
			if err != nil {
				log.Printf("Error finding batch summaries: %v", err)
				continue
			}

			if len(summaries) == 0 {
				log.Printf("No batch summaries found for dataset %s (waiting for aggregation...)", datasetName)
				continue
			}

			log.Printf("Found %d batch summaries matching dataset %s", len(summaries), datasetName)

			// Check completion status across all batches
			totalJobs := 0
			completedJobs := 0
			allComplete := true

			for batchID, summary := range summaries {
				log.Printf("Batch %s: %d/%d jobs complete", batchID, summary.CompleteJobCount, summary.JobCount)
				totalJobs += summary.JobCount
				completedJobs += summary.CompleteJobCount

				if summary.CompleteJobCount < summary.JobCount {
					allComplete = false
				}
			}

			log.Printf("Dataset %s overall: %d/%d jobs complete across %d batches",
				datasetName, completedJobs, totalJobs, len(summaries))

			// Track progress to detect stalled processing
			if totalJobs == lastJobCount && completedJobs == lastCompleteCount {
				progressStaleCount++
				if progressStaleCount >= maxStaleCount {
					log.Printf("Warning: No progress detected for %d cycles, but continuing to wait...", progressStaleCount)
				}
			} else {
				progressStaleCount = 0 // Reset counter on progress
				lastJobCount = totalJobs
				lastCompleteCount = completedJobs
				completionStableCount = 0 // Reset completion stability on progress
			}

			// Check for completion with stability requirement and job count validation
			if allComplete && totalJobs > 0 {
				// Validate job count is reasonable for dataset
				if totalJobs < minExpectedJobs {
					log.Printf("WARNING: Only %d jobs found for dataset %s - expected at least %d. UC3 may have data corruption.",
						totalJobs, datasetName, minExpectedJobs)
					log.Printf("This suggests UC3's aggregation service is mixing old/new data. Continuing to wait for more jobs...")
					completionStableCount = 0 // Reset stability
					continue
				}

				completionStableCount++
				log.Printf("Dataset %s appears complete (%d/%d jobs) - stability check %d/%d",
					datasetName, completedJobs, totalJobs, completionStableCount, minStableCount)

				if completionStableCount >= minStableCount {
					log.Printf("Dataset %s processing completed: %d jobs finished across %d batches (stable for %d checks)",
						datasetName, completedJobs, len(summaries), completionStableCount)

					// Brief buffer time for any final aggregation updates
					log.Printf("Waiting buffer period (30 seconds) for final aggregation updates...")
					time.Sleep(30 * time.Second)

					return nil
				}
			} else {
				completionStableCount = 0 // Reset if not complete
				// If we have jobs but not complete, continue waiting
				if totalJobs > 0 {
					log.Printf("Dataset %s processing in progress: %d/%d jobs complete", datasetName, completedJobs, totalJobs)
				}
			}
		}
	}
}

// findBatchSummariesForDataset finds all batch summaries that belong to a specific dataset
func (rc *RedisCollector) findBatchSummariesForDataset(ctx context.Context, datasetName string) (map[string]*metrics.BatchSummary, error) {
	// Get all summary keys from Redis using the pattern: metrics:summary:*
	pattern := "metrics:summary:*"
	keys, err := rc.metricsStore.RedisClient().Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary keys: %w", err)
	}

	summaries := make(map[string]*metrics.BatchSummary)
	matchingBatches := 0

	for _, key := range keys {
		// Extract batch ID from key: metrics:summary:{batchID}
		parts := strings.Split(key, ":")
		if len(parts) < 3 {
			continue
		}
		batchID := parts[2]

		// Filter batches that contain our dataset name
		if strings.Contains(batchID, datasetName) {
			matchingBatches++
			summary, err := rc.metricsStore.GetBatchSummary(ctx, batchID)
			if err != nil {
				log.Printf("Warning: failed to get summary for batch %s: %v", batchID, err)
				continue
			}
			if summary != nil {
				summaries[batchID] = summary
			}
		}
	}

	if matchingBatches > 0 {
		log.Printf("Found %d batch summaries matching dataset %s", len(summaries), datasetName)
	}

	return summaries, nil
}

// Close closes the Redis connection
func (rc *RedisCollector) Close() error {
	return rc.redisClient.Close()
}
