package collector

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
)

// RedisCollector handles metrics collection from UC3's Redis instance
type RedisCollector struct {
	redisClient  *metrics.RedisClient
	metricsStore *metrics.MetricsStore
}

// NewRedisCollector creates a new Redis collector using UC3's connection pattern
func NewRedisCollector() (*RedisCollector, error) {
	// Use UC3's Redis connection method
	redisClient, err := metrics.NewRedisConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create metrics store with 7-day TTL (same as UC3)
	metricsStore := metrics.NewMetricsStore(redisClient, 168*time.Hour)

	return &RedisCollector{
		redisClient:  redisClient,
		metricsStore: metricsStore,
	}, nil
}

// TestConnection verifies Redis connectivity
func (rc *RedisCollector) TestConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rc.redisClient.Ping(ctx)
	if err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// GetActiveBatches scans for active batch IDs in Redis
func (rc *RedisCollector) GetActiveBatches(ctx context.Context) ([]string, error) {
	return rc.metricsStore.GetAllBatchIDs(ctx)
}

// GetJobMetrics retrieves individual job metrics for a batch
func (rc *RedisCollector) GetJobMetrics(ctx context.Context, batchID string) ([]JobMetrics, error) {
	// Use UC3's existing method to get batch jobs
	ucMetrics, err := rc.metricsStore.GetBatchJobes(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch jobs: %w", err)
	}

	// Convert UC3 metrics to our format
	jobMetrics := make([]JobMetrics, len(ucMetrics))
	for i, ucMetric := range ucMetrics {
		jobMetrics[i] = JobMetrics{
			BatchID:            ucMetric.BatchID,
			JobID:              ucMetric.JobID,
			QueueStartTime:     ucMetric.QueueStartTime,
			QueueReceiveTime:   ucMetric.QueueReceiveTime,
			JobEndTime:         ucMetric.JobEndTime,
			QueueDuration:      ucMetric.QueueDuration,
			ProcessingDuration: ucMetric.ProcessingDuration,
			TotalDuration:      ucMetric.TotalDuration,
			IsComplete:         ucMetric.IsComplete,
			JobSizeMB:          ucMetric.JobSizeMB,
			QueueAheadLength:   ucMetric.JobQueueAheadLength,
		}
	}

	return jobMetrics, nil
}

// GetSystemSnapshot captures current system state
func (rc *RedisCollector) GetSystemSnapshot(ctx context.Context, processorCount int) (*SystemSnapshot, error) {
	activeBatches, err := rc.GetActiveBatches(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active batches: %w", err)
	}

	var allJobMetrics []JobMetrics
	for _, batchID := range activeBatches {
		jobMetrics, err := rc.GetJobMetrics(ctx, batchID)
		if err != nil {
			log.Printf("Warning: failed to get metrics for batch %s: %v", batchID, err)
			continue
		}
		allJobMetrics = append(allJobMetrics, jobMetrics...)
	}

	return &SystemSnapshot{
		Timestamp:      time.Now(),
		ProcessorCount: processorCount,
		ActiveBatches:  activeBatches,
		JobMetrics:     allJobMetrics,
	}, nil
}

// CalculateLiveMetrics aggregates job metrics into live system metrics
func (rc *RedisCollector) CalculateLiveMetrics(snapshot *SystemSnapshot, experimentID string) *LiveMetrics {
	activeJobs := 0
	completedJobs := 0
	totalProcessingTime := 0.0
	completedCount := 0

	for _, job := range snapshot.JobMetrics {
		if job.IsComplete {
			completedJobs++
			totalProcessingTime += job.ProcessingDuration
			completedCount++
		} else {
			activeJobs++
		}
	}

	avgProcessingTime := 0.0
	if completedCount > 0 {
		avgProcessingTime = totalProcessingTime / float64(completedCount)
	}

	// Calculate throughput (jobs per minute) - rough estimate
	throughput := 0.0
	if completedCount > 0 && avgProcessingTime > 0 {
		throughput = 60.0 / avgProcessingTime // jobs per minute per processor
		throughput *= float64(snapshot.ProcessorCount)
	}

	return &LiveMetrics{
		Timestamp:         snapshot.Timestamp,
		ProcessorCount:    snapshot.ProcessorCount,
		ExperimentID:      experimentID,
		ActiveJobs:        activeJobs,
		CompletedJobs:     completedJobs,
		QueueDepth:        activeJobs, // Approximation
		AvgProcessingTime: avgProcessingTime,
		Throughput:        throughput,
	}
}

// GetBatchSummary gets aggregated metrics for a specific batch using UC3's aggregation
func (rc *RedisCollector) GetBatchSummary(ctx context.Context, batchID string, processorCount int, experimentID string) (*BatchMetrics, error) {
	// Use UC3's existing batch aggregation
	ucSummary, err := rc.metricsStore.AggregateBatchMetrics(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate batch metrics: %w", err)
	}

	// Calculate throughput
	throughput := 0.0
	if ucSummary.TotalBatchDuration > 0 {
		throughput = float64(ucSummary.CompleteJobCount) / ucSummary.TotalBatchDuration.Minutes()
	}

	// Calculate total size
	jobs, err := rc.GetJobMetrics(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job metrics for size calculation: %w", err)
	}

	totalSizeMB := 0.0
	for _, job := range jobs {
		totalSizeMB += job.JobSizeMB
	}

	return &BatchMetrics{
		ExperimentID:       experimentID,
		BatchID:            batchID,
		ProcessorCount:     processorCount,
		JobCount:           ucSummary.JobCount,
		CompleteJobCount:   ucSummary.CompleteJobCount,
		AvgQueueTime:       ucSummary.AvgJobQueueTime,
		AvgProcessingTime:  ucSummary.AvgJobProcessingTime,
		TotalBatchDuration: ucSummary.TotalBatchDuration,
		Throughput:         throughput,
		TotalSizeMB:        totalSizeMB,
	}, nil
}

// Close closes the Redis connection
func (rc *RedisCollector) Close() error {
	return rc.redisClient.Close()
}
