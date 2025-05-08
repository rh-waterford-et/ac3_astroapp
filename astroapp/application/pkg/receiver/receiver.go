package receiver

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type ReceiverInterface interface {
	Start()
	ProcessMessages()
	ProcessMessage(d amqp.Delivery)
}

type Receiver struct {
	Queue  queue.QueueInterface
	Utils  common.UtilsInterface
	Bucket s3bucket.S3BucketInterface
}

func NewReceiver(queue queue.QueueInterface, utils common.UtilsInterface, bucket s3bucket.S3BucketInterface) *Receiver {
	return &Receiver{
		Queue:  queue,
		Utils:  utils,
		Bucket: bucket,
	}
}

func (r *Receiver) Start(side string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

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

	queueInfo, err := r.Queue.InspectQueue(queueName)
	if err != nil {
		log.Printf("QUEUE ERROR: Failed to inspect queue %s: %v", queueName, err)
		return
	}

	if queueInfo.Messages == 0 {
		return
	}

	log.Printf("\n==============================================")
	log.Printf("PROCESSING QUEUE: %s (%d messages)", queueName, queueInfo.Messages)
	log.Printf("==============================================")

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

	timeout := time.After(5 * time.Second)
	for {
		select {
		case d, ok := <-msgs:
			if !ok {
				return
			}
			r.ProcessMessage(d, side)
		case <-timeout:
			return
		}
	}
}

func (r *Receiver) ProcessMessage(d amqp.Delivery, side string) {
	processStart := time.Now()

	// Get the app_name from headers to determine processing logic
	appName, ok := d.Headers["app_name"].(string)
	if !ok {
		log.Printf("│ ERROR: 'app_name' header missing or invalid")
		r.requeueWithLog(d, "unknown-app")
		return
	}

	batchID := fmt.Sprintf("%s-%d", appName, d.DeliveryTag)

	log.Printf("\n■■■ BATCH START [%s] ■■■", batchID)
	log.Printf("│ App:        %s", appName)
	log.Printf("│ DeliveryTag: %d", d.DeliveryTag)
	log.Printf("│ Timestamp:   %s", d.Timestamp)

	if len(d.Headers) > 0 {
		headers, err := json.Marshal(d.Headers)
		if err != nil {
			log.Printf("│ ERROR: marshaling json: %v", err)
		}
		log.Printf("│ Headers:    %s", headers)
	}

	batchSize, ok := d.Headers["batch_size"].(int32)
	if !ok {
		log.Printf("│ ERROR: 'batch_size' header missing or invalid")
		r.requeueWithLog(d, batchID)
		return
	}

	filenamesHeader, ok := d.Headers["filenames"].(string)
	if !ok {
		log.Printf("│ ERROR: 'filenames' header missing or invalid")
		r.requeueWithLog(d, batchID)
		return
	}

	filenames := strings.Split(filenamesHeader, ",")
	if len(filenames) != int(batchSize) {
		log.Printf("│ ERROR: Filenames count doesn't match batch_size")
		r.requeueWithLog(d, batchID)
		return
	}

	log.Printf("│ Processing batch of %d files:", batchSize)
	for i, filename := range filenames {
		log.Printf("│ %d. %s", i+1, filename)
	}

	var outputPath string
	if side == "producer" {
		switch appName {
		case "STARLIGHT":
			outputPath = os.Getenv("OUTPUT_BUCKET_STARLIGHT")
		case "PPFX":
			outputPath = os.Getenv("OUTPUT_BUCKET_PPFX")
		case "STECKMAP":
			outputPath = os.Getenv("OUTPUT_BUCKET_STECKMAP")
		default:
			log.Printf("│ ERROR: Unknown app: %s", appName)
			r.requeueWithLog(d, batchID)
			return
		}
	} else {
		switch appName {
		case "STARLIGHT":
			outputPath = os.Getenv("INPUT_DIR_STARLIGHT")
		case "PPFX":
			outputPath = os.Getenv("EXPLORED_DIR_PPFX")
		case "STECKMAP":
			outputPath = os.Getenv("EXPLORED_DIR_STECKMAP")
		default:
			log.Printf("│ ERROR: Unknown app: %s", appName)
			r.requeueWithLog(d, batchID)
			return
		}
	}

	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for app")
		r.requeueWithLog(d, batchID)
		return
	}

	var msgBody api.MessageBody
	err := json.Unmarshal(d.Body, &msgBody)
	if err != nil {
		log.Printf("│ ERROR parsing message body: %v", err)
		r.requeueWithLog(d, batchID)
		return
	}

	if len(msgBody.Files) != int(batchSize) {
		log.Printf("│ ERROR: Files count in body doesn't match batch_size")
		r.requeueWithLog(d, batchID)
		return
	}

	successCount := 0

	starlight := app.NewStarlight([]api.DataFile{}, r.Utils)
	for _, file := range msgBody.Files {
		if strings.HasSuffix(file.Name, ".in") {
			starlight.UpdateToProcessList(file.Name, []byte(file.Content))
			log.Printf("│ ✓ Processed .in file: %s", file.Name)
			successCount++
			continue
		}
		if side == "producer" {
			// Handle bucket upload for producer side
			err := r.Bucket.UploadFileToBucket(outputPath, file.Name, []byte(file.Content))
			if err != nil {
				log.Printf("│ ✗ Error uploading file %s to bucket: %v", file.Name, err)
			} else {
				log.Printf("│ ✓ Uploaded file to bucket: %s", file.Name)
				successCount++
			}
		} else {
			// Handle local file write for processor side
			filePath := filepath.Join(outputPath, file.Name)
			err := os.WriteFile(filePath, []byte(file.Content), 0644)
			if err != nil {
				log.Printf("│ ✗ Error writing file %s: %v", file.Name, err)
			} else {
				log.Printf("│ ✓ Wrote file: %s to %s", file.Name, filePath)
				successCount++
			}
		}
	}

	if successCount == int(batchSize) {
		err := d.Ack(false)
		if err != nil {
			log.Printf("│ ERROR ack: %v", err)
		}
		log.Printf("│ ✔ Successfully processed all %d files", batchSize)
	} else {
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, batchSize)
		err := d.Nack(false, true)
		if err != nil {
			log.Printf("│ ERROR nack: %v", err)
		}
	}

	log.Printf("│ Duration: %s", time.Since(processStart))
	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", batchID)
}

func (r *Receiver) requeueWithLog(d amqp.Delivery, batchID string) {
	err := d.Nack(false, true)
	if err != nil {
		log.Printf("│ ERROR nack: %v", err)
	}
	log.Printf("■■■ BATCH ERROR [%s] - Message requeued ■■■", batchID)
}
