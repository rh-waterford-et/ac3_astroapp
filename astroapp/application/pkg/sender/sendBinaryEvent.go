// BinaryEventSender handles binary events for PPXF
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
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
)

type BinaryEventSender interface {
	SendBinaryEvent(event api.BinaryEvent, appName string, side string, q queue.QueueInterface)
}

// BinaryRabbitMQSender handles binary events via RabbitMQ
type BinaryRabbitMQSender struct {
	Queue queue.QueueInterface
	Utils common.UtilsInterface
}

// SendBinaryEvent handles binary events for PPXF (preserves .fits file integrity)
func (bs *BinaryRabbitMQSender) SendBinaryEvent(event api.BinaryEvent, appName string, side string, q queue.QueueInterface) {
	var queueName string

	if side == "producer" {
		queueName = "producer_to_processor_queue"
	} else {
		queueName = "processor_to_producer_queue"
	}

	err := q.Connect()
	if err != nil {
		bs.Utils.FailOnError("Failed to connect to RabbitMQ", err)
	}
	defer q.Close()

	err = q.DeclareQueue(queueName)
	if err != nil {
		bs.Utils.FailOnError(fmt.Sprintf("Failed to declare queue"), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		bs.Utils.FailOnError("Failed to marshal binary event", err)
	}

	headers := make(amqp.Table)
	headers["batch_size"] = len(event.Files)
	headers["app_name"] = appName
	headers["event_id"] = event.ID

	filenames := []string{}
	for _, f := range event.Files {
		filenames = append(filenames, f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")

	err = q.Publish(ctx, queueName, eventJSON, headers)
	if err != nil {
		bs.Utils.FailOnError("Failed to publish binary message: %v", err)
	}
	log.Printf(" [x] Sent binary batch with %d files for app %s\n", len(event.Files), appName)
	log.Printf("     Binary Files: %s\n", strings.Join(filenames, ", "))
	log.Printf("DEBUG: SendBinaryEvent completed successfully")
}
