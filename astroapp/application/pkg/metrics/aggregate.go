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

	//log.Printf("Starting metrics aggregation service with interval %v", as.interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Stopping metrics aggregation service")
			return
		case <-ticker.C:
			timeParam := time.Now().Format("15-04")
			if err := as.AggregateAllBatchs(ctx, timeParam); err != nil {
				log.Printf("Error aggregating batchs: %v", err)
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

// ExportBatchJobesToS3 writes a consolidated text file with all job metrics for an batch
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
	var totalQueueTime, totalProcessingTime, totalJobSize float64
	completeCount := 0

	for _, job := range jobes {
		if !onlyComplete || job.IsComplete {
			filteredJobes = append(filteredJobes, job)
			if job.IsComplete {
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

	// Resolve output path inside bucket: metrics/METRICS_<batchID>.txt
	metricsPrefix := os.Getenv("METRICS")
	fileName := fmt.Sprintf("METRICS_%s_%s.txt", batchID, timeParam)
	s3Bucket := s3bucket.NewS3Bucket()
	err = s3Bucket.UploadFileToBucket(metricsPrefix, fileName, content)
	if err != nil {
		return fmt.Errorf("failed to upload metrics to S3: %w", err)
	}

	log.Printf("Successfully exported metrics for batch %s to s3://%s/%s/%s",
		batchID, s3Bucket.GetBucketName(), metricsPrefix, fileName)

	key := filepath.Join(ms.keyPrefix, "export", fmt.Sprintf("%s:%s", batchID, fileName))
	if err := ms.redis.Set(ctx, key, string(content), ms.ttl); err != nil {
		return fmt.Errorf("failed to stage export content: %w", err)
	}

	log.Printf("Successfully exported %s metrics for batch %s (%d jobes)",
		map[bool]string{true: "complete", false: "all"}[onlyComplete], batchID, len(filteredJobes))

	return nil
}
