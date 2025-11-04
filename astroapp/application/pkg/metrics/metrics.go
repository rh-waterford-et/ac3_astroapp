package metrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type MetricRecord struct {
	BatchID          string    `json:"batch_id"`
	JobID            string    `json:"job_id"`
	QueueStartTime   time.Time `json:"queue_start_time"`   // Time of sending job to queue
	QueueReceiveTime time.Time `json:"queue_receive_time"` // Time of receiving job from queue
	JobEndTime       time.Time `json:"job_end_time"`       // Time of job processing completion

	QueueDuration      float64 `json:"queue_duration"`      // Duration of job in queue
	ProcessingDuration float64 `json:"processing_duration"` // Duration of job processing
	TotalDuration      float64 `json:"total_duration"`      // Total duration of job processing

	IsComplete bool `json:"is_complete"` // Whether the job has been processed

	JobSizeMB float64 `json:"job_size_mb"` // Size of job in MB

	JobQueueAheadLength int `json:"job_queue_ahead_length"`
}

type MetricsStore struct {
	redis     *RedisClient
	ttl       time.Duration
	keyPrefix string
}

func (ms *MetricsStore) RedisClient() *RedisClient {
	return ms.redis
}

func NewMetricsStore(redis *RedisClient, ttl time.Duration) *MetricsStore {
	return &MetricsStore{
		redis:     redis,
		ttl:       ttl,
		keyPrefix: "metrics",
	}
}

func (ms *MetricsStore) WithKeyPrefix(prefix string) *MetricsStore {
	ms.keyPrefix = prefix
	return ms
}

func (ms *MetricsStore) GetJobKey(batchID, jobID string) string {
	return fmt.Sprintf("%s:%s:%s", ms.keyPrefix, batchID, jobID)
}

func (ms *MetricsStore) getBatchPattern(batchID string) string {
	return fmt.Sprintf("%s:%s:*", ms.keyPrefix, batchID)
}

func (ms *MetricsStore) calculateDurations(metric *MetricRecord) {
	metric.IsComplete = false

	// Queue duration
	if !metric.QueueStartTime.IsZero() && !metric.QueueReceiveTime.IsZero() {
		metric.QueueDuration = metric.QueueReceiveTime.Sub(metric.QueueStartTime).Seconds()
	}

	// Processing duration
	if !metric.QueueReceiveTime.IsZero() && !metric.JobEndTime.IsZero() {
		metric.ProcessingDuration = metric.JobEndTime.Sub(metric.QueueReceiveTime).Seconds()
	}

	// Total duration
	if !metric.QueueStartTime.IsZero() && !metric.JobEndTime.IsZero() {
		metric.TotalDuration = metric.JobEndTime.Sub(metric.QueueStartTime).Seconds()
		metric.IsComplete = true
	}
}

func (ms *MetricsStore) GetMetric(ctx context.Context, batchID, jobID string) (*MetricRecord, error) {
	key := ms.GetJobKey(batchID, jobID)
	data, err := ms.redis.HGetAll(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	return ms.parseMetric(data)
}

func (ms *MetricsStore) parseMetric(data map[string]string) (*MetricRecord, error) {
	var result MetricRecord
	var err error

	result.BatchID = data["batch_id"]
	result.JobID = data["job_id"]

	if data["queue_start_time"] != "" {
		if result.QueueStartTime, err = time.Parse(time.RFC3339Nano, data["queue_start_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse queue_start_time: %w", err)
		}
	}

	if data["queue_receive_time"] != "" {
		if result.QueueReceiveTime, err = time.Parse(time.RFC3339Nano, data["queue_receive_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse queue_receive_time: %w", err)
		}
	}

	if data["job_end_time"] != "" {
		if result.JobEndTime, err = time.Parse(time.RFC3339Nano, data["job_end_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse job_end_time: %w", err)
		}
	}

	if data["queue_duration"] != "" {
		if _, err := fmt.Sscanf(data["queue_duration"], "%f", &result.QueueDuration); err != nil {
			log.Printf("WARNING: failed to parse queue_duration: %v", err)
		}
	}

	if data["processing_duration"] != "" {
		if _, err := fmt.Sscanf(data["processing_duration"], "%f", &result.ProcessingDuration); err != nil {
			log.Printf("WARNING: failed to parse processing_duration: %v", err)
		}
	}

	if data["total_duration"] != "" {
		if _, err := fmt.Sscanf(data["total_duration"], "%f", &result.TotalDuration); err != nil {
			log.Printf("WARNING: failed to parse total_duration: %v", err)
		}
	}

	if data["job_size_mb"] != "" {
		if _, err := fmt.Sscanf(data["job_size_mb"], "%f", &result.JobSizeMB); err != nil {
			log.Printf("WARNING: failed to parse job_size_mb: %v", err)
		}
	}

	if data["job_queue_ahead_length"] != "" {
		if _, err := fmt.Sscanf(data["job_queue_ahead_length"], "%d", &result.JobQueueAheadLength); err != nil {
			log.Printf("WARNING: failed to parse job_queue_ahead_length: %v", err)
		}
	}

	// If durations were not saved, recalculate them
	if result.QueueDuration == 0 || result.ProcessingDuration == 0 || result.TotalDuration == 0 {
		ms.calculateDurations(&result)
	}

	return &result, nil
}

func (ms *MetricsStore) GetBatchJobes(ctx context.Context, batchID string) ([]*MetricRecord, error) {
	// Get jobs from active metrics table (used by aggregation service)
	pattern := ms.getBatchPattern(batchID)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get job keys: %w", err)
	}

	var jobes []*MetricRecord
	for _, key := range keys {
		data, err := ms.redis.HGetAll(ctx, key)
		if err != nil {
			continue
		}

		job, err := ms.parseMetric(data)
		if err == nil {
			jobes = append(jobes, job)
		}
	}

	sort.Slice(jobes, func(i, j int) bool {
		return jobes[i].QueueStartTime.Before(jobes[j].QueueStartTime)
	})

	return jobes, nil
}

// GetCompletedBatchJobes retrieves only completed/archived jobs for a batch (used by Prometheus)
func (ms *MetricsStore) GetCompletedBatchJobes(ctx context.Context, batchID string) ([]*MetricRecord, error) {
	// Only retrieve from completed metrics table
	pattern := fmt.Sprintf("%s:completed:%s:*", ms.keyPrefix, batchID)
	log.Printf("DEBUG: GetCompletedBatchJobes searching with pattern: %s", pattern)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed job keys: %w", err)
	}

	log.Printf("DEBUG: GetCompletedBatchJobes found %d keys for batch %s", len(keys), batchID)

	var jobes []*MetricRecord
	for _, key := range keys {
		data, err := ms.redis.HGetAll(ctx, key)
		if err != nil {
			log.Printf("DEBUG: Failed to get data for key %s: %v", key, err)
			continue
		}

		job, err := ms.parseMetric(data)
		if err == nil {
			log.Printf("DEBUG: Parsed job %s from completed table, IsComplete=%v", job.JobID, job.IsComplete)
			jobes = append(jobes, job)
		} else {
			log.Printf("DEBUG: Failed to parse metric from key %s: %v", key, err)
		}
	}

	sort.Slice(jobes, func(i, j int) bool {
		return jobes[i].QueueStartTime.Before(jobes[j].QueueStartTime)
	})

	log.Printf("DEBUG: GetCompletedBatchJobes returning %d jobs for batch %s", len(jobes), batchID)
	return jobes, nil
}

func (ms *MetricsStore) UpdateMetricField(ctx context.Context, batchID, field, jobID string, value interface{}) error {
	key := ms.GetJobKey(batchID, jobID)

	// Always set basic fields
	err := ms.redis.HSet(ctx, key, "batch_id", batchID, "job_id", jobID)
	if err != nil {
		return fmt.Errorf("failed to set basic fields: %w", err)
	}

	// Handle different value types
	switch v := value.(type) {
	case time.Time:
		//log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %s", key, field, v.Format(time.RFC3339Nano))

		// Read current value to enforce monotonic policy for time fields
		currentMap, err := ms.redis.HGetAll(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to read current metric fields: %w", err)
		}
		currentRaw := currentMap[field]
		if currentRaw != "" {
			if t, parseErr := time.Parse(time.RFC3339Nano, currentRaw); parseErr == nil {
				switch field {
				case "queue_start_time", "queue_receive_time":
					// Keep earliest timestamp
					if !v.Before(t) {
						// Existing value is earlier or equal; keep it
						if ms.ttl > 0 {
							_ = ms.redis.Expire(ctx, key, ms.ttl)
						}
						return nil
					}
				case "job_end_time":
					// Keep latest timestamp
					if !v.After(t) {
						// Existing value is later or equal; keep it
						if ms.ttl > 0 {
							_ = ms.redis.Expire(ctx, key, ms.ttl)
						}
						return nil
					}
				}
			}
		}

		// Set the time field
		err = ms.redis.HSet(ctx, key, field, v.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("failed to update time field: %w", err)
		}

	case float64:
		//log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %.2f", key, field, v)

		// For job_size_mb field, just set the value directly
		if field == "job_size_mb" {
			err = ms.redis.HSet(ctx, key, field, v)
			if err != nil {
				return fmt.Errorf("failed to update size field: %w", err)
			}
		} else {
			return fmt.Errorf("unsupported field type for float64 value: %s", field)
		}

	case int:
		//log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %d", key, field, v)

		// For job_queue_ahead_length field, set the integer value
		if field == "job_queue_ahead_length" {
			err = ms.redis.HSet(ctx, key, field, v)
			if err != nil {
				return fmt.Errorf("failed to update queue ahead length field: %w", err)
			}
		} else {
			return fmt.Errorf("unsupported field type for int value: %s", field)
		}

	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}

	if ms.ttl > 0 {
		err = ms.redis.Expire(ctx, key, ms.ttl)
		if err != nil {
			return fmt.Errorf("failed to set TTL: %w", err)
		}
	}

	return nil
}

func (ms *MetricsStore) RecordMetricsJob(ctx context.Context, metrics []*MetricRecord) error {
	pipe := ms.redis.Pipeline()

	for _, metric := range metrics {
		key := ms.GetJobKey(metric.BatchID, metric.JobID)
		values := map[string]interface{}{
			"batch_id":           metric.BatchID,
			"job_id":             metric.JobID,
			"queue_start_time":   metric.QueueStartTime.Format(time.RFC3339Nano),
			"queue_receive_time": metric.QueueReceiveTime.Format(time.RFC3339Nano),
			"job_end_time":       metric.JobEndTime.Format(time.RFC3339Nano),
		}

		pipe.HSet(ctx, key, values)
		if ms.ttl > 0 {
			pipe.Expire(ctx, key, ms.ttl)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ExportBatchJobesToS3 writes a consolidated text file with all job metrics for a batch
// and archives completed metrics to metrics:completed:<batchID> table
func (ms *MetricsStore) ExportBatchJobesToS3(ctx context.Context, batchID string, timeParam string, onlyComplete bool) error {
	// Gather job records
	jobes, err := ms.GetBatchJobes(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get batch jobs for export: %w", err)
	}
	if len(jobes) == 0 {
		return nil
	}

	var filteredJobes []*MetricRecord
	var completedMetrics []*MetricRecord // For archiving
	var totalQueueTime, totalProcessingTime, totalJobSize float64
	completeCount := 0

	for _, job := range jobes {
		if !onlyComplete || job.IsComplete {
			filteredJobes = append(filteredJobes, job)
			if job.IsComplete {
				completedMetrics = append(completedMetrics, job) // Collect completed metrics for archiving
				totalQueueTime += job.QueueDuration
				totalProcessingTime += job.ProcessingDuration
				totalJobSize += job.JobSizeMB
				completeCount++
			}
		}
	}

	if len(filteredJobes) == 0 {
		log.Printf("No %s jobs found for batch %s", map[bool]string{true: "complete", false: ""}[onlyComplete], batchID)
		return nil
	}

	// Calculate averages
	var avgQueueTime, avgProcessingTime, avgJobSize float64
	if completeCount > 0 {
		avgQueueTime = totalQueueTime / float64(completeCount)
		avgProcessingTime = totalProcessingTime / float64(completeCount)
		avgJobSize = totalJobSize / float64(completeCount)
	}

	// Sort by QueueStartTime for stable output
	sort.Slice(filteredJobes, func(i, j int) bool {
		return filteredJobes[i].QueueStartTime.Before(filteredJobes[j].QueueStartTime)
	})

	// 1. First export to S3
	// Build text content
	var b strings.Builder
	b.WriteString(fmt.Sprintf("METRICS: %s (%s jobs)\n", batchID, map[bool]string{true: "COMPLETE", false: "ALL"}[onlyComplete]))

	for idx, rec := range filteredJobes {
		b.WriteString("----------------------------------------\n")
		b.WriteString(fmt.Sprintf("Job %d\n", idx+1))
		b.WriteString(fmt.Sprintf("Batch ID: %s\n", rec.BatchID))
		b.WriteString(fmt.Sprintf("Job ID: %s\n", rec.JobID))
		b.WriteString(fmt.Sprintf("Status: %s\n", map[bool]string{true: "COMPLETE", false: "INCOMPLETE"}[rec.IsComplete]))
		if rec.JobQueueAheadLength != 0 {
			b.WriteString(fmt.Sprintf("\njob_queue_ahead_length: %d\n", rec.JobQueueAheadLength))
		}

		if !rec.QueueStartTime.IsZero() {
			b.WriteString(fmt.Sprintf("queue_start_time: %s\n", rec.QueueStartTime.Format(time.RFC3339Nano)))
		}
		if !rec.QueueReceiveTime.IsZero() {
			b.WriteString(fmt.Sprintf("queue_receive_time: %s\n", rec.QueueReceiveTime.Format(time.RFC3339Nano)))
		}
		if rec.QueueDuration != 0 {
			b.WriteString(fmt.Sprintf("queue_duration: %f\n", rec.QueueDuration))
		}
		if !rec.JobEndTime.IsZero() {
			b.WriteString(fmt.Sprintf("job_end_time: %s\n", rec.JobEndTime.Format(time.RFC3339Nano)))
		}
		if rec.ProcessingDuration != 0 {
			b.WriteString(fmt.Sprintf("processing_duration: %f\n", rec.ProcessingDuration))
		}
		if rec.TotalDuration != 0 {
			b.WriteString(fmt.Sprintf("total_duration: %f\n", rec.TotalDuration))
		}
		if rec.JobSizeMB != 0 {
			b.WriteString(fmt.Sprintf("job_size_mb: %.2f\n", rec.JobSizeMB))
		}
	}

	// Add summary statistics at the end
	b.WriteString("----------------------------------------\n")
	b.WriteString("SUMMARY STATISTICS (COMPLETE jobs only):\n")
	b.WriteString("----------------------------------------\n")
	b.WriteString(fmt.Sprintf("Total complete jobs: %d\n", completeCount))
	b.WriteString(fmt.Sprintf("Average queue time: %.6f seconds\n", avgQueueTime))
	b.WriteString(fmt.Sprintf("Average processing time: %.6f seconds\n", avgProcessingTime))
	b.WriteString(fmt.Sprintf("Total queue time: %.6f seconds\n", totalQueueTime))
	b.WriteString(fmt.Sprintf("Total processing time: %.6f seconds\n", totalProcessingTime))
	b.WriteString(fmt.Sprintf("Total time: %.6f seconds\n", totalQueueTime+totalProcessingTime))
	b.WriteString(fmt.Sprintf("Average job size: %.6f MB\n", avgJobSize))
	b.WriteString(fmt.Sprintf("Total job size: %.6f MB\n", totalJobSize))
	content := []byte(b.String())

	// Upload to S3
	metricsPrefix := os.Getenv("METRICS")
	fileName := fmt.Sprintf("METRICS_%s_%s.txt", batchID, timeParam)
	s3Bucket := s3bucket.NewS3Bucket()
	err = s3Bucket.UploadFileToBucket(metricsPrefix, fileName, content)
	if err != nil {
		return fmt.Errorf("failed to upload metrics to S3: %w", err)
	}

	log.Printf("Successfully exported metrics for batch %s to s3://%s/%s/%s",
		batchID, s3Bucket.GetBucketName(), metricsPrefix, fileName)

	// 2. Then archive completed metrics
	if len(completedMetrics) > 0 {
		if err := ms.ArchiveCompletedMetrics(ctx, batchID, completedMetrics); err != nil {
			log.Printf("WARNING: failed to archive completed metrics for batch %s: %v", batchID, err)
			// Continue even if archiving fails
		} else {
			log.Printf("Successfully archived %d completed metrics to metrics:completed:%s", len(completedMetrics), batchID)
		}
	}

	log.Printf("Successfully exported %s metrics for batch %s (%d jobes, %d completed)",
		map[bool]string{true: "complete", false: "all"}[onlyComplete], batchID, len(filteredJobes), len(completedMetrics))

	return nil
}

func (ms *MetricsStore) ArchiveCompletedMetrics(ctx context.Context, batchID string, metrics []*MetricRecord) error {
	if len(metrics) == 0 {
		return nil
	}

	archiveKey := fmt.Sprintf("metrics:completed:%s", batchID)
	
	// Use pipeline for efficiency
	pipe := ms.redis.Pipeline()
	
	for _, metric := range metrics {
		jobArchiveKey := fmt.Sprintf("%s:%s", archiveKey, metric.JobID)
		
		data := map[string]interface{}{
			"batch_id":               metric.BatchID,
			"job_id":                 metric.JobID,
			"is_complete":            metric.IsComplete,
			"job_queue_ahead_length": metric.JobQueueAheadLength,
			"queue_start_time":       metric.QueueStartTime.Format(time.RFC3339Nano),
			"queue_receive_time":     metric.QueueReceiveTime.Format(time.RFC3339Nano),
			"queue_duration":         metric.QueueDuration,
			"job_end_time":           metric.JobEndTime.Format(time.RFC3339Nano),
			"processing_duration":    metric.ProcessingDuration,
			"total_duration":         metric.TotalDuration,
			"job_size_mb":            metric.JobSizeMB,
		}
		
		pipe.HSet(ctx, jobArchiveKey, data)
		pipe.Expire(ctx, jobArchiveKey, 30*24*time.Hour)
	}
	
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to archive metrics: %w", err)
	}
	
	log.Printf("Archived %d completed metrics to %s", len(metrics), archiveKey)
	return nil
}

// CleanupJobes now only deletes metrics since archiving is already done in Export
func (ms *MetricsStore) CleanupJobes(ctx context.Context, batchID string, timeParam string, cleanupMode string) error {
		// Export only completed work
		if err := ms.ExportBatchJobesToS3(ctx, batchID, timeParam, true); err != nil {
			log.Printf("WARNING: failed to export COMPLETE metrics for batch %s: %v", batchID, err)
		}
	
		log.Printf("DEBUG: Starting CleanupJobes with mode: %s", cleanupMode)
	
	log.Printf("DEBUG: Starting CleanupJobes with mode: %s", cleanupMode)

	pattern := ms.getBatchPattern(batchID)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get job keys: %w", err)
	}

	var keysToDelete []string

	for _, key := range keys {
		if strings.Contains(key, ":summary:") {
			continue
		}

		// For complete_only mode check completeness
		if cleanupMode == "complete_only" {
			data, err := ms.redis.HGetAll(ctx, key)
			if err != nil {
				continue
			}

			metric, err := ms.parseMetric(data)
			if err != nil || !metric.IsComplete {
				continue // Skip incomplete jobs
			}
		}

		keysToDelete = append(keysToDelete, key)
	}

	// Delete filtered keys
	for _, key := range keysToDelete {
		if err := ms.redis.Del(ctx, key); err != nil {
			log.Printf("failed to delete key %s: %v", key, err)
		}
	}

	log.Printf("Cleaned up %d jobes from batch %s (mode: %s)", len(keysToDelete), batchID, cleanupMode)
	return nil
}

func (ms *MetricsStore) GetAllBatchIDs(ctx context.Context) ([]string, error) {
	// Get batch IDs from active metrics table (used by aggregation service)
	pattern := fmt.Sprintf("%s:*", ms.keyPrefix)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	batchIDs := make(map[string]struct{})
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) >= 2 && !strings.Contains(key, ":summary:") && !strings.Contains(key, ":completed:") {
			batchIDs[parts[1]] = struct{}{}
		}
	}

	result := make([]string, 0, len(batchIDs))
	for id := range batchIDs {
		result = append(result, id)
	}

	return result, nil
}

// GetAllCompletedBatchIDs retrieves batch IDs from completed/archived metrics (used by Prometheus)
func (ms *MetricsStore) GetAllCompletedBatchIDs(ctx context.Context) ([]string, error) {
	// Only scan completed metrics table
	pattern := fmt.Sprintf("%s:completed:*", ms.keyPrefix)
	log.Printf("DEBUG: GetAllCompletedBatchIDs scanning with pattern: %s", pattern)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed keys: %w", err)
	}

	log.Printf("DEBUG: GetAllCompletedBatchIDs found %d keys in Redis", len(keys))
	if len(keys) > 0 && len(keys) <= 10 {
		log.Printf("DEBUG: Sample keys: %v", keys)
	} else if len(keys) > 10 {
		log.Printf("DEBUG: Sample keys (first 10): %v", keys[:10])
	}

	batchIDs := make(map[string]struct{})
	for _, key := range keys {
		// Expected format: metrics:completed:BATCH_ID:JOB_ID
		parts := strings.Split(key, ":")
		log.Printf("DEBUG: Parsing key: %s, parts: %v, len=%d", key, parts, len(parts))
		if len(parts) >= 3 && parts[1] == "completed" {
			batchIDs[parts[2]] = struct{}{}
			log.Printf("DEBUG: Extracted batch ID: %s", parts[2])
		}
	}

	result := make([]string, 0, len(batchIDs))
	for id := range batchIDs {
		result = append(result, id)
	}

	log.Printf("DEBUG: GetAllCompletedBatchIDs returning %d batch IDs: %v", len(result), result)
	return result, nil
}