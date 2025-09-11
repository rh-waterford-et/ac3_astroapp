package receiver

import (
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type ReceiverInterface interface {
	Start()
	ProcessMessages()
	ProcessMessage(d amqp.Delivery)
}

type Receiver struct {
	Queue       queue.QueueInterface
	Utils       common.UtilsInterface
	Bucket      s3bucket.S3BucketInterface
	RedisClient *metrics.RedisClient
	processing  bool
}

func NewReceiver(queue queue.QueueInterface, utils common.UtilsInterface, bucket s3bucket.S3BucketInterface, redisClient *metrics.RedisClient) *Receiver {
	return &Receiver{
		Queue:       queue,
		Utils:       utils,
		Bucket:      bucket,
		RedisClient: redisClient,
		processing:  true,
	}
}

func (r *Receiver) Start(side string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Clear process list on startup to prevent processing old entries
	processLists := []string{os.Getenv("PROCESS_LIST_STARLIGHT"), os.Getenv("PROCESS_LIST_PPXF")}
	for _, processList := range processLists {
		if side == "processor" {
			r.clearProcessList(processList)

		}
	}

	var queueName string
	if side == "producer" {
		queueName = "processor_to_producer_queue"
	} else {
		queueName = "producer_to_processor_queue"
	}

	err := r.Queue.Connect()
	r.Utils.FailOnError("Failed to connect to RabbitMQ", err)
	defer r.Queue.Close()

	err = r.Queue.DeclareQueue(queueName)
	r.Utils.FailOnError(fmt.Sprintf("Failed to declare queue: %s", queueName), err)

	err = r.Queue.SetQoS(1)
	r.Utils.FailOnError("Failed to set QoS", err)

	for {
		r.ProcessMessages(queueName, side)
		time.Sleep(1 * time.Second)
	}
}

func (r *Receiver) ProcessMessages(queueName string, side string) {

	if side == "processor" {
		status, err := r.checkProcessLists()
		if err != nil {
			log.Printf("Error checking process lists: %v", err)
			return
		}

		if !status {
			//log.Println("Process lists are not empty, waiting...")
			return
		}
	}

	// Ensure queue connection is valid
	if !r.ensureQueueConnection(queueName) {
		return
	}

	queueInfo, err := r.Queue.InspectQueue(queueName)
	if err != nil {
		log.Printf("QUEUE ERROR: Failed to inspect queue %s: %v", queueName, err)
		return
	}
	if queueInfo.Messages == 0 {
		return
	}

	log.Printf("PROCESSING QUEUE: %s (%d messages)", queueName, queueInfo.Messages)

	consumerTag := fmt.Sprintf("consumer-%s-%d", queueName, time.Now().UnixNano())
	msgs, err := r.Queue.Consume(queueName, consumerTag)
	if err != nil {
		log.Printf("CONSUME ERROR: Failed to register consumer for queue %s: %v", queueName, err)
		return
	}

	defer func() {
		if err := r.Queue.CancelConsumer(consumerTag); err != nil {
			log.Printf("WARNING: Failed to cancel consumer %s: %v", consumerTag, err)
		}
	}()

	// Process only ONE message at a time
	select {
	case d, ok := <-msgs:
		if !ok {
			return
		}
		r.ProcessMessage(d, side)

		// After processing one message, check if process lists are now non-empty
		// This prevents processing additional messages when the process list fills up
		if side == "processor" {
			status, err := r.checkProcessLists()
			if err != nil {
				log.Printf("Error checking process lists after message processing: %v", err)
			} else if !status {
				log.Println("Process lists are now non-empty, stopping message processing")
			}
		}
	case <-time.After(5 * time.Second):
		return
	}
}

func (r *Receiver) ProcessMessage(d amqp.Delivery, side string) {
	// Initial validation and setup
	appName, batchID, jobID, batchN, ok := r.validateHeaders(d)
	if !ok {
		return
	}

	// Log batch start information
	r.logBatchStart(batchN, appName, batchID, jobID)

	// Handle queue metrics for processor side
	if side == "processor" {
		r.recordQueueFirstReceiveTime(batchID, jobID)
	}

	// Process binary messages separately
	if isBinary, _ := d.Headers["is_binary"].(bool); isBinary {
		r.ProcessBinaryMessage(d, side, appName, jobID)
		return
	}

	// Process standard text message
	r.processStandardMessage(d, side, appName, batchID, jobID)
}

func (r *Receiver) finalizeBatchProcessing(d amqp.Delivery, side, appName, batchID, jobID string, successCount int, batchSize int32) {
	if successCount == int(batchSize) {
		err := d.Ack(false)
		if err != nil {
			log.Printf("│ ERROR ack: %v", err)
		}
		log.Printf("│ ✔ Successfully processed all %d files", batchSize)

		if side == "producer" && r.RedisClient != nil {
			r.recordJobEndTime(batchID, jobID)
		}
		if side == "processor" {
			if filenamesHeader, ok := d.Headers["filenames"].(string); ok {
				if err := r.createBatchInfoFile(appName, batchID, jobID, filenamesHeader); err != nil {
					log.Printf("│ ⚠ Failed to create batch info file: %v", err)
				}
			}
		}
		r.updateProgress(appName, jobID, api.StageComplete, 100.0)
	} else {
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, batchSize)
		err := d.Nack(false, true)
		if err != nil {
			log.Printf("│ ERROR nack: %v", err)
		}
		r.updateProgress(appName, jobID, api.StageError, 0.0)
	}

	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", jobID)
}
