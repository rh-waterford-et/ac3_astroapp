package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// MetricRecord represents an individual job metric (matches UC3's structure)
type MetricRecord struct {
	BatchID          string    `json:"batch_id"`
	JobID            string    `json:"job_id"`
	QueueStartTime   time.Time `json:"queue_start_time"`
	QueueReceiveTime time.Time `json:"queue_receive_time"`
	JobEndTime       time.Time `json:"job_end_time"`

	QueueDuration      float64 `json:"queue_duration"`      // seconds
	ProcessingDuration float64 `json:"processing_duration"` // seconds
	TotalDuration      float64 `json:"total_duration"`      // seconds

	IsComplete bool `json:"is_complete"`

	JobSizeMB           float64 `json:"job_size_mb"`
	JobQueueAheadLength int     `json:"job_queue_ahead_length"`
}

// BatchSummary represents aggregated metrics for a batch (matches UC3's structure)
type BatchSummary struct {
	BatchID                string        `json:"batch_id"`
	JobCount               int           `json:"job_count"`
	FirstJobQueueStartTime time.Time     `json:"first_job_queue_start_time"`
	LastJobExitQueueTime   time.Time     `json:"last_job_exit_queue_time"`
	LastJobEndTime         time.Time     `json:"last_job_end_time"`
	TotalBatchDuration     time.Duration `json:"total_batch_duration_s"`

	CompleteJobCount int `json:"complete_job_count"`

	AvgJobQueueTime      time.Duration `json:"avg_job_queue_time"`
	AvgJobProcessingTime time.Duration `json:"avg_job_processing_time"`
}

// RedisCollector connects to UC3's Redis instance and retrieves metrics
type RedisCollector struct {
	client    *redis.Client
	keyPrefix string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// NewRedisCollector creates a new Redis metrics collector
func NewRedisCollector(config RedisConfig) (*RedisCollector, error) {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: config.Password,
		DB:       config.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	return &RedisCollector{
		client:    client,
		keyPrefix: "metrics", // UC3's default prefix
	}, nil
}

// Close closes the Redis connection
func (r *RedisCollector) Close() error {
	return r.client.Close()
}

// TestConnection verifies Redis connectivity
func (r *RedisCollector) TestConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err()
}

// GetJobMetrics retrieves all job metrics for a specific batch
func (r *RedisCollector) GetJobMetrics(ctx context.Context, batchID string) ([]*MetricRecord, error) {
	pattern := fmt.Sprintf("%s:%s:*", r.keyPrefix, batchID)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to find keys for batch %s: %w", batchID, err)
	}

	var metrics []*MetricRecord
	for _, key := range keys {
		// Skip summary keys
		if strings.Contains(key, ":summary:") {
			continue
		}

		metric, err := r.getMetricFromKey(ctx, key)
		if err != nil {
			continue // Skip failed metrics but continue processing
		}
		metrics = append(metrics, metric)
	}

	return metrics, nil
}

// GetBatchSummary retrieves aggregated batch metrics
func (r *RedisCollector) GetBatchSummary(ctx context.Context, batchID string) (*BatchSummary, error) {
	key := fmt.Sprintf("%s:summary:%s", r.keyPrefix, batchID)
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get batch summary for %s: %w", batchID, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no batch summary found for batch %s", batchID)
	}

	summary := &BatchSummary{BatchID: batchID}

	// Parse job count
	if data["job_count"] != "" {
		if count, err := strconv.Atoi(data["job_count"]); err == nil {
			summary.JobCount = count
		}
	}

	if data["complete_job_count"] != "" {
		if count, err := strconv.Atoi(data["complete_job_count"]); err == nil {
			summary.CompleteJobCount = count
		}
	}

	// Parse time fields
	parseTime := func(field string) (time.Time, error) {
		if data[field] == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339Nano, data[field])
	}

	summary.FirstJobQueueStartTime, _ = parseTime("first_job_queue_start_time")
	summary.LastJobExitQueueTime, _ = parseTime("last_job_exit_queue_time")
	summary.LastJobEndTime, _ = parseTime("last_job_end_time")

	// Parse duration fields
	parseDuration := func(field string) time.Duration {
		if data[field] == "" {
			return 0
		}
		if secs, err := strconv.ParseFloat(data[field], 64); err == nil {
			return time.Duration(secs * float64(time.Second))
		}
		return 0
	}

	summary.TotalBatchDuration = parseDuration("total_batch_duration_s")
	summary.AvgJobQueueTime = parseDuration("avg_job_queue_time_s")
	summary.AvgJobProcessingTime = parseDuration("avg_job_processing_time_s")

	return summary, nil
}

// GetAllBatchIDs retrieves all available batch IDs
func (r *RedisCollector) GetAllBatchIDs(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("%s:*", r.keyPrefix)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get batch keys: %w", err)
	}

	batchIDSet := make(map[string]bool)
	for _, key := range keys {
		// Extract batch ID from key format: "metrics:batchID:jobID" or "metrics:summary:batchID"
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			if parts[1] == "summary" && len(parts) >= 3 {
				// Summary key: metrics:summary:batchID
				batchIDSet[parts[2]] = true
			} else if len(parts) >= 3 {
				// Job key: metrics:batchID:jobID
				batchIDSet[parts[1]] = true
			}
		}
	}

	var batchIDs []string
	for batchID := range batchIDSet {
		batchIDs = append(batchIDs, batchID)
	}

	return batchIDs, nil
}

// CalculateAverages calculates average metrics from job records
func (r *RedisCollector) CalculateAverages(metrics []*MetricRecord) (avgJobSizeMB, avgQueueTime, avgProcessingTime, avgTotalTime float64) {
	if len(metrics) == 0 {
		return 0, 0, 0, 0
	}

	var totalJobSize, totalQueueTime, totalProcessingTime, totalTotalTime float64
	completeJobs := 0

	for _, metric := range metrics {
		if metric.IsComplete {
			totalJobSize += metric.JobSizeMB
			totalQueueTime += metric.QueueDuration
			totalProcessingTime += metric.ProcessingDuration
			totalTotalTime += metric.TotalDuration
			completeJobs++
		}
	}

	if completeJobs == 0 {
		return 0, 0, 0, 0
	}

	return totalJobSize / float64(completeJobs),
		totalQueueTime / float64(completeJobs),
		totalProcessingTime / float64(completeJobs),
		totalTotalTime / float64(completeJobs)
}

// getMetricFromKey retrieves a single metric record from a Redis key
func (r *RedisCollector) getMetricFromKey(ctx context.Context, key string) (*MetricRecord, error) {
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get metric data for key %s: %w", key, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no data found for key %s", key)
	}

	metric := &MetricRecord{
		BatchID: data["batch_id"],
		JobID:   data["job_id"],
	}

	// Parse time fields
	parseTime := func(field string) (time.Time, error) {
		if data[field] == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339Nano, data[field])
	}

	metric.QueueStartTime, _ = parseTime("queue_start_time")
	metric.QueueReceiveTime, _ = parseTime("queue_receive_time")
	metric.JobEndTime, _ = parseTime("job_end_time")

	// Parse numeric fields
	if data["queue_duration"] != "" {
		metric.QueueDuration, _ = strconv.ParseFloat(data["queue_duration"], 64)
	}
	if data["processing_duration"] != "" {
		metric.ProcessingDuration, _ = strconv.ParseFloat(data["processing_duration"], 64)
	}
	if data["total_duration"] != "" {
		metric.TotalDuration, _ = strconv.ParseFloat(data["total_duration"], 64)
	}
	if data["job_size_mb"] != "" {
		metric.JobSizeMB, _ = strconv.ParseFloat(data["job_size_mb"], 64)
	}
	if data["job_queue_ahead_length"] != "" {
		metric.JobQueueAheadLength, _ = strconv.Atoi(data["job_queue_ahead_length"])
	}

	// Parse boolean
	metric.IsComplete = data["is_complete"] == "true" || data["is_complete"] == "1"

	return metric, nil
}
