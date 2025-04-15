package common

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ReceiverInterface interface {
	Start()
	ProcessQueue(queueName string)
	ProcessMessage(queueName string, d amqp.Delivery)
}

type Receiver struct {
	Queue     common.QueueInterface
	AppQueues []string
}
type FileData struct {
	Name    string `json:"Name"`
	Content string `json:"Content"`
}

type MessageBody struct {
	Files []FileData `json:"Files"`
}

func NewReceiver(queue common.QueueInterface, queues []string) *Receiver {
	return &Receiver{
		Queue:     queue,
		AppQueues: queues,
	}
}

func (r *Receiver) Start() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	err := r.Queue.Connect()
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer r.Queue.Close()

	for _, q := range r.AppQueues {
		err := r.Queue.DeclareQueue(q)
		utils.FailOnError(err, fmt.Sprintf("Failed to declare queue: %s", q))
	}

	err = r.Queue.SetQoS(1)
	utils.FailOnError(err, "Failed to set QoS")

	for {
		for _, q := range r.AppQueues {
			r.ProcessQueue(q)
		}
		time.Sleep(1 * time.Second)
	}
}

func (r *Receiver) ProcessQueue(queueName string) {
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
	defer r.Queue.CancelConsumer(consumerTag)

	timeout := time.After(5 * time.Second)
	for {
		select {
		case d, ok := <-msgs:
			if !ok {
				return
			}
			r.ProcessMessage(queueName, d)
		case <-timeout:
			return
		}
	}
}
func (r *Receiver) ProcessMessage(queue string, d amqp.Delivery) {
	processStart := time.Now()
	batchID := fmt.Sprintf("%s-%d", queue, d.DeliveryTag)

	log.Printf("\n■■■ BATCH START [%s] ■■■", batchID)
	log.Printf("│ Queue:      %s", queue)
	log.Printf("│ DeliveryTag: %d", d.DeliveryTag)
	log.Printf("│ Timestamp:   %s", d.Timestamp)

	if len(d.Headers) > 0 {
		headers, _ := json.Marshal(d.Headers)
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
	switch queue {
	case "starlight":
		outputPath = os.Getenv("INPUT_DIR_STARLIGHT")
	case "ppfx":
		outputPath = os.Getenv("INPUT_DIR_PPFX")
	case "steckmap":
		outputPath = os.Getenv("INPUT_DIR_STECKMAP")
	default:
		log.Printf("│ ERROR: Unknown queue: %s", queue)
		r.requeueWithLog(d, batchID)
		return
	}

	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for queue")
		r.requeueWithLog(d, batchID)
		return
	}

	var msgBody MessageBody
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

	if exists, _ := utils.Exists(outputPath); !exists {
		err := os.Mkdir(outputPath, 0700)
		if err != nil {
			log.Printf("│ ERROR creating directory: %v", err)
			r.requeueWithLog(d, batchID)
			return
		}
		log.Printf("│ Created output directory: %s", outputPath)
	}

	successCount := 0
	for _, file := range msgBody.Files {
		if strings.HasSuffix(file.Name, ".in") {
			starlight.UpdateToProcessList(file.Name, []byte(file.Content))
			log.Printf("│ ✓ Processed .in file: %s", file.Name)
			successCount++
			continue
		}

		filePath := filepath.Join(outputPath, file.Name)
		err := os.WriteFile(filePath, []byte(file.Content), 0644)
		if err != nil {
			log.Printf("│ ✗ Error writing file %s: %v", file.Name, err)
		} else {
			log.Printf("│ ✓ Wrote file: %s", file.Name)
			successCount++
		}
	}

	if successCount == int(batchSize) {
		d.Ack(false)
		log.Printf("│ ✔ Successfully processed all %d files", batchSize)
	} else {
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, batchSize)
		d.Nack(false, true)
	}

	log.Printf("│ Duration: %s", time.Since(processStart))
	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", batchID)
}
func (r *Receiver) requeueWithLog(d amqp.Delivery, batchID string) {
	d.Nack(false, true)
	log.Printf("■■■ BATCH ERROR [%s] - Message requeued ■■■", batchID)
}
