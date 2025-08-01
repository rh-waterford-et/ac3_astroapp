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
	EventID          string    `json:"event_id"`
	BatchID          string    `json:"batch_id"`
	QueueStartTime   time.Time `json:"queue_start_time"`   // Time of sending batch to queue
	QueueReceiveTime time.Time `json:"queue_receive_time"` // Time of receiving batch from queue
	BatchEndTime     time.Time `json:"batch_end_time"`     // Time of batch processing completion
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
	key := ms.GetBatchKey(metric.EventID, metric.BatchID)

	values := map[string]interface{}{
		"event_id":           metric.EventID,
		"batch_id":           metric.BatchID,
		"queue_start_time":   metric.QueueStartTime.Format(time.RFC3339Nano),
		"queue_receive_time": metric.QueueReceiveTime.Format(time.RFC3339Nano),
		"batch_end_time":     metric.BatchEndTime.Format(time.RFC3339Nano),
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

	result.EventID = data["event_id"]
	result.BatchID = data["batch_id"]

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

	if data["batch_end_time"] != "" {
		if result.BatchEndTime, err = time.Parse(time.RFC3339Nano, data["batch_end_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse batch_end_time: %w", err)
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

func (ms *MetricsStore) UpdateMetricField(ctx context.Context, eventID, batchID, field string, value time.Time) error {
	key := ms.GetBatchKey(eventID, batchID)
	log.Printf("DEBUG: UpdateMetricField - key: %s, field: %s, value: %s", key, field, value.Format(time.RFC3339Nano))

	// Always set basic fields
	err := ms.redis.HSet(ctx, key, "event_id", eventID, "batch_id", batchID)
	if err != nil {
		return fmt.Errorf("failed to set basic fields: %w", err)
	}

	// Set the specified field
	err = ms.redis.HSet(ctx, key, field, value.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("failed to update metric field: %w", err)
	}

	// For queue_start_time, also update first_queue_start_time if this is earlier
	if field == "queue_start_time" {
		currentFirst, err := ms.redis.HGetAll(ctx, key)
		if err != nil || currentFirst["first_queue_start_time"] == "" {
			// If no value exists, set this as first
			err = ms.redis.HSet(ctx, key, "first_queue_start_time", value.Format(time.RFC3339Nano))
			if err != nil {
				return fmt.Errorf("failed to set first_queue_start_time: %w", err)
			}
		} else {
			// Compare with existing first_queue_start_time
			currentTime, err := time.Parse(time.RFC3339Nano, currentFirst["first_queue_start_time"])
			if err == nil && value.Before(currentTime) {
				err = ms.redis.HSet(ctx, key, "first_queue_start_time", value.Format(time.RFC3339Nano))
				if err != nil {
					return fmt.Errorf("failed to update first_queue_start_time: %w", err)
				}
			}
		}
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
		key := ms.GetBatchKey(metric.EventID, metric.BatchID)
		values := map[string]interface{}{
			"event_id":           metric.EventID,
			"batch_id":           metric.BatchID,
			"queue_start_time":   metric.QueueStartTime.Format(time.RFC3339Nano),
			"queue_receive_time": metric.QueueReceiveTime.Format(time.RFC3339Nano),
			"batch_end_time":     metric.BatchEndTime.Format(time.RFC3339Nano),
		}

		pipe.HSet(ctx, key, values)
		if ms.ttl > 0 {
			pipe.Expire(ctx, key, ms.ttl)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

type EventSummary struct {
	EventID             string        `json:"event_id"`
	BatchCount          int           `json:"batch_count"`
	FirstQueueStartTime time.Time     `json:"first_queue_start_time"`
	FirstBatchEndTime   time.Time     `json:"first_batch_end_time"`
	LastBatchEndTime    time.Time     `json:"last_batch_end_time"`
	AvgQueueTime        time.Duration `json:"avg_queue_time_ms"`
	AvgProcessingTime   time.Duration `json:"avg_processing_time_ms"`
	TotalEventDuration  time.Duration `json:"total_event_duration_ms"`
}

func (ms *MetricsStore) AggregateEventMetrics(ctx context.Context, eventID string) (*EventSummary, error) {
	batches, err := ms.GetEventBatches(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event batches: %w", err)
	}

	if len(batches) == 0 {
		return nil, fmt.Errorf("no batches found for event %s", eventID)
	}

	summary := &EventSummary{
		EventID:    eventID,
		BatchCount: len(batches),
	}

	// Initialize with first batch values
	summary.FirstQueueStartTime = batches[0].QueueStartTime
	summary.FirstBatchEndTime = batches[0].BatchEndTime
	summary.LastBatchEndTime = batches[0].BatchEndTime

	var (
		totalQueueTime      time.Duration
		totalProcessingTime time.Duration
	)

	for _, batch := range batches {
		// Update first and last times
		if batch.QueueStartTime.Before(summary.FirstQueueStartTime) {
			summary.FirstQueueStartTime = batch.QueueStartTime
		}

		if batch.BatchEndTime.Before(summary.FirstBatchEndTime) {
			summary.FirstBatchEndTime = batch.BatchEndTime
		}

		if batch.BatchEndTime.After(summary.LastBatchEndTime) {
			summary.LastBatchEndTime = batch.BatchEndTime
		}

		// Calculate durations
		queueTime := batch.QueueReceiveTime.Sub(batch.QueueStartTime)
		processingTime := batch.BatchEndTime.Sub(batch.QueueReceiveTime)

		// Update totals
		totalQueueTime += queueTime
		totalProcessingTime += processingTime
	}

	// Calculate averages
	if len(batches) > 0 {
		summary.AvgQueueTime = totalQueueTime / time.Duration(len(batches))
		summary.AvgProcessingTime = totalProcessingTime / time.Duration(len(batches))
	}

	summary.TotalEventDuration = summary.LastBatchEndTime.Sub(summary.FirstQueueStartTime)

	return summary, nil
}

func (ms *MetricsStore) GetEventSummaryKey(eventID string) string {
	return fmt.Sprintf("%s:summary:%s", ms.keyPrefix, eventID)
}

func (ms *MetricsStore) StoreEventSummary(ctx context.Context, summary *EventSummary) error {
	key := fmt.Sprintf("%s:summary:%s", ms.keyPrefix, summary.EventID)

	values := map[string]interface{}{
		"event_id":               summary.EventID,
		"batch_count":            summary.BatchCount,
		"first_queue_start_time": summary.FirstQueueStartTime.Format(time.RFC3339Nano),
		"first_batch_end_time":   summary.FirstBatchEndTime.Format(time.RFC3339Nano),
		"last_batch_end_time":    summary.LastBatchEndTime.Format(time.RFC3339Nano),
		"avg_queue_time_ms":      summary.AvgQueueTime.Milliseconds(),
		"avg_processing_time_ms": summary.AvgProcessingTime.Milliseconds(),
		"total_event_duration_ms": summary.TotalEventDuration.Milliseconds(),
	}

	err := ms.redis.HSet(ctx, key, values)
	if err != nil {
		return fmt.Errorf("failed to store event summary: %w", err)
	}

	if ms.ttl > 0 {
		err = ms.redis.Expire(ctx, key, ms.ttl)
		if err != nil {
			return fmt.Errorf("failed to set TTL: %w", err)
		}
	}

	return nil
}


func (ms *MetricsStore) GetEventSummary(ctx context.Context, eventID string) (*EventSummary, error) {
	key := fmt.Sprintf("%s:summary:%s", ms.keyPrefix, eventID)
	data, err := ms.redis.HGetAll(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get event summary: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	summary := &EventSummary{EventID: eventID}
	var parseErr error

	if data["batch_count"] != "" {
		fmt.Sscanf(data["batch_count"], "%d", &summary.BatchCount)
	}

	parseTime := func(field string) (time.Time, error) {
		if data[field] == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339Nano, data[field])
	}

	summary.FirstQueueStartTime, parseErr = parseTime("first_queue_start_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse first_queue_start_time: %w", parseErr)
	}

	summary.FirstBatchEndTime, parseErr = parseTime("first_batch_end_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse first_batch_end_time: %w", parseErr)
	}

	summary.LastBatchEndTime, parseErr = parseTime("last_batch_end_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse last_batch_end_time: %w", parseErr)
	}

	parseDuration := func(field string) time.Duration {
		if data[field] == "" {
			return 0
		}
		var ms int64
		fmt.Sscanf(data[field], "%d", &ms)
		return time.Duration(ms) * time.Millisecond
	}

	summary.AvgQueueTime = parseDuration("avg_queue_time_ms")
	summary.AvgProcessingTime = parseDuration("avg_processing_time_ms")
	summary.TotalEventDuration = parseDuration("total_event_duration_ms")

	return summary, nil
}

func (ms *MetricsStore) CleanupBatches(ctx context.Context, eventID string) error {
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

func (ms *MetricsStore) AggregateAndStoreEventSummary(ctx context.Context, eventID string) (*EventSummary, error) {
	summary, err := ms.AggregateEventMetrics(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if err := ms.StoreEventSummary(ctx, summary); err != nil {
		return nil, err
	}

	if err := ms.CleanupBatches(ctx, eventID); err != nil {
		log.Printf("failed to cleanup batches for event %s: %v", eventID, err)
	}

	return summary, nil
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

type AggregationService struct {
	metricsStore *MetricsStore
	interval     time.Duration
}

func NewAggregationService(metricsStore *MetricsStore, interval time.Duration) *AggregationService {
	return &AggregationService{
		metricsStore: metricsStore,
		interval:     interval,
	}
}

func (as *AggregationService) Run(ctx context.Context) {
	ticker := time.NewTicker(as.interval)
	defer ticker.Stop()

	log.Printf("Starting metrics aggregation service with interval %v", as.interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Stopping metrics aggregation service")
			return
		case <-ticker.C:
			if err := as.AggregateAllEvents(ctx); err != nil {
				log.Printf("Error aggregating events: %v", err)
			}
		}
	}
}

func (as *AggregationService) AggregateAllEvents(ctx context.Context) error {
	eventIDs, err := as.metricsStore.GetAllEventIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get event IDs: %w", err)
	}

	log.Printf("Aggregating metrics for %d events", len(eventIDs))

	successCount := 0
	for _, eventID := range eventIDs {
		summary, err := as.metricsStore.AggregateAndStoreEventSummary(ctx, eventID)
		if err != nil {
			log.Printf("Failed to aggregate event %s: %v", eventID, err)
			continue
		}

		log.Printf("Aggregated event %s: batches=%d, duration=%v, queue_time=(avg:%v, min:%v, max:%v), processing_time=(avg:%v, min:%v, max:%v)",
			eventID,
			summary.BatchCount,
			summary.TotalEventDuration.Round(time.Second),
			summary.AvgQueueTime.Round(time.Second),
			summary.AvgProcessingTime.Round(time.Second))

		successCount++
	}

	log.Printf("Aggregation complete: %d/%d events processed", successCount, len(eventIDs))
	return nil
}