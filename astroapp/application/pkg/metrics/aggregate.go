package metrics

import (
	"context"
	"fmt"
	"log"
	"time"
)

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

	//log.Printf("AGG: Starting metrics aggregation service with interval %v", as.interval)

	for {
		select {
		case <-ctx.Done():
			//log.Printf("AGG: Stopping metrics aggregation service")
			return
		case <-ticker.C:
			timeParam := time.Now().Format("15-04")
			if err := as.AggregateAllBatchs(ctx, timeParam); err != nil {
				//log.Printf("AGG: Error aggregating batchs: %v", err)
			}
		}
	}
}

func (ms *MetricsStore) AggregateBatchMetrics(ctx context.Context, batchID string) (*BatchSummary, error) {
	jobes, err := ms.GetBatchJobes(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch jobs: %w", err)
	}

	if len(jobes) == 0 {
		return nil, fmt.Errorf("no jobes found for batch %s", batchID)
	}

	summary := &BatchSummary{
		BatchID:  batchID,
		JobCount: len(jobes),
	}
	completeCount := 0

	// Initialize with first job values
	summary.FirstJobQueueStartTime = jobes[0].QueueStartTime
	summary.LastJobExitQueueTime = jobes[0].QueueReceiveTime
	summary.LastJobEndTime = jobes[0].JobEndTime

	var (
		totalQueueTime      time.Duration
		totalProcessingTime time.Duration
	)

	for _, job := range jobes {
		if job.IsComplete {
			completeCount++
		}

		// Update first and last times
		if job.QueueStartTime.Before(summary.FirstJobQueueStartTime) {
			summary.FirstJobQueueStartTime = job.QueueStartTime
		}

		if job.QueueReceiveTime.After(summary.LastJobExitQueueTime) {
			summary.LastJobExitQueueTime = job.QueueReceiveTime
		}

		if job.JobEndTime.After(summary.LastJobEndTime) {
			summary.LastJobEndTime = job.JobEndTime
		}

		// Calculate durations
		queueTime := job.QueueReceiveTime.Sub(job.QueueStartTime)
		processingTime := job.JobEndTime.Sub(job.QueueReceiveTime)

		// Update totals
		totalQueueTime += queueTime
		totalProcessingTime += processingTime
	}

	summary.TotalBatchDuration = summary.LastJobEndTime.Sub(summary.FirstJobQueueStartTime)
	summary.CompleteJobCount = completeCount

	if completeCount > 0 {
		summary.AvgJobQueueTime = totalQueueTime / time.Duration(completeCount)
		summary.AvgJobProcessingTime = totalProcessingTime / time.Duration(completeCount)
	}

	return summary, nil
}

func (ms *MetricsStore) AggregateAndStoreBatchSummary(ctx context.Context, batchID string, timeParam string) (*BatchSummary, error) {
	summary, err := ms.AggregateBatchMetrics(ctx, batchID)
	if err != nil {
		return nil, err
	}

	if err := ms.StoreBatchSummary(ctx, summary); err != nil {
		return nil, err
	}

	// Removed for now to allow prometheus to read Job details before they are removed, please evaulate if still needed
	// TODO possibly introduce a timed job to clean down the Redis data
	if err := ms.CleanupJobes(ctx, batchID, timeParam, "complete_only"); err != nil {
		log.Printf("failed to cleanup jobes for batch %s: %v", batchID, err)
	}

	return summary, nil
}

func (as *AggregationService) AggregateAllBatchs(ctx context.Context, timeParam string) error {
	batchIDs, err := as.metricsStore.GetAllBatchIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get batch IDs: %w", err)
	}

	log.Printf("Aggregating metrics for %d batchs", len(batchIDs))

	successCount := 0
	for _, batchID := range batchIDs {
		summary, err := as.metricsStore.AggregateAndStoreBatchSummary(ctx, batchID, timeParam)
		if err != nil {
			log.Printf("Failed to aggregate batch %s: %v", batchID, err)
			continue
		}
		log.Printf("Aggregated batch %s: jobs=%d (complete=%d), duration=%v, queue_time=avg:%v, processing_time=avg:%v",
			batchID,
			summary.JobCount,
			summary.CompleteJobCount,
			summary.TotalBatchDuration.Round(time.Second),
			summary.AvgJobQueueTime.Round(time.Millisecond),
			summary.AvgJobProcessingTime.Round(time.Millisecond))

		successCount++
	}

	log.Printf("Aggregation complete: %d/%d batchs processed", successCount, len(batchIDs))
	return nil
}
