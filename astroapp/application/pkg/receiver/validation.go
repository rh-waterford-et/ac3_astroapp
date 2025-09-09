package receiver

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
)

// Helper methods broken down by logical functionality
func (r *Receiver) validateHeaders(d amqp.Delivery) (string, string, string, string, bool) {
	appName, ok := d.Headers["app_name"].(string)
	if !ok {
		log.Printf("│ ERROR: 'app_name' header missing or invalid")
		r.requeueWithLog(d, "unknown-app")
		return "", "", "", "", false
	}

	batchID, ok := d.Headers["batch_id"].(string)
	if !ok {
		log.Printf("│ ERROR: 'batch_id' header missing or invalid")
		r.requeueWithLog(d, "unknown-batch")
		return "", "", "", "", false
	}

	jobID, ok := d.Headers["job_id"].(string)
	if !ok {
		log.Printf("│ ERROR: 'job_id' header missing or invalid")
		r.requeueWithLog(d, "unknown-job")
		return "", "", "", "", false
	}

	batchN := fmt.Sprintf("%s", appName)
	return appName, batchID, jobID, batchN, true
}

func (r *Receiver) validateJobMetadata(d amqp.Delivery, jobID string) (int32, []string, bool) {
	// Log headers if present
	if len(d.Headers) > 0 {
		headers, err := json.Marshal(d.Headers)
		if err != nil {
			log.Printf("│ ERROR: marshaling json: %v", err)
		}
		log.Printf("│ Headers:    %s", headers)
	}

	jobSize, ok := d.Headers["job_size"].(int32)
	if !ok {
		log.Printf("│ ERROR: 'job_size' header missing or invalid")
		r.requeueWithLog(d, jobID)
		return 0, nil, false
	}

	filenamesHeader, ok := d.Headers["filenames"].(string)
	if !ok {
		log.Printf("│ ERROR: 'filenames' header missing or invalid")
		r.requeueWithLog(d, jobID)
		return 0, nil, false
	}

	filenames := strings.Split(filenamesHeader, ",")
	if len(filenames) != int(jobSize) {
		log.Printf("│ ERROR: Filenames count doesn't match job_size")
		r.requeueWithLog(d, jobID)
		return 0, nil, false
	}

	//log.Printf("│ Processing batch of %d files:", batchSize)
	/* for i, filename := range filenames {
		log.Printf("│ %d. %s", i+1, filename)
	} */

	return jobSize, filenames, true
}

func (r *Receiver) processMessageBody(d amqp.Delivery, jobID string, batchSize int32) (api.MessageBody, bool) {
	var msgBody api.MessageBody
	err := json.Unmarshal(d.Body, &msgBody)
	if err != nil {
		log.Printf("│ ERROR parsing message body: %v", err)
		r.requeueWithLog(d, jobID)
		return api.MessageBody{}, false
	}

	if len(msgBody.Files) != int(batchSize) {
		log.Printf("│ ERROR: Files count in body doesn't match batch_size")
		r.requeueWithLog(d, jobID)
		return api.MessageBody{}, false
	}

	return msgBody, true
}


// ensureQueueConnection validates the queue connection and attempts reconnection if needed
func (r *Receiver) ensureQueueConnection(queueName string) bool {
	if r.Queue == nil {
		log.Printf("QUEUE ERROR: Queue connection is nil, attempting to reconnect...")
		err := r.Queue.Connect()
		if err != nil {
			log.Printf("QUEUE ERROR: Failed to reconnect to RabbitMQ: %v", err)
			return false
		}
		err = r.Queue.DeclareQueue(queueName)
		if err != nil {
			log.Printf("QUEUE ERROR: Failed to redeclare queue after reconnect: %v", err)
			return false
		}
	}

	// Test the connection by trying to inspect the queue
	_, err := r.Queue.InspectQueue(queueName)
	if err != nil {
		log.Printf("QUEUE ERROR: Failed to inspect queue %s: %v", queueName, err)
		// Try to reconnect on inspection failure
		log.Printf("QUEUE ERROR: Attempting to reconnect due to inspection failure...")
		reconnectErr := r.Queue.Connect()
		if reconnectErr != nil {
			log.Printf("QUEUE ERROR: Failed to reconnect: %v", reconnectErr)
			return false
		}
		redeclareErr := r.Queue.DeclareQueue(queueName)
		if redeclareErr != nil {
			log.Printf("QUEUE ERROR: Failed to redeclare queue after reconnect: %v", redeclareErr)
			return false
		}
		// Try inspection again after reconnection
		_, err = r.Queue.InspectQueue(queueName)
		if err != nil {
			log.Printf("QUEUE ERROR: Failed to inspect queue after reconnect: %v", err)
			return false
		}
	}

	return true
}