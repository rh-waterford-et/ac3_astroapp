package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
)

// RedisCollector handles metrics collection from UC3's Redis instance
// Note: Completion detection has been moved to S3Monitor for reliability.
// This collector now focuses solely on metrics collection and CSV export.
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
			// Use UC3's method directly (we're no longer relying on complete_job_count)
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

// GetBatchSummariesForDataset is a public wrapper for findBatchSummariesForDataset
func (rc *RedisCollector) GetBatchSummariesForDataset(ctx context.Context, datasetName string) (map[string]*metrics.BatchSummary, error) {
	return rc.findBatchSummariesForDataset(ctx, datasetName)
}

// GetJobRecordsForDataset gets all individual job records for a dataset from export keys
func (rc *RedisCollector) GetJobRecordsForDataset(ctx context.Context, datasetName string) ([]*metrics.MetricRecord, error) {
	// Get all export keys for this dataset using the pattern: metrics/export/{datasetName}:*
	pattern := fmt.Sprintf("metrics/export/%s:*", datasetName)
	keys, err := rc.metricsStore.RedisClient().Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get export keys: %w", err)
	}

	var allJobRecords []*metrics.MetricRecord

	for _, key := range keys {
		// Get the exported metrics data
		data, err := rc.metricsStore.RedisClient().Get(ctx, key)
		if err != nil {
			log.Printf("Warning: failed to get export data for key %s: %v", key, err)
			continue
		}

		// Parse the individual job records from the export text
		jobRecords := rc.parseExportedJobRecords(data, datasetName)
		allJobRecords = append(allJobRecords, jobRecords...)
	}

	log.Printf("Found %d job records for dataset %s from %d export keys", len(allJobRecords), datasetName, len(keys))
	return allJobRecords, nil
}

// parseExportedJobRecords parses the exported metrics text to extract individual job records
func (rc *RedisCollector) parseExportedJobRecords(exportData, datasetName string) []*metrics.MetricRecord {
	var jobRecords []*metrics.MetricRecord

	lines := strings.Split(exportData, "\n")
	var currentJob *metrics.MetricRecord

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of a new job
		if strings.HasPrefix(line, "Job ") {
			if currentJob != nil {
				// Save previous job if it's complete
				if currentJob.IsComplete {
					jobRecords = append(jobRecords, currentJob)
				}
			}
			currentJob = &metrics.MetricRecord{
				BatchID: datasetName,
			}
		}

		if currentJob == nil {
			continue
		}

		// Parse job fields
		if strings.HasPrefix(line, "Job ID: ") {
			currentJob.JobID = strings.TrimPrefix(line, "Job ID: ")
		} else if strings.HasPrefix(line, "Status: ") {
			status := strings.TrimPrefix(line, "Status: ")
			currentJob.IsComplete = (status == "COMPLETE")
		} else if strings.HasPrefix(line, "queue_start_time: ") {
			timeStr := strings.TrimPrefix(line, "queue_start_time: ")
			if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
				currentJob.QueueStartTime = t
			}
		} else if strings.HasPrefix(line, "queue_receive_time: ") {
			timeStr := strings.TrimPrefix(line, "queue_receive_time: ")
			if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
				currentJob.QueueReceiveTime = t
			}
		} else if strings.HasPrefix(line, "job_end_time: ") {
			timeStr := strings.TrimPrefix(line, "job_end_time: ")
			if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
				currentJob.JobEndTime = t
			}
		} else if strings.HasPrefix(line, "queue_duration: ") {
			durStr := strings.TrimPrefix(line, "queue_duration: ")
			if f, err := strconv.ParseFloat(durStr, 64); err == nil {
				currentJob.QueueDuration = f
			}
		} else if strings.HasPrefix(line, "processing_duration: ") {
			durStr := strings.TrimPrefix(line, "processing_duration: ")
			if f, err := strconv.ParseFloat(durStr, 64); err == nil {
				currentJob.ProcessingDuration = f
			}
		} else if strings.HasPrefix(line, "total_duration: ") {
			durStr := strings.TrimPrefix(line, "total_duration: ")
			if f, err := strconv.ParseFloat(durStr, 64); err == nil {
				currentJob.TotalDuration = f
			}
		} else if strings.HasPrefix(line, "job_size_mb: ") {
			sizeStr := strings.TrimPrefix(line, "job_size_mb: ")
			if f, err := strconv.ParseFloat(sizeStr, 64); err == nil {
				currentJob.JobSizeMB = f
			}
		} else if strings.HasPrefix(line, "job_queue_ahead_length: ") {
			queueStr := strings.TrimPrefix(line, "job_queue_ahead_length: ")
			if i, err := strconv.Atoi(queueStr); err == nil {
				currentJob.JobQueueAheadLength = i
			}
		}
	}

	// Don't forget the last job
	if currentJob != nil && currentJob.IsComplete {
		jobRecords = append(jobRecords, currentJob)
	}

	return jobRecords
}

// getJobRecord retrieves a single job record from Redis (now unused)
func (rc *RedisCollector) getJobRecord(ctx context.Context, key string) (*metrics.MetricRecord, error) {
	// This method is no longer used since we read from export keys
	return nil, fmt.Errorf("individual job keys not available, use export keys instead")
}

// AppendJobRecordsToCSV appends new job records to an existing CSV file
func (rc *RedisCollector) AppendJobRecordsToCSV(ctx context.Context, datasetName, csvPath string, seenJobIDs map[string]bool, processorCount int) (int, error) {
	jobRecords, err := rc.GetJobRecordsForDataset(ctx, datasetName)
	if err != nil {
		return 0, fmt.Errorf("failed to get job records: %w", err)
	}

	// Filter out jobs we've already seen
	var newRecords []*metrics.MetricRecord
	for _, record := range jobRecords {
		if !seenJobIDs[record.JobID] {
			newRecords = append(newRecords, record)
			seenJobIDs[record.JobID] = true
		}
	}

	if len(newRecords) == 0 {
		return 0, nil // No new records
	}

	// Sort new records by queue start time before appending
	sort.Slice(newRecords, func(i, j int) bool {
		return newRecords[i].QueueStartTime.Before(newRecords[j].QueueStartTime)
	})

	// Open file in append mode
	file, err := os.OpenFile(csvPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write new records
	for _, record := range newRecords {
		row := []string{
			record.BatchID,
			record.JobID,
			record.QueueStartTime.Format(time.RFC3339Nano),
			record.QueueReceiveTime.Format(time.RFC3339Nano),
			record.JobEndTime.Format(time.RFC3339Nano),
			fmt.Sprintf("%.6f", record.QueueDuration),
			fmt.Sprintf("%.6f", record.ProcessingDuration),
			fmt.Sprintf("%.6f", record.TotalDuration),
			fmt.Sprintf("%.2f", record.JobSizeMB),
			fmt.Sprintf("%d", processorCount),
			fmt.Sprintf("%d", record.JobQueueAheadLength),
		}

		if err := writer.Write(row); err != nil {
			return len(newRecords), fmt.Errorf("failed to write record: %w", err)
		}
	}

	log.Printf("Appended %d new job records to %s", len(newRecords), csvPath)
	return len(newRecords), nil
}

// SortJobRecordsCSV sorts an entire job records CSV file by queue_start_time
func (rc *RedisCollector) SortJobRecordsCSV(csvPath string) error {
	// Read the entire CSV file
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	file.Close()

	if err != nil {
		return fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) <= 1 {
		return nil // No data rows to sort (only header or empty)
	}

	// Separate header and data
	header := records[0]
	dataRows := records[1:]

	// Parse queue_start_time (column index 2) and sort
	sort.Slice(dataRows, func(i, j int) bool {
		timeI, errI := time.Parse(time.RFC3339Nano, dataRows[i][2])
		timeJ, errJ := time.Parse(time.RFC3339Nano, dataRows[j][2])

		if errI != nil || errJ != nil {
			return false // Keep original order if parse fails
		}

		return timeI.Before(timeJ)
	})

	// Write sorted data back to file
	outFile, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create sorted CSV file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	// Write header
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write sorted data rows
	for _, row := range dataRows {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	log.Printf("Sorted %d job records in %s by queue_start_time", len(dataRows), csvPath)
	return nil
}

// Close closes the Redis connection
func (rc *RedisCollector) Close() error {
	return rc.redisClient.Close()
}
