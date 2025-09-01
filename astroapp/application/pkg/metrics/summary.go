package metrics

import (
	"context"
	"fmt"
	"time"
)

type EventSummary struct {
	BatchID                string    `json:"batch_id"`
	JobCount               int       `json:"job_count"`
	FirstJobQueueStartTime time.Time `json:"first_job_queue_start_time"`
	FirstJobExitQueueTime  time.Time `json:"first_job_exit_queue_time"`
	LastJobEndTime         time.Time `json:"last_job_end_time"`
	TotalEventDuration time.Duration `json:"total_event_duration_s"`
}

func (ms *MetricsStore) GetEventSummaryKey(eventID string) string {
	return fmt.Sprintf("%s:summary:%s", ms.keyPrefix, eventID)
}

func (ms *MetricsStore) StoreEventSummary(ctx context.Context, summary *EventSummary) error {
	key := fmt.Sprintf("%s:summary:%s", ms.keyPrefix, summary.BatchID)

	values := map[string]interface{}{
		"batch_id":               summary.BatchID,
		"job_count":              summary.JobCount,
		"first_job_queue_start_time": summary.FirstJobQueueStartTime.Format(time.RFC3339Nano),
		"first_job_exit_queue_time":  summary.FirstJobExitQueueTime.Format(time.RFC3339Nano),
		"last_job_end_time":      summary.LastJobEndTime.Format(time.RFC3339Nano),
		"total_event_duration_s": summary.TotalEventDuration.Seconds(),
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

	summary := &EventSummary{BatchID: eventID}
	var parseErr error

	if data["job_count"] != "" {
		fmt.Sscanf(data["job_count"], "%d", &summary.JobCount)
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

	summary.FirstJobExitQueueTime, parseErr = parseTime("first_job_exit_queue_time")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse first_job_exit_queue_time: %w", parseErr)
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

	summary.TotalEventDuration = parseSeconds("total_event_duration_s")

	return summary, nil
}
