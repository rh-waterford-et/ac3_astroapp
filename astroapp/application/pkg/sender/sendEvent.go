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

type EventSender interface {
	SendEvent(event api.Event, appName string, side string, q queue.QueueInterface)
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



func (s *RabbitMQSender) SendEvent(event api.Event, appName string, side string, q queue.QueueInterface) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		s.Utils.FailOnError("Failed to marshal event", err)
	}

	headers := make(amqp.Table)
	headers["batch_size"] = len(event.Files)
	headers["app_name"] = appName
	headers["event_id"] = event.ID
	headers["batch_id"] = event.BatchID

	filenames := []string{}
	for _, f := range event.Files {
		filenames = append(filenames, f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")

	err = q.Publish(ctx, queueName, eventJSON, headers)
	if err != nil {
		s.Utils.FailOnError("Failed to publish message: %v", err)
	}

	// Record queue start time for each batch (individual batch tracking)
	if s.RedisClient != nil {
		metricsStore := metrics.NewMetricsStore(s.RedisClient, 168*time.Hour)

		// Record queue start time for this specific batch
		err = metricsStore.UpdateMetricField(ctx, event.ID, "queue_start_time", time.Now())
		if err != nil {
			log.Printf("Failed to record queue start time: %v", err)
		} /* else {
			log.Printf("✓ Recorded batch start time for event %s", event.ID)
		} */
	} else {
		log.Printf("No Redis client found")
	}

	log.Printf("DEBUG: Published message to queue %s", queueName)
	log.Printf(" [x] Sent batch with %d files for app %s\n", len(event.Files), appName)
	log.Printf("     Files: %s\n", strings.Join(filenames, ", "))
}

