// BinaryBatchSender handles binary batchs for PPXF
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

type BinaryBatchSender interface {
	SendBinaryBatch(batch api.BinaryBatch, appName string, side string, q queue.QueueInterface)
}

// BinaryRabbitMQSender handles binary batchs via RabbitMQ
type BinaryRabbitMQSender struct {
	Queue queue.QueueInterface
	Utils common.UtilsInterface
}

// SendBinaryBatch handles binary batchs for PPXF (preserves .fits file integrity)
func (bs *BinaryRabbitMQSender) SendBinaryBatch(batch api.BinaryBatch, appName string, side string, q queue.QueueInterface) {
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

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		bs.Utils.FailOnError("Failed to marshal binary batch", err)
	}

	headers := make(amqp.Table)
	headers["job_size"] = len(batch.Files)
	headers["app_name"] = appName
	headers["batch_id"] = batch.ID
	headers["job_id"] = batch.JobID
	headers["is_binary"] = true // CRITICAL: Mark as binary batch

	filenames := []string{}
	for _, f := range batch.Files {
		filenames = append(filenames, f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")

	err = q.Publish(ctx, queueName, batchJSON, headers)
	if err != nil {
		bs.Utils.FailOnError("Failed to publish binary message: %v", err)
	}
	log.Printf(" [x] Sent binary job with %d files for app %s\n", len(batch.Files), appName)
	log.Printf("     Binary Files: %s\n", strings.Join(filenames, ", "))
	log.Printf("DEBUG: SendBinaryBatch completed successfully")
}
