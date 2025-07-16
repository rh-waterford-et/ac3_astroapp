package metrics

import (
	"context"
	"fmt"
	"time"
)

type MetricRecord struct {
	EventID               string    `json:"event_id"`
	BatchID               string    `json:"batch_id"`
	QueueStartTime        time.Time `json:"queue_start_time"`         // Time of sending to the queue (fixed in Producer)
	QueueFirstReceiveTime time.Time `json:"queue_first_receive_time"` // Time of receiving first batch from the queue (fixed in Receiver)
	BatchEndTime          time.Time `json:"batch_end_time"`           // Time of end of processing (fixed in Receiver on producer side)
}

type MetricsStore struct {
	redis     *RedisClient
	ttl       time.Duration
	keyPrefix string
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

// RecordMetric records a single metric for an event-batch combination
func (ms *MetricsStore) RecordMetric(ctx context.Context, metric *MetricRecord) error {
	key := ms.getBatchKey(metric.EventID, metric.BatchID)

	values := map[string]interface{}{
		"event_id":                 metric.EventID,
		"batch_id":                 metric.BatchID,
		"queue_start_time":         metric.QueueStartTime.Format(time.RFC3339Nano),
		"queue_first_receive_time": metric.QueueFirstReceiveTime.Format(time.RFC3339Nano),
		"batch_end_time":           metric.BatchEndTime.Format(time.RFC3339Nano),
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

// GetMetric retrieves a metric for a specific event-batch combination
func (ms *MetricsStore) GetMetric(ctx context.Context, eventID, batchID string) (*MetricRecord, error) {
	key := ms.getBatchKey(eventID, batchID)
	data, err := ms.redis.HGetAll(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	return ms.parseMetric(data)
}

// GetEventMetrics retrieves all batches for a specific event
func (ms *MetricsStore) GetEventMetrics(ctx context.Context, eventID string) ([]*MetricRecord, error) {
	pattern := ms.getEventPattern(eventID)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get event keys: %w", err)
	}

	var metrics []*MetricRecord
	for _, key := range keys {
		data, err := ms.redis.HGetAll(ctx, key)
		if err != nil {
			continue // Skip this key if we can't read it
		}
		if len(data) > 0 {
			metric, err := ms.parseMetric(data)
			if err == nil {
				metrics = append(metrics, metric)
			}
		}
	}

	return metrics, nil
}

// UpdateMetricField updates a specific field for an event-batch combination
func (ms *MetricsStore) UpdateMetricField(ctx context.Context, eventID, batchID, field string, value time.Time) error {
	key := ms.getBatchKey(eventID, batchID)

	// First check if the record exists
	exists, err := ms.redis.Exists(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if metric exists: %w", err)
	}

	if exists == 0 {
		// Create new record with only the specified field
		metric := &MetricRecord{
			EventID: eventID,
			BatchID: batchID,
		}

		switch field {
		case "queue_start_time":
			metric.QueueStartTime = value
		case "queue_first_receive_time":
			metric.QueueFirstReceiveTime = value
		case "batch_end_time":
			metric.BatchEndTime = value
		}

		return ms.RecordMetric(ctx, metric)
	}

	// Update existing record
	fieldValue := value.Format(time.RFC3339Nano)
	err = ms.redis.HSet(ctx, key, field, fieldValue)
	if err != nil {
		return fmt.Errorf("failed to update metric field: %w", err)
	}

	return nil
}

// RecordMetricsBatch records multiple metrics in a batch
func (ms *MetricsStore) RecordMetricsBatch(ctx context.Context, metrics []*MetricRecord) error {
	pipe := ms.redis.Pipeline()

	for _, metric := range metrics {
		key := ms.getBatchKey(metric.EventID, metric.BatchID)
		values := map[string]interface{}{
			"event_id":                 metric.EventID,
			"batch_id":                 metric.BatchID,
			"queue_start_time":         metric.QueueStartTime.Format(time.RFC3339Nano),
			"queue_first_receive_time": metric.QueueFirstReceiveTime.Format(time.RFC3339Nano),
			"batch_end_time":           metric.BatchEndTime.Format(time.RFC3339Nano),
		}

		pipe.HSet(ctx, key, values)
		if ms.ttl > 0 {
			pipe.Expire(ctx, key, ms.ttl)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (ms *MetricsStore) parseMetric(data map[string]string) (*MetricRecord, error) {
	var result MetricRecord
	var err error

	result.EventID = data["event_id"]
	result.BatchID = data["batch_id"]

	if data["queue_start_time"] != "" {
		if result.QueueStartTime, err = time.Parse(time.RFC3339Nano, data["queue_start_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse queue_start_time: %w", err)
		}
	}

	if data["queue_first_receive_time"] != "" {
		if result.QueueFirstReceiveTime, err = time.Parse(time.RFC3339Nano, data["queue_first_receive_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse queue_first_receive_time: %w", err)
		}
	}

	if data["batch_end_time"] != "" {
		if result.BatchEndTime, err = time.Parse(time.RFC3339Nano, data["batch_end_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse batch_end_time: %w", err)
		}
	}

	return &result, nil
}

func (ms *MetricsStore) getBatchKey(eventID, batchID string) string {
	return fmt.Sprintf("%s:%s:%s", ms.keyPrefix, eventID, batchID)
}

func (ms *MetricsStore) getEventPattern(eventID string) string {
	return fmt.Sprintf("%s:%s:*", ms.keyPrefix, eventID)
}
