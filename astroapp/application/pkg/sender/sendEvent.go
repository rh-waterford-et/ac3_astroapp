package sender

import (
	"context"
	"encoding/json"
	"fmt"
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
	var queueName string

	if side == "producer" {
		queueName = "producer_to_processor_queue"
	} else {
		queueName = "processor_to_producer_queue"
	}

	err := q.Connect()
	if err != nil {
		s.Utils.FailOnError("Failed to connect to RabbitMQ", err)
	}
	defer q.Close()

	err = q.DeclareQueue(queueName)
	if err != nil {
		s.Utils.FailOnError(fmt.Sprintf("Failed to declare queue"), err)
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
		s.Utils.FailOnError("Failed to marshal batch", err)
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

	err = q.Publish(ctx, queueName, batchJSON, headers)
	stats, err := q.InspectQueue(queueName)
	log.Printf("Queue %s after publish: messages=%d, consumers=%d",
				queueName, stats.Messages, stats.Consumers)
	if err != nil {
		s.Utils.FailOnError("Failed to publish message: %v", err)
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
