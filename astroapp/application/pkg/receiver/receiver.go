package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	Queue              queue.QueueInterface
	Utils              common.UtilsInterface
	Bucket             s3bucket.S3BucketInterface
	RedisClient        *metrics.RedisClient
	AggregationService *metrics.AggregationService
}

func NewReceiver(queue queue.QueueInterface, utils common.UtilsInterface, bucket s3bucket.S3BucketInterface, redisClient *metrics.RedisClient) *Receiver {
	metricsStore := metrics.NewMetricsStore(redisClient, 168*time.Hour)
	aggregationService := metrics.NewAggregationService(metricsStore)

	return &Receiver{
		Queue:              queue,
		Utils:              utils,
		Bucket:             bucket,
		RedisClient:        redisClient,
		AggregationService: aggregationService,
	}
}

func (r *Receiver) Start(side string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Clear process list on startup to prevent processing old entries
	if side == "processor" {
		r.clearProcessList()
	}

	// Start aggregation service in background (only on processor side to avoid duplication)
	if side == "processor" && r.AggregationService != nil {
		ctx := context.Background()
		go func() {
			// Wait a bit before starting aggregation to let the system initialize
			time.Sleep(30 * time.Second)
			// Run aggregation every 5 minutes
			r.AggregationService.RunPeriodicAggregation(ctx, 5*time.Minute)
		}()
		log.Printf("🔄 Started event metrics aggregation service (5-minute intervals)")
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
	// Get the app_name from headers to determine processing logic
	appName, ok := d.Headers["app_name"].(string)
	if !ok {
		log.Printf("│ ERROR: 'app_name' header missing or invalid")
		r.requeueWithLog(d, "unknown-app")
		return
	}

	// Get event_id and batch_id from headers
	eventID, ok := d.Headers["event_id"].(string)
	if !ok {
		log.Printf("│ ERROR: 'event_id' header missing or invalid")
		r.requeueWithLog(d, "unknown-event")
		return
	}

	batchID, ok := d.Headers["batch_id"].(string)
	if !ok {
		log.Printf("│ ERROR: 'batch_id' header missing or invalid")
		r.requeueWithLog(d, "unknown-batch")
		return
	}

	log.Printf("\n■■■ BATCH START [%s] ■■■", batchID)
	log.Printf("│ App:        %s", appName)
	log.Printf("│ Event ID:   %s", eventID)
	log.Printf("│ Batch ID:   %s", batchID)

	// Record queue first receive time (when first batch is received)
	if side == "processor" {
		log.Printf("│ DEBUG: Recording QueueFirstReceiveTime for processor side")
		if r.RedisClient != nil {
			ctx := context.Background()
			metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)

			// Check if metric already exists
			key := metricsStore.GetBatchKey(eventID, batchID)
			log.Printf("│ DEBUG: Redis key = %s", key)
			exists, err := r.RedisClient.Exists(ctx, key)
			if err != nil {
				log.Printf("│ ✗ Failed to check metric existence: %v", err)
			} else if exists == 0 {
				// Create new metric record with queue first receive time
				log.Printf("│ DEBUG: Creating new metric record")
				err = metricsStore.RecordMetric(ctx, &metrics.MetricRecord{
					EventID:               eventID,
					BatchID:               batchID,
					QueueFirstReceiveTime: time.Now(),
				})
				if err != nil {
					log.Printf("│ ✗ Failed to record queue first receive time: %v", err)
				} else {
					log.Printf("│ ✓ Recorded queue first receive time for event %s, batch %s", eventID, batchID)
				}
			} else {
				log.Printf("│ DEBUG: Metric already exists, checking if queue_first_receive_time is set")
				// Check if queue_first_receive_time is already set by getting all fields
				allFields, err := r.RedisClient.HGetAll(ctx, key)
				if err != nil {
					log.Printf("│ ✗ Failed to get metric fields: %v", err)
				} else {
					currentTime := allFields["queue_first_receive_time"]
					log.Printf("│ DEBUG: Current queue_first_receive_time = '%s'", currentTime)
					if currentTime == "" || currentTime == "0001-01-01T00:00:00Z" {
						err = metricsStore.UpdateMetricField(ctx, eventID, batchID,
							"queue_first_receive_time", time.Now())
						if err != nil {
							log.Printf("│ ✗ Failed to update queue first receive time: %v", err)
						} else {
							log.Printf("│ ✓ Updated queue first receive time for event %s, batch %s", eventID, batchID)
						}
					}
				}
			}
		}
	}

	// Update progress to processing stage
	r.updateProgress(appName, batchID, api.StageProcessing, 20.0)

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
		case "PPXF":
			outputPath = os.Getenv("OUTPUT_BUCKET_PPXF")
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
		case "PPXF":
			outputPath = os.Getenv("EXPLORED_DIR_PPXF")
		case "STECKMAP":
			outputPath = os.Getenv("EXPLORED_DIR_STECKMAP")
		default:
			log.Printf("│ ERROR: Unknown app: %s", appName)
			r.requeueWithLog(d, batchID)
			return
		}
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
	var inFileName string
	var inFileContent string
	var spectrumFiles []api.DataFile

	starlight := app.NewStarlight([]api.DataFile{}, r.Utils)
	ppxf := app.NewPPXF(r.Utils)
	for _, file := range msgBody.Files {
		// Skip hidden files (files starting with .)
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden file: %s", file.Name)
			successCount++
			continue
		}

		if strings.HasSuffix(file.Name, ".in") {
			// Store the .in file name but don't process it yet
			inFileName = file.Name
			inFileContent = string(file.Content)
			log.Printf("│ ✓ Found .in file: %s", file.Name)
			successCount++
			continue
		}

		// Collect spectrum files for later .in file generation (exclude hidden files and .in files)
		if !strings.HasSuffix(file.Name, ".in") && !strings.HasPrefix(filepath.Base(file.Name), ".") {
			spectrumFiles = append(spectrumFiles, file)
		}
		if side == "producer" {
			// Handle bucket upload for producer side
			// Extract batch name from filename (e.g., NGC7025 from output_NGC7025_LR-V_final_cube_voronoi_cell_0.txt)
			batchName := r.extractBatchNameFromFilename(file.Name)
			var uploadPath string
			if batchName != "" {
				// Include batch name in the upload path: starlight/output/NGC7025/filename.txt
				uploadPath = filepath.Join(outputPath, batchName, file.Name)
			} else {
				// Fallback to original path if batch name cannot be extracted
				uploadPath = filepath.Join(outputPath, file.Name)
			}

			// Split uploadPath into folder and filename for S3 upload
			folderPath := filepath.Dir(uploadPath)
			fileName := filepath.Base(uploadPath)

			// Handle root path case
			if folderPath == "." {
				folderPath = ""
			}

			log.Printf("│ DEBUG: S3 upload - folderPath: '%s', fileName: '%s'", folderPath, fileName)

			err := r.Bucket.UploadFileToBucket(folderPath, fileName, []byte(file.Content))
			if err != nil {
				log.Printf("│ ✗ Error uploading file %s to bucket: %v", uploadPath, err)
			} else {
				log.Printf("│ ✓ Uploaded file to bucket: %s", uploadPath)
				successCount++
			}

			// Record batch end time when processing is complete (on producer side)
			// Use the extracted batch name as the event ID (e.g., NGC7025)
			if r.RedisClient != nil && batchName != "" {
				metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)
				err = metricsStore.UpdateMetricField(context.Background(), batchName, batchID, "batch_end_time", time.Now())
				if err != nil {
					log.Printf("│ ✗ Failed to record batch end time: %v", err)
				} else {
					log.Printf("│ ✓ Recorded batch end time for event %s, batch %s", batchName, batchID)
				}
			}

		} else {
			// Handle local file write for processor side
			// Store files flat using just the basename to match .in file references
			filename := filepath.Base(file.Name)
			filePath := filepath.Join(outputPath, filename)

			err := os.WriteFile(filePath, []byte(file.Content), 0644)
			if err != nil {
				log.Printf("│ ✗ Error writing file %s: %v", filePath, err)
			} else {
				log.Printf("│ ✓ Wrote file: %s to %s", file.Name, filePath)
				successCount++

				// For pPXF, add each .fits file to the process list immediately
				if appName == "PPXF" && strings.HasSuffix(file.Name, ".fits") {
					ppxf.AddToProcessList(filename)
					log.Printf("│ ✓ Added pPXF file to process list: %s", filename)
				}
			}
		}
	}

	// Generate and process .in file after all files are written (for STARLIGHT processor side)
	// Only generate .in files from spectrum files, NOT when receiving .in files from input side
	if successCount == int(batchSize) && appName == "STARLIGHT" && side == "processor" && len(spectrumFiles) > 0 && inFileName == "" {
		// Use only the spectrum files from the current message batch, not all files in the directory
		// This prevents the continuous loop where .in files keep growing with previously processed files
		if len(spectrumFiles) > 0 {
			// Generate new .in file with only the spectrum files from this batch
			newInFileName, newInContent := starlight.UpdateInFile(spectrumFiles)
			if newInFileName != "" && newInContent != "" {
				// Add the newly generated .in file to the processlist
				starlight.UpdateToProcessList(newInFileName, []byte(newInContent))
				log.Printf("│ ✓ Generated and added .in file to processlist: %s with %d spectrum files from current batch", newInFileName, len(spectrumFiles))

				// Update progress to analysis stage when .in file is processed
				r.updateProgress(appName, batchID, api.StageAnalysis, 70.0)
			} else {
				log.Printf("│ ⚠ Failed to generate .in file content")
			}
		} else {
			log.Printf("│ ⚠ No spectrum files found in current batch")
		}
	} else if successCount == int(batchSize) && appName == "STARLIGHT" && side == "processor" && inFileName != "" {
		// If we received a .in file from input side, ADD it to processlist - this is the work to be done
		starlight.UpdateToProcessList(inFileName, []byte(inFileContent))
		log.Printf("│ ✓ Added .in file from input side to processlist: %s", inFileName)

		// Update progress to analysis stage when .in file is processed
		r.updateProgress(appName, batchID, api.StageAnalysis, 70.0)
	}

	// Handle pPXF processing (individual files, no .in file generation needed)
	if successCount == int(batchSize) && appName == "PPXF" && side == "processor" {
		log.Printf("│ ✓ All pPXF files processed and added to process list")
		// Update progress to analysis stage for pPXF
		r.updateProgress(appName, batchID, api.StageAnalysis, 70.0)
	}

	if successCount == int(batchSize) {
		err := d.Ack(false)
		if err != nil {
			log.Printf("│ ERROR ack: %v", err)
		}
		log.Printf("│ ✔ Successfully processed all %d files", batchSize)

		// Update progress to completion if all files processed
		r.updateProgress(appName, batchID, api.StageComplete, 100.0)
	} else {
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, batchSize)
		err := d.Nack(false, true)
		if err != nil {
			log.Printf("│ ERROR nack: %v", err)
		}

		// Update progress to error state
		r.updateProgress(appName, batchID, api.StageError, 0.0)
	}

	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", batchID)

	// Remove Redis entry for this batch after completion
	// COMMENTED OUT - Keep Redis entries for debugging
	/*
		if r.RedisClient != nil {
			metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)
			key := metricsStore.GetBatchKey(eventID, batchID)
			err := r.RedisClient.Del(context.Background(), key)
			if err != nil {
				log.Printf("│ ⚠ Failed to remove Redis entry for batch %s: %v", batchID, err)
			} else {
				log.Printf("│ ✓ Removed Redis entry for batch %s", batchID)
			}
		}
	*/
}

func (r *Receiver) requeueWithLog(d amqp.Delivery, batchID string) {
	err := d.Nack(false, true)
	if err != nil {
		log.Printf("│ ERROR nack: %v", err)
	}
	log.Printf("■■■ BATCH ERROR [%s] - Message requeued ■■■", batchID)
}

// extractBatchNameFromFilename extracts the batch name from output filenames
// STARLIGHT: "output_NGC7025_LR-V_final_cube_voronoi_cell_0.txt" -> "NGC7025"
// pPXF: "NGC7025_LR-V_final_cube_voronoi_cell_0_kinematics_and_stellar_pops_info.txt" -> "NGC7025"
func (r *Receiver) extractBatchNameFromFilename(filename string) string {
	// Remove file extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	log.Printf("│ DEBUG: Extracting batch name from filename: %s (without extension: %s)", filename, name)

	// Check if it's a STARLIGHT output file pattern
	if strings.HasPrefix(name, "output_") && strings.Contains(name, "_LR-V") {
		// Extract the part between "output_" and "_LR-V"
		parts := strings.Split(name, "_LR-V")
		if len(parts) > 0 {
			prefixPart := parts[0]
			if strings.HasPrefix(prefixPart, "output_") {
				batchName := strings.TrimPrefix(prefixPart, "output_")
				log.Printf("│ DEBUG: STARLIGHT batch name extracted: %s", batchName)
				return batchName
			}
		}
	}

	// Check if it's a pPXF output file pattern
	// pPXF files start with batch name: "NGC7025_LR-V_final_cube_voronoi_cell_0_..."
	if strings.Contains(name, "_LR-V_final_cube_voronoi_cell_") {
		// Extract the part before "_LR-V"
		parts := strings.Split(name, "_LR-V")
		if len(parts) > 0 && parts[0] != "" {
			batchName := parts[0]
			log.Printf("│ DEBUG: pPXF batch name extracted: %s", batchName)
			return batchName
		}
	}

	// More generic pattern for pPXF files - look for anything before "_LR-V" or "_LR-R"
	if strings.Contains(name, "_LR-V") || strings.Contains(name, "_LR-R") {
		var separator string
		if strings.Contains(name, "_LR-V") {
			separator = "_LR-V"
		} else {
			separator = "_LR-R"
		}

		parts := strings.Split(name, separator)
		if len(parts) > 0 && parts[0] != "" {
			batchName := parts[0]
			log.Printf("│ DEBUG: Generic pPXF batch name extracted: %s", batchName)
			return batchName
		}
	}

	// For other file patterns, try to extract from different patterns
	// This could be extended for STECKMAP if needed

	log.Printf("│ DEBUG: No batch name could be extracted from filename: %s", filename)
	return "" // Return empty if no batch name can be extracted
}

// updateProgress sends a progress update to the API server
func (r *Receiver) updateProgress(appName, batchID string, stage api.PipelineStage, progress float64) {
	request := api.ProgressUpdateRequest{
		DatasetID:   batchID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
	}

	// Send progress update to local API server
	go func() {
		jsonData, err := json.Marshal(request)
		if err != nil {
			log.Printf("Error marshaling progress update: %v", err)
			return
		}

		apiURL := "http://localhost:8080/api/progress/update"
		if serverURL := os.Getenv("API_SERVER_URL"); serverURL != "" {
			apiURL = serverURL + "/api/progress/update"
		}

		resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error sending progress update: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Progress update returned non-200 status: %d", resp.StatusCode)
		}
	}()
}

// clearProcessList removes all entries from the process list file on startup
// This only runs once per deployment using a lock file mechanism
func (r *Receiver) clearProcessList() {
	processListPath := os.Getenv("PROCESS_LIST")
	if processListPath == "" {
		log.Printf("│ ⚠ PROCESS_LIST environment variable not set, skipping cleanup")
		return
	}

	// Use a lock file to ensure cleanup only happens once per deployment
	lockFilePath := "/processing_data/starlight/runtime/.cleanup_done"

	// Check if cleanup has already been done
	if _, err := os.Stat(lockFilePath); err == nil {
		log.Printf("│ ✓ Cleanup already performed (lock file exists), skipping")
		return
	}

	// Clear the process list file
	err := os.WriteFile(processListPath, []byte(""), 0644)
	if err != nil {
		log.Printf("│ ⚠ Failed to clear process list file %s: %v", processListPath, err)
		return
	}

	log.Printf("│ ✓ Cleared process list file: %s", processListPath)

	// Clean up old .in files from runtime directory
	inFileOutputPath := os.Getenv("IN_FILE_OUTPUT_PATH")
	if inFileOutputPath == "" {
		log.Printf("│ ⚠ IN_FILE_OUTPUT_PATH environment variable not set, skipping .in file cleanup")
		return
	}

	// Get runtime directory (parent of infiles directory)
	runtimeDir := filepath.Dir(inFileOutputPath)

	// Find all old .in files in the runtime directory
	pattern := filepath.Join(runtimeDir, "infilesgrid_example_*.in")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("│ ⚠ Failed to find old .in files: %v", err)
		return
	}

	if len(matches) > 0 {
		log.Printf("│ 🧹 Found %d old .in files to clean up", len(matches))
		for _, file := range matches {
			if err := os.Remove(file); err != nil {
				log.Printf("│ ⚠ Failed to remove old .in file %s: %v", file, err)
			} else {
				log.Printf("│ ✓ Removed old .in file: %s", filepath.Base(file))
			}
		}
	} else {
		log.Printf("│ ✓ No old .in files found to clean up")
	}

	// Create lock file to indicate cleanup is complete
	err = os.WriteFile(lockFilePath, []byte("cleanup completed"), 0644)
	if err != nil {
		log.Printf("│ ⚠ Failed to create cleanup lock file: %v", err)
	} else {
		log.Printf("│ ✓ Created cleanup lock file: %s", lockFilePath)
	}
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
