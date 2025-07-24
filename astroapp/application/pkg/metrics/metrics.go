package metrics

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

type MetricRecord struct {
	EventID               string    `json:"event_id"`
	QueueStartTime        time.Time `json:"queue_start_time"`         // Time of sending to the queue (fixed in Producer)
	QueueFirstReceiveTime time.Time `json:"queue_first_receive_time"` // Time of receiving first batch from the queue (fixed in Receiver)
	BatchEndTime          time.Time `json:"batch_end_time"`           // Time of end of processing (fixed in Receiver on producer side)
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

// RecordMetric records a single metric for an event-batch combination
func (ms *MetricsStore) RecordMetric(ctx context.Context, metric *MetricRecord) error {
	key := ms.GetBatchKey(metric.EventID)

	values := map[string]interface{}{
		"event_id":                 metric.EventID,
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
func (ms *MetricsStore) GetMetric(ctx context.Context, eventID string) (*MetricRecord, error) {
	key := ms.GetBatchKey(eventID)
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
func (ms *MetricsStore) UpdateMetricField(ctx context.Context, eventID, field string, value time.Time) error {
	key := ms.GetBatchKey(eventID)

	// First check if the record exists
	exists, err := ms.redis.Exists(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if metric exists: %w", err)
	}
	metric := &MetricRecord{
			EventID: eventID,
		}
	if exists == 0 {
		// Create new record with only the first batch
		switch field {
		case "queue_start_time":
			metric.QueueStartTime = value
			log.Printf("✓ Recorded batch start time %s for event %s", value, eventID)
		case "queue_first_receive_time":
			metric.QueueFirstReceiveTime = value
		}

		return ms.RecordMetric(ctx, metric)
	}
	if field == "batch_end_time" {
		metric.BatchEndTime = value
		log.Printf("DEBUG: Updating metric end_time at %s for event %s", value, eventID)
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
		key := ms.GetBatchKey(metric.EventID)
		values := map[string]interface{}{
			"event_id":                 metric.EventID,
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

func (ms *MetricsStore) GetBatchKey(eventID string) string {
	return fmt.Sprintf("%s:%s", ms.keyPrefix, eventID)
}

func (ms *MetricsStore) getEventPattern(eventID string) string {
	return fmt.Sprintf("%s:%s:*", ms.keyPrefix, eventID)
}

// EventSummary represents aggregated metrics for an entire event
type EventSummary struct {
	EventID             string        `json:"event_id"`
	EventStartTime      time.Time     `json:"event_start_time"`      // Earliest queue_start_time across all batches
	ProcessingStartTime time.Time     `json:"processing_start_time"` // Earliest queue_first_receive_time across all batches
	EventEndTime        time.Time     `json:"event_end_time"`        // Latest batch_end_time across all batches
	TotalFiles          int           `json:"total_files"`           // Total number of files processed
	TotalDuration       time.Duration `json:"total_duration"`        // EventEndTime - EventStartTime
	QueueDelay          time.Duration `json:"queue_delay"`           // ProcessingStartTime - EventStartTime
	ProcessingDuration  time.Duration `json:"processing_duration"`   // EventEndTime - ProcessingStartTime
}

// AggregateEventMetrics aggregates all batch metrics for an event into a summary
func (ms *MetricsStore) AggregateEventMetrics(ctx context.Context, eventID string) (*EventSummary, error) {
	// Get all batch metrics for this event
	batchMetrics, err := ms.GetEventMetrics(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event metrics: %w", err)
	}

	if len(batchMetrics) == 0 {
		return nil, fmt.Errorf("no metrics found for event %s", eventID)
	}

	summary := &EventSummary{
		EventID:    eventID,
		TotalFiles: len(batchMetrics),
	}

	// Initialize with first batch
	summary.EventStartTime = batchMetrics[0].QueueStartTime
	summary.ProcessingStartTime = batchMetrics[0].QueueFirstReceiveTime
	summary.EventEndTime = batchMetrics[0].BatchEndTime

	// Find earliest start times and latest end time
	for _, metric := range batchMetrics {
		// Skip zero times (unset timestamps)
		if !metric.QueueStartTime.IsZero() {
			if summary.EventStartTime.IsZero() || metric.QueueStartTime.Before(summary.EventStartTime) {
				summary.EventStartTime = metric.QueueStartTime
			}
		}

		if !metric.QueueFirstReceiveTime.IsZero() {
			if summary.ProcessingStartTime.IsZero() || metric.QueueFirstReceiveTime.Before(summary.ProcessingStartTime) {
				summary.ProcessingStartTime = metric.QueueFirstReceiveTime
			}
		}

		if !metric.BatchEndTime.IsZero() {
			if summary.EventEndTime.IsZero() || metric.BatchEndTime.After(summary.EventEndTime) {
				summary.EventEndTime = metric.BatchEndTime
			}
		}
	}

	// Calculate durations
	if !summary.EventStartTime.IsZero() && !summary.EventEndTime.IsZero() {
		summary.TotalDuration = summary.EventEndTime.Sub(summary.EventStartTime)
	}

	if !summary.EventStartTime.IsZero() && !summary.ProcessingStartTime.IsZero() {
		summary.QueueDelay = summary.ProcessingStartTime.Sub(summary.EventStartTime)
	}

	if !summary.ProcessingStartTime.IsZero() && !summary.EventEndTime.IsZero() {
		summary.ProcessingDuration = summary.EventEndTime.Sub(summary.ProcessingStartTime)
	}

	return summary, nil
}

// StoreEventSummary stores an aggregated event summary in Redis
func (ms *MetricsStore) StoreEventSummary(ctx context.Context, summary *EventSummary) error {
	key := ms.GetEventSummaryKey(summary.EventID)

	values := map[string]interface{}{
		"event_id":               summary.EventID,
		"event_start_time":       summary.EventStartTime.Format(time.RFC3339Nano),
		"processing_start_time":  summary.ProcessingStartTime.Format(time.RFC3339Nano),
		"event_end_time":         summary.EventEndTime.Format(time.RFC3339Nano),
		"total_files":            summary.TotalFiles,
		"total_duration_ms":      summary.TotalDuration.Milliseconds(),
		"queue_delay_ms":         summary.QueueDelay.Milliseconds(),
		"processing_duration_ms": summary.ProcessingDuration.Milliseconds(),
	}

	// Convert map to slice for HSet
	hsetArgs := make([]interface{}, 0, len(values)*2)
	for k, v := range values {
		hsetArgs = append(hsetArgs, k, v)
	}

	err := ms.redis.HSet(ctx, key, hsetArgs...)
	if err != nil {
		return fmt.Errorf("failed to store event summary: %w", err)
	}

	// Set TTL
	err = ms.redis.Expire(ctx, key, ms.ttl)
	if err != nil {
		return fmt.Errorf("failed to set TTL: %w", err)
	}

	return nil
}

// GetEventSummary retrieves an aggregated event summary from Redis
func (ms *MetricsStore) GetEventSummary(ctx context.Context, eventID string) (*EventSummary, error) {
	key := ms.GetEventSummaryKey(eventID)

	data, err := ms.redis.HGetAll(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get event summary: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no summary found for event %s", eventID)
	}

	summary := &EventSummary{
		EventID: eventID,
	}

	// Parse timestamps
	if data["event_start_time"] != "" {
		if summary.EventStartTime, err = time.Parse(time.RFC3339Nano, data["event_start_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse event_start_time: %w", err)
		}
	}

	if data["processing_start_time"] != "" {
		if summary.ProcessingStartTime, err = time.Parse(time.RFC3339Nano, data["processing_start_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse processing_start_time: %w", err)
		}
	}

	if data["event_end_time"] != "" {
		if summary.EventEndTime, err = time.Parse(time.RFC3339Nano, data["event_end_time"]); err != nil {
			return nil, fmt.Errorf("failed to parse event_end_time: %w", err)
		}
	}

	// Parse other fields
	if data["total_files"] != "" {
		fmt.Sscanf(data["total_files"], "%d", &summary.TotalFiles)
	}

	if data["total_duration_ms"] != "" {
		var totalMs int64
		fmt.Sscanf(data["total_duration_ms"], "%d", &totalMs)
		summary.TotalDuration = time.Duration(totalMs) * time.Millisecond
	}

	if data["queue_delay_ms"] != "" {
		var queueMs int64
		fmt.Sscanf(data["queue_delay_ms"], "%d", &queueMs)
		summary.QueueDelay = time.Duration(queueMs) * time.Millisecond
	}

	if data["processing_duration_ms"] != "" {
		var procMs int64
		fmt.Sscanf(data["processing_duration_ms"], "%d", &procMs)
		summary.ProcessingDuration = time.Duration(procMs) * time.Millisecond
	}

	return summary, nil
}

// CleanupIndividualBatches removes all individual batch entries for an event after aggregation
func (ms *MetricsStore) CleanupBatches(ctx context.Context, eventID string) error {
	pattern := ms.getEventPattern(eventID)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get batch keys for cleanup: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete all individual batch entries (skip summary keys)
	deletedCount := 0
	for _, key := range keys {
		if strings.Contains(key, ":summary:") {
			continue // Skip summary keys
		}
		err := ms.redis.Del(ctx, key)
		if err != nil {
			log.Printf("⚠ Failed to delete key %s: %v", key, err)
		} else {
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Printf("🧹 Cleaned up %d individual batch entries for event %s", deletedCount, eventID)
	}
	return nil
}

// AggregateAndStoreEventSummary aggregates metrics for an event and stores the summary
func (ms *MetricsStore) AggregateAndStoreEventSummary(ctx context.Context, eventID string) (*EventSummary, error) {
	summary, err := ms.AggregateEventMetrics(ctx, eventID)
	if err != nil {
		return nil, err
	}

	err = ms.StoreEventSummary(ctx, summary)
	if err != nil {
		return nil, err
	}

	// Clean up individual batch entries after successful aggregation
	err = ms.CleanupBatches(ctx, eventID)
	if err != nil {
		log.Printf("⚠ Failed to cleanup individual batch entries for event %s: %v", eventID, err)
		// Don't fail the aggregation if cleanup fails - summary is already stored
	}

	return summary, nil
}

// GetEventSummaryKey returns the Redis key for an event summary
func (ms *MetricsStore) GetEventSummaryKey(eventID string) string {
	return fmt.Sprintf("%s:summary:%s", ms.keyPrefix, eventID)
}

// GetAllEventIDs returns all unique event IDs that have metrics
func (ms *MetricsStore) GetAllEventIDs(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("%s:*", ms.keyPrefix)
	keys, err := ms.redis.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	eventIDMap := make(map[string]bool)
	for _, key := range keys {
		// Skip summary keys
		if strings.Contains(key, ":summary:") {
			continue
		}

		// Extract event ID from key format: metrics:eventID:batchID
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			eventID := parts[1]
			if eventID != "" {
				eventIDMap[eventID] = true
			}
		}
	}

	eventIDs := make([]string, 0, len(eventIDMap))
	for eventID := range eventIDMap {
		eventIDs = append(eventIDs, eventID)
	}

	return eventIDs, nil
}

// AggregationService handles periodic aggregation of event metrics
type AggregationService struct {
	metricsStore *MetricsStore
}

// NewAggregationService creates a new aggregation service
func NewAggregationService(metricsStore *MetricsStore) *AggregationService {
	return &AggregationService{
		metricsStore: metricsStore,
	}
}

// AggregateAllEvents runs aggregation for all events that have metrics
func (as *AggregationService) AggregateAllEvents(ctx context.Context) error {
	eventIDs, err := as.metricsStore.GetAllEventIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get event IDs: %w", err)
	}

	log.Printf("📊 Aggregating metrics for %d events", len(eventIDs))

	successCount := 0
	for _, eventID := range eventIDs {
		summary, err := as.metricsStore.AggregateAndStoreEventSummary(ctx, eventID)
		if err != nil {
			log.Printf("⚠ Failed to aggregate metrics for event %s: %v", eventID, err)
			continue
		}

		log.Printf("✅ Aggregated event %s: %d batches, total: %s, queue delay: %s, processing: %s",
			eventID,
			summary.TotalFiles,
			summary.TotalDuration.Round(time.Millisecond),
			summary.QueueDelay.Round(time.Millisecond),
			summary.ProcessingDuration.Round(time.Millisecond))
		successCount++
	}

	log.Printf("📈 Aggregation complete: %d/%d events processed successfully", successCount, len(eventIDs))
	return nil
}

// RunPeriodicAggregation runs aggregation on a timer (can be called in a goroutine)
func (as *AggregationService) RunPeriodicAggregation(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("🔄 Starting periodic aggregation every %s", interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 Stopping periodic aggregation")
			return
		case <-ticker.C:
			if err := as.AggregateAllEvents(ctx); err != nil {
				log.Printf("❌ Periodic aggregation failed: %v", err)
			}
		}
	}
}
