package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// ExportJobMetricsCSV exports individual job metrics to CSV in the requested format
func (rc *RedisCollector) ExportJobMetricsCSV(ctx context.Context, batchID string, outputPath string) error {
	// Get all job metrics for the batch
	jobMetrics, err := rc.GetJobMetrics(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get job metrics: %w", err)
	}

	if len(jobMetrics) == 0 {
		return fmt.Errorf("no job metrics found for batch %s", batchID)
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

	// Write header
	header := []string{
		"job_id",
		"queue_start_time",
		"queue_receive_time",
		"job_queue_time",
		"job_end_time",
		"job_processing_time",
		"total_duration",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write job records
	for _, job := range jobMetrics {
		record := []string{
			job.JobID,
			job.QueueStartTime.Format(time.RFC3339Nano),
			job.QueueReceiveTime.Format(time.RFC3339Nano),
			fmt.Sprintf("%.6f", job.QueueDuration),
			job.JobEndTime.Format(time.RFC3339Nano),
			fmt.Sprintf("%.6f", job.ProcessingDuration),
			fmt.Sprintf("%.6f", job.TotalDuration),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write job record: %w", err)
		}
	}

	log.Printf("✅ Exported %d job records to %s", len(jobMetrics), outputPath)
	return nil
}

// ExportTrainingDataPoint exports a single training data point to CSV (appends to existing file)
func (rc *RedisCollector) ExportTrainingDataPoint(ctx context.Context, batchID string, processorCount int, outputPath string) error {
	// Get batch summary from UC3's aggregation
	ucSummary, err := rc.metricsStore.AggregateBatchMetrics(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to aggregate batch metrics: %w", err)
	}

	// Get individual job metrics to calculate average job size
	jobMetrics, err := rc.GetJobMetrics(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get job metrics: %w", err)
	}

	if len(jobMetrics) == 0 {
		return fmt.Errorf("no job metrics found for batch %s", batchID)
	}

	// Calculate average job size and queue length
	totalSizeMB := 0.0
	totalQueueAhead := 0
	validSizeCount := 0
	validQueueCount := 0

	for _, job := range jobMetrics {
		if job.JobSizeMB > 0 {
			totalSizeMB += job.JobSizeMB
			validSizeCount++
		}
		if job.QueueAheadLength >= 0 {
			totalQueueAhead += job.QueueAheadLength
			validQueueCount++
		}
	}

	avgJobSizeMB := 0.0
	if validSizeCount > 0 {
		avgJobSizeMB = totalSizeMB / float64(validSizeCount)
	}

	avgQueueAhead := 0
	if validQueueCount > 0 {
		avgQueueAhead = totalQueueAhead / validQueueCount
	}

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

	// Write header if file is new
	if !fileExists {
		header := []string{
			"num_processors",
			"avg_job_size_mb",
			"job_queue_length_ahead",
			"avg_queue_job_time",
			"avg_processed_time",
			"avg_job_total_time",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Write training data point
	record := []string{
		fmt.Sprintf("%d", processorCount),
		fmt.Sprintf("%.2f", avgJobSizeMB),
		fmt.Sprintf("%d", avgQueueAhead),
		fmt.Sprintf("%.6f", ucSummary.AvgJobQueueTime.Seconds()),
		fmt.Sprintf("%.6f", ucSummary.AvgJobProcessingTime.Seconds()),
		fmt.Sprintf("%.6f", (ucSummary.AvgJobQueueTime + ucSummary.AvgJobProcessingTime).Seconds()),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write training data record: %w", err)
	}

	log.Printf("✅ Exported training data point: processors=%d, batch=%s to %s", processorCount, batchID, outputPath)
	return nil
}

// WaitForBatchCompletion waits for a batch to complete processing with timeout
func (rc *RedisCollector) WaitForBatchCompletion(ctx context.Context, batchID string, timeout time.Duration) error {
	log.Printf("⏳ Waiting for batch completion: %s (timeout: %v)", batchID, timeout)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for batch %s to complete", batchID)

		case <-ticker.C:
			// Check if batch summary exists and all jobs are complete
			summary, err := rc.metricsStore.GetBatchSummary(ctx, batchID)
			if err != nil {
				log.Printf("⏳ Batch %s not yet aggregated, continuing to wait...", batchID)
				continue
			}

			if summary == nil {
				log.Printf("⏳ Batch %s summary not found, continuing to wait...", batchID)
				continue
			}

			// Check if ALL jobs in the batch are complete
			if summary.CompleteJobCount > 0 && summary.CompleteJobCount == summary.JobCount {
				log.Printf("✅ Batch %s fully completed: %d/%d jobs finished",
					batchID, summary.CompleteJobCount, summary.JobCount)

				// Wait additional buffer time for aggregation to stabilize
				log.Printf("⏳ Waiting buffer period (2 minutes) for metrics to stabilize...")
				time.Sleep(2 * time.Minute)

				return nil
			}

			log.Printf("⏳ Batch %s still processing: %d/%d jobs complete",
				batchID, summary.CompleteJobCount, summary.JobCount)
		}
	}
}

// Close closes the Redis connection
func (rc *RedisCollector) Close() error {
	return rc.redisClient.Close()
}
