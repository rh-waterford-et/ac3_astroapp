package receiver

import (
	"context"
	"log"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
)

func (r *Receiver) recordQueueFirstReceiveTime(batchID, jobID string) {
	if r.RedisClient == nil {
		return
	}

	log.Printf("│ DEBUG: Recording QueueFirstReceiveTime for processor side")
	ctx := context.Background()
	metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)

	key := metricsStore.GetJobKey(batchID, jobID)
	log.Printf("│ DEBUG: Redis key = %s", key)

	exists, err := r.RedisClient.Exists(ctx, key)
	if err != nil {
		log.Printf("│ ✗ Failed to check metric existence: %v", err)
		return
	}

	if exists == 0 {
		r.createNewMetricRecord(metricsStore, ctx, batchID, jobID)
	} else {
		r.updateExistingMetricRecord(metricsStore, ctx, batchID, jobID, key)
	}
}

func (r *Receiver) createNewMetricRecord(metricsStore *metrics.MetricsStore, ctx context.Context, batchID, jobID string) {
	log.Printf("│ DEBUG: Creating new metric record")
	// When the processor sees a batch first time, set only queue_receive_time; queue_start_time is set by producer at publish time
	err := metricsStore.UpdateMetricField(ctx, batchID, "queue_receive_time", jobID, time.Now())
	if err != nil {
		log.Printf("│ ✗ Failed to record queue receive time: %v", err)
	} else {
		log.Printf("│ ✓ Recorded queue receive time for batch %s, job %s", batchID, jobID)
	}
}

func (r *Receiver) updateExistingMetricRecord(metricsStore *metrics.MetricsStore, ctx context.Context, batchID, jobID, key string) {
	log.Printf("│ DEBUG: Metric already exists, checking if queue_receive_time is set")
	allFields, err := r.RedisClient.HGetAll(ctx, key)
	if err != nil {
		log.Printf("│ ✗ Failed to get metric fields: %v", err)
		return
	}

	currentTime := allFields["queue_receive_time"]
	log.Printf("│ DEBUG: Current queue_receive_time = '%s'", currentTime)
	if currentTime == "" || currentTime == "0001-01-01T00:00:00Z" {
		err = metricsStore.UpdateMetricField(ctx, batchID, "queue_receive_time", jobID, time.Now())
		if err != nil {
			log.Printf("│ ✗ Failed to update queue receive time: %v", err)
		} else {
			log.Printf("│ ✓ Updated queue receive time for batch %s", batchID)
		}
	}
}

func (r *Receiver) recordJobEndTime(batchID, jobID string) {
	metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)
	err := metricsStore.UpdateMetricField(context.Background(), batchID, "job_end_time", jobID, time.Now())
	if err != nil {
		log.Printf("│ ✗ Failed to record batch end time: %v", err)
	} /* else {
		log.Printf("│ ✓ Recorded job end time for %s", jobID)
	} */
}

// recordJobSize records the job size in metrics
func (r *Receiver) recordJobSize(batchID, jobID string, sizeMB float64) {
	ctx := context.Background()
	metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)

	err := metricsStore.UpdateMetricField(ctx, batchID, "job_size_mb", jobID, sizeMB)
	if err != nil {
		log.Printf("│ ✗ Failed to record job size: %v", err)
	} else {
		log.Printf("│ ✓ Recorded job size: %.2f MB for batch %s, job %s", sizeMB, batchID, jobID)
	}
}
