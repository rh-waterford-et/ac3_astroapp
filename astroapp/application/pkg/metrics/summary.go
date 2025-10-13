package metrics

import (
	"context"
	"fmt"
	"time"
)

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

func (ms *MetricsStore) GetBatchSummaryKey(batchID string) string {
	return fmt.Sprintf("%s:summary:%s", ms.keyPrefix, batchID)
}

func (ms *MetricsStore) StoreBatchSummary(ctx context.Context, summary *BatchSummary) error {
	key := fmt.Sprintf("%s:summary:%s", ms.keyPrefix, summary.BatchID)

	values := map[string]interface{}{
		"batch_id":                   summary.BatchID,
		"job_count":                  summary.JobCount,
		"first_job_queue_start_time": summary.FirstJobQueueStartTime.Format(time.RFC3339Nano),
		"last_job_exit_queue_time":   summary.LastJobExitQueueTime.Format(time.RFC3339Nano),
		"last_job_end_time":          summary.LastJobEndTime.Format(time.RFC3339Nano),
		"total_batch_duration_s":     summary.TotalBatchDuration.Seconds(),

		"complete_job_count": summary.CompleteJobCount,

		"avg_job_queue_time_s":      summary.AvgJobQueueTime.Seconds(),
		"avg_job_processing_time_s": summary.AvgJobProcessingTime.Seconds(),
	}

	err := ms.redis.HSet(ctx, key, values)
	if err != nil {
		return fmt.Errorf("failed to store batch summary: %w", err)
	}

	if ms.ttl > 0 {
		err = ms.redis.Expire(ctx, key, ms.ttl)
		if err != nil {
			return fmt.Errorf("failed to set TTL: %w", err)
		}
	}

	return nil
}

func (ms *MetricsStore) GetBatchSummary(ctx context.Context, batchID string) (*BatchSummary, error) {
	key := fmt.Sprintf("%s:summary:%s", ms.keyPrefix, batchID)
	data, err := ms.redis.HGetAll(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch summary: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	summary := &BatchSummary{BatchID: batchID}
	var parseErr error

	if data["job_count"] != "" {
		fmt.Sscanf(data["job_count"], "%d", &summary.JobCount)
	}

	if data["complete_job_count"] != "" {
		fmt.Sscanf(data["complete_job_count"], "%d", &summary.CompleteJobCount)
	}

	parseTime := func(field string) (time.Time, error) {
		if data[field] == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339Nano, data[field])
	}

	summary.FirstJobQueueStartTime, parseErr = parseTime("first_job_queue_start_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse first_job_queue_start_time: %w", parseErr)
	}

	summary.LastJobExitQueueTime, parseErr = parseTime("last_job_exit_queue_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse last_job_exit_queue_time: %w", parseErr)
	}

	summary.LastJobEndTime, parseErr = parseTime("last_job_end_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse last_job_end_time: %w", parseErr)
	}

	parseSeconds := func(field string) time.Duration {
		if data[field] == "" {
			return 0
		}
		var secs float64
		fmt.Sscanf(data[field], "%f", &secs)
		return time.Duration(secs * float64(time.Second))
	}

	summary.TotalBatchDuration = parseSeconds("total_batch_duration_s")
	summary.AvgJobQueueTime = parseSeconds("avg_job_queue_time_s")
	summary.AvgJobProcessingTime = parseSeconds("avg_job_processing_time_s")

	return summary, nil
}
