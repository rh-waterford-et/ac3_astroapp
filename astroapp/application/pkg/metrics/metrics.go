package metrics

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
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
		log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %s", key, field, v.Format(time.RFC3339Nano))

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
		log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %.2f", key, field, v)

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
		log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %d", key, field, v)

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

func (ms *MetricsStore) CleanupJobes(ctx context.Context, batchID string, timeParam string, cleanupMode string) error {

	// Export all data before clearing
	/*   if err := ms.ExportBatchJobesToS3(ctx, batchID, timeParam, false); err != nil {
	     log.Printf("WARNING: failed to export ALL metrics for batch %s: %v", batchID, err)
	 } */

	// Export only completed work
	if err := ms.ExportBatchJobesToS3(ctx, batchID, timeParam, true); err != nil {
		log.Printf("WARNING: failed to export COMPLETE metrics for batch %s: %v", batchID, err)
	}

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

		// Check cleanup mode
		if cleanupMode == "complete_only" {
			// Get work data for checking completeness
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
	pattern := fmt.Sprintf("%s:*", ms.keyPrefix)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	batchIDs := make(map[string]struct{})
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) >= 2 && !strings.Contains(key, ":summary:") {
			batchIDs[parts[1]] = struct{}{}
		}
	}

	result := make([]string, 0, len(batchIDs))
	for id := range batchIDs {
		result = append(result, id)
	}

	return result, nil
}
