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
	QueueStartTime   time.Time `json:"queue_start_time"`   // Time of sending batch to queue
	QueueReceiveTime time.Time `json:"queue_receive_time"` // Time of receiving batch from queue
	JobEndTime     time.Time `json:"job_end_time"`     // Time of batch processing completion
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

func (ms *MetricsStore) GetBatchKey(eventID, batchID string) string {
	return fmt.Sprintf("%s:%s:%s", ms.keyPrefix, eventID, batchID)
}

func (ms *MetricsStore) getEventPattern(eventID string) string {
	return fmt.Sprintf("%s:%s:*", ms.keyPrefix, eventID)
}

func (ms *MetricsStore) RecordMetric(ctx context.Context, metric *MetricRecord) error {
	key := ms.GetBatchKey(metric.BatchID, metric.JobID)

	values := map[string]interface{}{
		"batch_id":           metric.BatchID,
		"job_id":             metric.JobID,
		"queue_start_time":   metric.QueueStartTime.Format(time.RFC3339Nano),
		"queue_receive_time": metric.QueueReceiveTime.Format(time.RFC3339Nano),
		"job_processed_end_time":     metric.JobEndTime.Format(time.RFC3339Nano),
	}

	err := ms.redis.HSet(ctx, key, values)
	if err != nil {
		return fmt.Errorf("failed to record metric: %w", err)
	}

	if ms.ttl > 0 {
		err = ms.redis.Expire(ctx, key, ms.ttl)
		if err != nil {
			return fmt.Errorf("failed to set TTL for metric: %w", err)
		}
	}

	return nil
}

func (ms *MetricsStore) GetMetric(ctx context.Context, eventID, batchID string) (*MetricRecord, error) {
	key := ms.GetBatchKey(eventID, batchID)
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

	return &result, nil
}

func (ms *MetricsStore) GetEventBatches(ctx context.Context, eventID string) ([]*MetricRecord, error) {
	pattern := ms.getEventPattern(eventID)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch keys: %w", err)
	}

	var batches []*MetricRecord
	for _, key := range keys {
		data, err := ms.redis.HGetAll(ctx, key)
		if err != nil {
			continue
		}

		batch, err := ms.parseMetric(data)
		if err == nil {
			batches = append(batches, batch)
		}
	}

	sort.Slice(batches, func(i, j int) bool {
		return batches[i].QueueStartTime.Before(batches[j].QueueStartTime)
	})

	return batches, nil
}

func (ms *MetricsStore) UpdateMetricField(ctx context.Context, eventID, field, batchID string, value time.Time) error {
	key := ms.GetBatchKey(eventID, batchID)
	log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %s", key, field, value.Format(time.RFC3339Nano))

	// Always set basic fields
	err := ms.redis.HSet(ctx, key, "event_id", eventID, "batch_id", batchID)
	if err != nil {
		return fmt.Errorf("failed to set basic fields: %w", err)
	}

	// Read current value to enforce monotonic policy
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
				if !value.Before(t) {
					// Existing value is earlier or equal; keep it
					if ms.ttl > 0 {
						_ = ms.redis.Expire(ctx, key, ms.ttl)
					}
					return nil
				}
			case "job_end_time":
				// Keep latest timestamp
				if !value.After(t) {
					// Existing value is later or equal; keep it
					if ms.ttl > 0 {
						_ = ms.redis.Expire(ctx, key, ms.ttl)
					}
					return nil
				}
			}
		}
	}

	// Set the specified field with the new value (passed policy checks)
	err = ms.redis.HSet(ctx, key, field, value.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("failed to update metric field: %w", err)
	}

	if ms.ttl > 0 {
		err = ms.redis.Expire(ctx, key, ms.ttl)
		if err != nil {
			return fmt.Errorf("failed to set TTL: %w", err)
		}
	}

	return nil
}

func (ms *MetricsStore) RecordMetricsBatch(ctx context.Context, metrics []*MetricRecord) error {
	pipe := ms.redis.Pipeline()

	for _, metric := range metrics {
		key := ms.GetBatchKey(metric.BatchID, metric.JobID)
		values := map[string]interface{}{
			"batch_id":           metric.BatchID,
			"queue_start_time":   metric.QueueStartTime.Format(time.RFC3339Nano),
			"queue_receive_time": metric.QueueReceiveTime.Format(time.RFC3339Nano),
			"job_end_time":     metric.JobEndTime.Format(time.RFC3339Nano),
		}

		pipe.HSet(ctx, key, values)
		if ms.ttl > 0 {
			pipe.Expire(ctx, key, ms.ttl)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (ms *MetricsStore) CleanupBatches(ctx context.Context, eventID string) error {

	if err := ms.ExportEventBatchesToS3(ctx, eventID); err != nil {
		log.Printf("WARNING: failed to export metrics for event %s before cleanup: %v", eventID, err)
	}

	pattern := ms.getEventPattern(eventID)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get batch keys: %w", err)
	}

	for _, key := range keys {
		if !strings.Contains(key, ":summary:") {
			if err := ms.redis.Del(ctx, key); err != nil {
				log.Printf("failed to delete key %s: %v", key, err)
			}
		}
	}

	return nil
}

func (ms *MetricsStore) GetAllEventIDs(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("%s:*", ms.keyPrefix)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	eventIDs := make(map[string]struct{})
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) >= 2 && !strings.Contains(key, ":summary:") {
			eventIDs[parts[1]] = struct{}{}
		}
	}

	result := make([]string, 0, len(eventIDs))
	for id := range eventIDs {
		result = append(result, id)
	}

	return result, nil
}
