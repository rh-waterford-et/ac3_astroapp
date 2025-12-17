package sender

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
)

type BatchSender interface {
	SendBatch(batch api.Batch, appName string, side string, q queue.QueueInterface)
}

type RabbitMQSender struct {
	Queue       queue.QueueInterface
	Utils       common.UtilsInterface
	RedisClient *metrics.RedisClient
}

func NewRabbitMQSender(queue queue.QueueInterface, utils common.UtilsInterface, redisClient *metrics.RedisClient) *RabbitMQSender {
	return &RabbitMQSender{
		Queue:       queue,
		Utils:       utils,
		RedisClient: redisClient,
	}
}

func (s *RabbitMQSender) SendBatch(batch api.Batch, appName string, side string, q queue.QueueInterface) {
	// Validate queue interface to prevent nil pointer panics
	if q == nil {
		log.Printf("ERROR: Queue interface is nil, cannot send batch %s (JobID: %s). Job will be lost!", batch.ID, batch.JobID)
		return
	}

	var queueName string

	if side == "producer" {
		queueName = "producer_to_processor_queue"
	} else {
		queueName = "processor_to_producer_queue"
	}

	// Declare queue with error handling instead of panic
	err := q.DeclareQueue(queueName)
	if err != nil {
		log.Printf("ERROR: Failed to declare queue %s: %v. Batch %s (JobID: %s) cannot be sent.", queueName, err, batch.ID, batch.JobID)
		return
	}

	if side == "producer" {
		stats, err := q.InspectQueue(queueName)
		if err != nil {
			log.Printf("Failed to inspect queue %s: %v", queueName, err)
		} else {
			log.Printf("Queue %s before publish: messages=%d, consumers=%d",
				queueName, stats.Messages, stats.Consumers)

			// Record queue metrics in Redis
			if s.RedisClient != nil {
				metricsStore := metrics.NewMetricsStore(s.RedisClient, 168*time.Hour)
				metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer metricsCancel()

				// Record queue length ahead of this job
				err = metricsStore.UpdateMetricField(metricsCtx, batch.ID, "job_queue_ahead_length", batch.JobID, stats.Messages)
				if err != nil {
					log.Printf("Failed to record job_queue_ahead_length: %v", err)
				}

				// Calculate and record job size in MB
				jobSizeMB := calculateJobSizeMB(batch.Files)
				if jobSizeMB > 0 {
					err = metricsStore.UpdateMetricField(metricsCtx, batch.ID, "job_size_mb", batch.JobID, jobSizeMB)
					if err != nil {
						log.Printf("Failed to record job_size_mb: %v", err)
					} else {
						log.Printf("Recorded job size: %.2f MB for batch %s, job %s", jobSizeMB, batch.ID, batch.JobID)
					}
				}

				/* // Record number of active consumers
				err = metricsStore.UpdateMetricField(metricsCtx, batch.ID, "queue_consumers_count", batch.JobID, stats.Consumers)
				if err != nil {
					log.Printf("Failed to record queue_consumers_count: %v", err)
				} */

			} else {
				log.Printf("No Redis client found for queue metrics")
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		log.Printf("ERROR: Failed to marshal batch %s (JobID: %s): %v. Job cannot be sent.", batch.ID, batch.JobID, err)
		return
	}

	headers := make(amqp.Table)
	headers["job_size"] = len(batch.Files)
	headers["app_name"] = appName
	headers["batch_id"] = batch.ID
	headers["job_id"] = batch.JobID

	filenames := []string{}
	for _, f := range batch.Files {
		filenames = append(filenames, f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")

	// Retry logic for publishing to handle transient RabbitMQ failures
	maxRetries := 5
	retryDelay := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = q.Publish(ctx, queueName, batchJSON, headers)
		if err == nil {
			// Success!
			break
		}

		if attempt < maxRetries {
			log.Printf("WARNING: Failed to publish batch %s (JobID: %s) to queue %s (attempt %d/%d): %v. Retrying in %v...",
				batch.ID, batch.JobID, queueName, attempt, maxRetries, err, retryDelay)
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		} else {
			log.Printf("ERROR: Failed to publish batch %s (JobID: %s) to queue %s after %d attempts: %v. Job will be lost!",
				batch.ID, batch.JobID, queueName, maxRetries, err)
			return
		}
	}

	stats, err := q.InspectQueue(queueName)
	if err != nil {
		log.Printf("Failed to inspect queue after publish: %v", err)
	} else {
		log.Printf("Queue %s after publish: messages=%d, consumers=%d",
			queueName, stats.Messages, stats.Consumers)
	}

	if side == "producer" {
		// Record queue start time for each job (individual job tracking)
		if s.RedisClient != nil {
			metricsStore := metrics.NewMetricsStore(s.RedisClient, 168*time.Hour)

			// Create a separate context with longer timeout for metrics operations
			metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer metricsCancel()

			// Record queue start time for this specific job
			err = metricsStore.UpdateMetricField(metricsCtx, batch.ID, "queue_start_time", batch.JobID, time.Now())
			if err != nil {
				log.Printf("Failed to record queue start time: %v", err)
			}
		} else {
			log.Printf("No Redis client found")
		}
	}

	log.Printf("DEBUG: Published message to queue %s", queueName)
	log.Printf(" [x] Sent job with %d files for app %s\n", len(batch.Files), appName)
	log.Printf("     Files: %s\n", strings.Join(filenames, ", "))
}

// calculateJobSizeMB calculates the total size of all files in MB
func calculateJobSizeMB(files []api.DataFile) float64 {
	totalSize := 0
	totalFiles := 0
	for _, file := range files {
		// Skip .in files (config files, not actual data)
		if strings.HasSuffix(file.Name, ".in") {
			continue
		}
		totalSize += len(file.Content)
		totalFiles++
	}

	// Convert bytes to MB
	sizeMB := float64(totalSize) / (1024 * 1024)
	log.Printf("DEBUG: Calculated job size: %.2f MB (%d bytes across %d files)",
		sizeMB, totalSize, totalFiles)

	return sizeMB
}
