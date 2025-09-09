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
}

func NewReceiver(queue queue.QueueInterface, utils common.UtilsInterface, bucket s3bucket.S3BucketInterface, redisClient *metrics.RedisClient) *Receiver {
	return &Receiver{
		Queue:       queue,
		Utils:       utils,
		Bucket:      bucket,
		RedisClient: redisClient,
	}
}

func (r *Receiver) Start(side string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Clear process list on startup to prevent processing old entries
	if side == "processor" {
		r.clearProcessList()
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

func (r *Receiver) processStandardMessage(d amqp.Delivery, side, appName, batchID, jobID string) {
	log.Printf("│ Processing standard text batch for %s", appName)
	r.updateProgress(appName, jobID, api.StageProcessing, 20.0)

	// Validate batch metadata
	jobSize, filenames, ok := r.validateJobMetadata(d, jobID)
	if !ok {
		return
	}

	jobSizeMB := r.calculateJobSizeMB(d.Body, filenames)

	// Record job size in metrics

	if side == "processor" && jobSizeMB > 0 && r.RedisClient != nil {
		r.recordJobSize(batchID, jobID, jobSizeMB)
	}
	// Process message body
	msgBody, ok := r.processMessageBody(d, jobID, jobSize)
	if !ok {
		return
	}

	// Determine output path based on side and app
	outputPath := r.getOutputPath(side, appName)
	if outputPath == "" {
		return
	}

	// Process files
	successCount, inFileName, inFileContent, spectrumFiles := r.processFiles(msgBody, side, appName, outputPath)

	// Handle application-specific processing
	r.handleApplicationProcessing(side, appName, jobID, successCount, jobSize, inFileName, inFileContent, spectrumFiles)

	// Finalize batch processing
	r.finalizeBatchProcessing(d, side, appName, batchID, jobID, successCount, jobSize)
}
// ProcessBinaryMessage handles binary events for PPXF (prevents .fits corruption)
func (r *Receiver) ProcessBinaryMessage(d amqp.Delivery, side string, appName string, jobID string) {
	processStart := time.Now()

	// Update progress to processing stage
	r.updateProgress(appName, jobID, api.StageProcessing, 20.0)

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
		r.requeueWithLog(d, jobID)
		return
	}

	filenamesHeader, ok := d.Headers["filenames"].(string)
	if !ok {
		log.Printf("│ ERROR: 'filenames' header missing or invalid")
		r.requeueWithLog(d, jobID)
		return
	}

	filenames := strings.Split(filenamesHeader, ",")
	if len(filenames) != int(batchSize) {
		log.Printf("│ ERROR: Filenames count doesn't match batch_size")
		r.requeueWithLog(d, jobID)
		return
	}
	jobSizeMB := r.calculateBinaryJobSizeMB(d.Body, filenames)
	if batchID, ok := d.Headers["batch_id"].(string); ok && jobSizeMB > 0 && r.RedisClient != nil && side == "processor" {
		r.recordJobSize(batchID, jobID, jobSizeMB)
	}

	log.Printf("│ Processing binary batch of %d files:", batchSize)
	for i, filename := range filenames {
		log.Printf("│ %d. %s", i+1, filename)
	}

	// Get output path for binary files
	var outputPath string
	if side == "producer" {
		switch appName {
		case "PPXF":
			outputPath = os.Getenv("OUTPUT_BUCKET_PPXF")
		default:
			log.Printf("│ ERROR: Unknown binary app: %s", appName)
			r.requeueWithLog(d, jobID)
			return
		}
	} else {
		switch appName {
		case "PPXF":
			outputPath = os.Getenv("EXPLORED_DIR_PPXF")
		default:
			log.Printf("│ ERROR: Unknown binary app: %s", appName)
			r.requeueWithLog(d, jobID)
			return
		}
	}

	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for binary app")
		r.requeueWithLog(d, jobID)
		return
	}

	// Unmarshal as BinaryMessageBody for binary events
	var binaryMsgBody api.BinaryMessageBody
	err := json.Unmarshal(d.Body, &binaryMsgBody)
	if err != nil {
		log.Printf("│ ERROR parsing binary message body: %v", err)
		r.requeueWithLog(d, jobID)
		return
	}

	if len(binaryMsgBody.Files) != int(batchSize) {
		log.Printf("│ ERROR: Binary files count in body doesn't match batch_size")
		r.requeueWithLog(d, jobID)
		return
	}

	successCount := 0
	for _, file := range binaryMsgBody.Files {
		// Skip hidden files (files starting with .)
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden binary file: %s", file.Name)
			successCount++
			continue
		}

		if side == "producer" {
			// Handle S3 upload for binary files
			batchName := r.extractBatchNameFromFilename(file.Name)
			var uploadPath string

			// Special handling for PPXF: organize by cell number
			if appName == "PPXF" {
				log.Printf("│ DEBUG: Processing PPXF file for cell organization: %s", file.Name)
				cellNumber := r.extractCellNumberFromFilename(file.Name)
				log.Printf("│ DEBUG: Cell number extracted: '%s' (batchName: '%s')", cellNumber, batchName)
				if batchName != "" && cellNumber != "" {
					// Path: /ppxf/output/<batch_name>/<cell_number>/filename
					uploadPath = filepath.Join(outputPath, batchName, cellNumber, file.Name)
					log.Printf("│ DEBUG: Using cell-organized path: %s", uploadPath)
				} else if batchName != "" {
					// Fallback: /ppxf/output/<batch_name>/filename
					uploadPath = filepath.Join(outputPath, batchName, file.Name)
					log.Printf("│ DEBUG: Using batch fallback path (missing cell): %s", uploadPath)
				} else {
					// Last resort: flat structure
					uploadPath = filepath.Join(outputPath, file.Name)
					log.Printf("│ DEBUG: Using flat fallback path: %s", uploadPath)
				}
			} else {
				// Standard logic for other apps (Starlight, etc.)
				if batchName != "" {
					uploadPath = filepath.Join(outputPath, batchName, file.Name)
				} else {
					uploadPath = filepath.Join(outputPath, file.Name)
				}
			}

			folderPath := filepath.Dir(uploadPath)
			fileName := filepath.Base(uploadPath)

			if folderPath == "." {
				folderPath = ""
			}

			log.Printf("│ DEBUG: S3 binary upload - folderPath: '%s', fileName: '%s'", folderPath, fileName)

			// Create directory structure if folderPath exists
			if folderPath != "" {
				err := r.Bucket.CreateDirectory(folderPath)
				if err != nil {
					log.Printf("│ ⚠ Error creating directory %s: %v", folderPath, err)
					// Continue with upload even if directory creation fails
				}
			}

			// Upload binary content directly (no string conversion corruption)
			err := r.Bucket.UploadFileToBucket(folderPath, fileName, file.Content)
			if err != nil {
				log.Printf("│ ✗ Error uploading binary file %s to bucket: %v", uploadPath, err)
			} else {
				log.Printf("│ ✓ Uploaded binary file to bucket: %s (%d bytes)", uploadPath, len(file.Content))
				successCount++
			}
		} else {
			// Handle local file write for binary files - NO STRING CONVERSION
			filename := filepath.Base(file.Name)
			filePath := filepath.Join(outputPath, filename)

			// Write binary content directly (preserves .fits integrity)
			err := os.WriteFile(filePath, file.Content, 0644)
			if err != nil {
				log.Printf("│ ✗ Error writing binary file %s: %v", filePath, err)
			} else {
				log.Printf("│ ✓ Wrote binary file: %s to %s (%d bytes)", file.Name, filePath, len(file.Content))
				successCount++

				// For pPXF, add each .fits file to the process list immediately
				if appName == "PPXF" && strings.HasSuffix(file.Name, ".fits") {
					ppxf := app.NewPPXF(r.Utils)
					ppxf.AddToProcessList(filename)
					log.Printf("│ ✓ Added pPXF binary file to process list: %s", filename)
				}
			}
		}
	}

	if successCount == int(batchSize) {
		log.Printf("│ ✓ Successfully processed all %d binary files", successCount)
		// Update progress to analysis stage for pPXF
		r.updateProgress(appName, jobID, api.StageAnalysis, 70.0)
	} else {
		log.Printf("│ ⚠ Processed %d of %d binary files", successCount, batchSize)
	}

	processDuration := time.Since(processStart)
	log.Printf("\n■■■ BINARY BATCH END [%s] ■■■ Duration: %v\n", jobID, processDuration)
	d.Ack(false)
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
