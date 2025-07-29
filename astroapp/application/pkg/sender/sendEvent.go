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

type EventSender interface {
	SendEvent(event api.Event, appName string, side string, q queue.QueueInterface)
}
type RabbitMQSender struct {
	Queue       queue.QueueInterface
	Utils       common.UtilsInterface
	RedisClient *metrics.RedisClient
}

func (s *RabbitMQSender) SendEvent(event api.Event, appName string, side string, q queue.QueueInterface) {
	log.Printf("DEBUG: SendEvent called with event.ID=%s, appName=%s, side=%s", event.ID, appName, side)
	var queueName string


	if side == "producer" {
		queueName = "producer_to_processor_queue"
		log.Printf("DEBUG: queueName set to: %s", queueName)
	} else {
		queueName = "processor_to_producer_queue"
		log.Printf("DEBUG: queueName set to: %s", queueName)
	}

	err := q.Connect()
	log.Printf("DEBUG: Queue connect attempt finished, err=%v", err)
	if err != nil {
		s.Utils.FailOnError("Failed to connect to RabbitMQ", err)
	}
	defer q.Close()
	log.Printf("DEBUG: Deferred queue close registered")

	err = q.DeclareQueue(queueName)
	log.Printf("DEBUG: DeclareQueue(%s) finished, err=%v", queueName, err)
	if err != nil {
		s.Utils.FailOnError("Failed to declare queue", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	log.Printf("DEBUG: Event marshaling finished, eventJSON length=%d, err=%v", len(eventJSON), err)
	if err != nil {
		log.Printf("DEBUG: Event marshaling failed, calling FailOnError")
		s.Utils.FailOnError("Failed to marshal event", err)
		log.Printf("DEBUG: FailOnError called for marshaling failure")
	}

	headers := make(amqp.Table)
	log.Printf("DEBUG: AMQP headers table created")
	headers["batch_size"] = len(event.Files)
	log.Printf("DEBUG: batch_size header set to %d", len(event.Files))
	headers["app_name"] = appName
	log.Printf("DEBUG: app_name header set to %s", appName)
	headers["event_id"] = event.ID
	log.Printf("DEBUG: event_id header set to %s", event.ID)

	filenames := []string{}
	log.Printf("DEBUG: Filenames slice initialized")
	for _, f := range event.Files {
		log.Printf("DEBUG: Processing file: %s", f.Name)
		filenames = append(filenames, f.Name)
		log.Printf("DEBUG: Added filename to slice: %s", f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")
	log.Printf("DEBUG: filenames header set to: %s", headers["filenames"])

	err = q.Publish(ctx, queueName, eventJSON, headers)
	log.Printf("DEBUG: Queue publish finished, err=%v", err)
	if err != nil {
		s.Utils.FailOnError("Failed to publish message: %v", err)
	}

	// Record queue start time for each batch (individual batch tracking)
	if s.RedisClient != nil {
		log.Printf("DEBUG: RedisClient is available, creating metrics store")
		metricsStore := metrics.NewMetricsStore(s.RedisClient, 168*time.Hour)
		log.Printf("DEBUG: MetricsStore created with 168h TTL")

		// Record queue start time for this specific batch
		err = metricsStore.UpdateMetricField(ctx, event.ID, "queue_start_time", time.Now())
		log.Printf("DEBUG: UpdateMetricField called for queue_start_time, err=%v", err)
		if err != nil {
			log.Printf("Failed to record queue start time: %v", err)
		}
	} else {
		log.Printf("No Redis client found")
	}

	log.Printf("DEBUG: Published message to queue %s", queueName)
	log.Printf(" [x] Sent batch with %d files for app %s\n", len(event.Files), appName)
	log.Printf("     Files: %s\n", strings.Join(filenames, ", "))
	log.Printf("DEBUG: SendEvent completed successfully")
}
