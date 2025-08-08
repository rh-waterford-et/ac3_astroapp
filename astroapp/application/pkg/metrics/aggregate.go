package metrics

import (
	"context"
	"fmt"
	"log"
	"time"
)

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
