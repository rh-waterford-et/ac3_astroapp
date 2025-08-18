package metrics

import (
	"context"
	"fmt"
	"time"
)

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
		"avg_queue_time_s":       summary.AvgQueueTime.Seconds(),
		"avg_processing_time_s":  summary.AvgProcessingTime.Seconds(),
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

	parseSeconds := func(field string) time.Duration {
		if data[field] == "" {
			return 0
		}
		var secs float64
		fmt.Sscanf(data[field], "%f", &secs)
		return time.Duration(secs * float64(time.Second))
	}

	summary.AvgQueueTime = parseSeconds("avg_queue_time_s")
	summary.AvgProcessingTime = parseSeconds("avg_processing_time_s")
	summary.TotalEventDuration = parseSeconds("total_event_duration_s")

	return summary, nil
}
