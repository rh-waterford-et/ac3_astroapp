package metrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)



// ExportEventBatchesToS3 writes a consolidated text file with all batch metrics for an event
func (ms *MetricsStore) ExportEventBatchesToS3(ctx context.Context, eventID string) error {
	// Gather batch records
	batches, err := ms.GetEventBatches(ctx, eventID)
	if err != nil {
		return fmt.Errorf("failed to get event batches for export: %w", err)
	}
	if len(batches) == 0 {
		return nil
	}

	// Sort by QueueStartTime for stable output
	sort.Slice(batches, func(i, j int) bool {
		return batches[i].QueueStartTime.Before(batches[j].QueueStartTime)
	})

	// Build text content
	var b strings.Builder
	b.WriteString(fmt.Sprintf("METRICS: %s\n", eventID))
	for idx, rec := range batches {
		b.WriteString("----------------------------------------\n")
		b.WriteString(fmt.Sprintf("Job %d\n", idx+1))
		b.WriteString(fmt.Sprintf("batch_id: %s\n", rec.BatchID))
		b.WriteString(fmt.Sprintf("job_id: %s\n", rec.JobID))
		if !rec.QueueStartTime.IsZero() {
			b.WriteString(fmt.Sprintf("queue_start_time: %s\n", rec.QueueStartTime.Format(time.RFC3339Nano)))
		}
		if !rec.QueueReceiveTime.IsZero() {
			b.WriteString(fmt.Sprintf("queue_receive_time: %s\n", rec.QueueReceiveTime.Format(time.RFC3339Nano)))
		}
		if !rec.JobEndTime.IsZero() {
			b.WriteString(fmt.Sprintf("job_processed_end_time: %s\n", rec.JobEndTime.Format(time.RFC3339Nano)))
		}
	}

	content := []byte(b.String())

	// Resolve output path inside bucket: metrics/METRICS_<eventID>.txt
	metricsPrefix := os.Getenv("METRICS")
	fileName := fmt.Sprintf("METRICS_%s_%s.txt", eventID, time.Now().Format("15-04"))
	s3Bucket := s3bucket.NewS3Bucket()
	err = s3Bucket.UploadFileToBucket(metricsPrefix, fileName, content)
	if err != nil {
		return fmt.Errorf("failed to upload metrics to S3: %w", err)
	}

	log.Printf("Successfully exported metrics for event %s to s3://%s/%s/%s",
		eventID, s3Bucket.GetBucketName(), metricsPrefix, fileName)

	key := filepath.Join(ms.keyPrefix, "export", fmt.Sprintf("%s:%s", eventID, fileName))
	if err := ms.redis.Set(ctx, key, string(content), ms.ttl); err != nil {
		return fmt.Errorf("failed to stage export content: %w", err)
	}
	log.Printf("Staged metrics export for event %s at redis key %s (to be written to bucket path %s/%s)", eventID, key, metricsPrefix, fileName)
	return nil
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
		BatchID:  eventID,
		JobCount: len(batches),
	}

	// Initialize with first batch values
	summary.FirstJobQueueStartTime = batches[0].QueueStartTime
	summary.FirstJobExitQueueTime = batches[0].QueueReceiveTime
	summary.LastJobEndTime = batches[0].JobEndTime

	var (
		totalQueueTime      time.Duration
		totalProcessingTime time.Duration
	)

	for _, batch := range batches {
		// Update first and last times
		if batch.QueueStartTime.Before(summary.FirstJobQueueStartTime) {
			summary.FirstJobQueueStartTime = batch.QueueStartTime
		}

		if batch.QueueReceiveTime.Before(summary.FirstJobExitQueueTime) {
			summary.FirstJobExitQueueTime = batch.JobEndTime
		}

		if batch.JobEndTime.After(summary.LastJobEndTime) {
			summary.LastJobEndTime = batch.JobEndTime
		}

		// Calculate durations
		queueTime := batch.QueueReceiveTime.Sub(batch.QueueStartTime)
		processingTime := batch.JobEndTime.Sub(batch.QueueReceiveTime)

		// Update totals
		totalQueueTime += queueTime
		totalProcessingTime += processingTime
	}

	// Calculate averages
	if len(batches) > 0 {
		/* 	summary.AvgQueueTime = totalQueueTime / time.Duration(len(batches))
		summary.AvgProcessingTime = totalProcessingTime / time.Duration(len(batches)) */
	}

	summary.TotalEventDuration = summary.LastJobEndTime.Sub(summary.FirstJobQueueStartTime)

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
		log.Printf("Aggregated batch %s: jobs=%d, duration=%v",
			eventID,
			summary.JobCount,
			summary.TotalEventDuration.Round(time.Second))
		/* log.Printf("Aggregated batch %s: jobs=%d, duration=%v, queue_time=avg:%v, processing_time=avg:%v",
			eventID,
			summary.JobCount,
			summary.TotalEventDuration.Round(time.Second),
			summary.AvgQueueTime.Round(time.Second),
			summary.AvgProcessingTime.Round(time.Second)) */

		successCount++
	}

	log.Printf("Aggregation complete: %d/%d events processed", successCount, len(eventIDs))
	return nil
}
