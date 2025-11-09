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
	Queue             queue.QueueInterface
	Utils             common.UtilsInterface
	Bucket            s3bucket.S3BucketInterface
	RedisClient       *metrics.RedisClient
	ProcessingMessage bool
}

func NewReceiver(queue queue.QueueInterface, utils common.UtilsInterface, bucket s3bucket.S3BucketInterface, redisClient *metrics.RedisClient) *Receiver {
	return &Receiver{
		Queue:             queue,
		Utils:             utils,
		Bucket:            bucket,
		RedisClient:       redisClient,
		ProcessingMessage: false,
	}
}

func (r *Receiver) Start(side string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Clear process list on startup to prevent processing old entries
	processLists := []string{os.Getenv("PROCESS_LIST_STARLIGHT"), os.Getenv("PROCESS_LIST_PPXF"), os.Getenv("PROCESS_LIST_VORONOI")}
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
		//log.Printf("ProcessingMessage: %t", r.ProcessingMessage)
		if r.ProcessingMessage {
			// Check if processlists are now empty - if so, reset flag
			status, err := r.checkProcessLists()
			if err == nil && status {
				log.Printf("│ ✓ All processlists empty - resetting ProcessingMessage flag")
				r.ProcessingMessage = false
			} else {
				return
			}
		}
		status, err := r.checkProcessLists()
		if err != nil {
			log.Printf("Error checking process lists: %v", err)
			return
		}

		if !status {
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

	d, ok, err := r.Queue.GetOne(queueName)
	if err != nil {
		log.Printf("GET ERROR: %v", err)
		return
	}
	if !ok {
		return
	}
	r.ProcessingMessage = true
	r.ProcessMessage(d, side)
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

	// Check for S3 reference messages FIRST (before binary)
	// S3 reference messages are for VORONOI large files
	isS3Ref := false
	if val, ok := d.Headers["is_s3_reference"]; ok {
		switch v := val.(type) {
		case bool:
			isS3Ref = v
		case string:
			isS3Ref = (v == "true" || v == "True" || v == "1")
		}
	}
	if isS3Ref {
		r.ProcessS3ReferenceMessage(d, side, appName, jobID)
		return
	}

	// Process binary messages separately
	if isBinary, _ := d.Headers["is_binary"].(bool); isBinary {
		r.ProcessBinaryMessage(d, side, appName, jobID)
		return
	}

	// Process standard text message
	r.processStandardMessage(d, side, appName, batchID, jobID)
}

func (r *Receiver) finalizeJobProcessing(d amqp.Delivery, side, appName, batchID, jobID string, successCount int, jobSize int32) {
	if successCount == int(jobSize) {
		err := d.Ack(false)
		if err != nil {
			log.Printf("│ ERROR ack: %v", err)
		}
		log.Printf("│ ✔ Successfully processed all %d files", jobSize)

		if side == "producer" && r.RedisClient != nil {
			log.Printf("Recording Job End Time")
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
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, jobSize)
		err := d.Nack(false, true)
		if err != nil {
			log.Printf("│ ERROR nack: %v", err)
		}
		r.updateProgress(appName, jobID, api.StageError, 0.0)
	}

	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", jobID)
}
