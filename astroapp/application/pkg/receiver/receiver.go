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
	"regexp"
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
	aggregationService := metrics.NewAggregationService(metricsStore, 5*time.Minute)

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
			r.AggregationService.Run(ctx)
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
	appName, eventID, batchID, batchN, ok := r.validateHeaders(d)
	if !ok {
		return
	}

	// Log batch start information
	r.logBatchStart(batchN, appName, eventID, batchID)

	// Handle queue metrics for processor side
	if side == "processor" {
		r.recordQueueFirstReceiveTime(eventID, batchID)
	}

	// Process binary messages separately
	if isBinary, _ := d.Headers["is_binary"].(bool); isBinary {
		r.ProcessBinaryMessage(d, side, appName, batchID)
		return
	}

	// Process standard text message
	r.processStandardMessage(d, side, appName, eventID, batchID, batchN)
}

func (r *Receiver) createBatchInfoFile(appName, eventID, batchID, filenamesHeader string) error {

	filePath := filepath.Join(os.Getenv("BATCH_INFO_DIR"), batchID+".txt")

	filenames := strings.Split(filenamesHeader, ",")
	var cleanFilenames []string
	for _, f := range filenames {

		cleanName := filepath.Base(f)

		if strings.HasSuffix(cleanName, ".in") {
			continue
		}

		cleanFilenames = append(cleanFilenames, "output_"+cleanName)
	}

	content := fmt.Sprintf("%s/data/output/\n%s\n%s",
		strings.ToLower(appName),
		eventID,
		strings.Join(cleanFilenames, ", "))

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write batch info file: %v", err)
	}

	log.Printf("│ ✓ Created batch info file: %s", filePath)
	return nil
}

// Helper methods broken down by logical functionality
func (r *Receiver) validateHeaders(d amqp.Delivery) (string, string, string, string, bool) {
	appName, ok := d.Headers["app_name"].(string)
	if !ok {
		log.Printf("│ ERROR: 'app_name' header missing or invalid")
		r.requeueWithLog(d, "unknown-app")
		return "", "", "", "", false
	}

	eventID, ok := d.Headers["event_id"].(string)
	if !ok {
		log.Printf("│ ERROR: 'event_id' header missing or invalid")
		r.requeueWithLog(d, "unknown-event")
		return "", "", "", "", false
	}

	batchID, ok := d.Headers["batch_id"].(string)
	if !ok {
		log.Printf("│ ERROR: 'batch_id' header missing or invalid")
		r.requeueWithLog(d, "unknown-batch")
		return "", "", "", "", false
	}

	batchN := fmt.Sprintf("%s-%d", appName, d.DeliveryTag)
	return appName, eventID, batchID, batchN, true
}

func (r *Receiver) logBatchStart(batchN, appName, eventID, batchID string) {
	log.Printf("\n■■■ BATCH START [%s] ■■■", batchN)
	log.Printf("│ App:        %s", appName)
	log.Printf("│ Event ID:   %s", eventID)
	log.Printf("│ Batch ID:   %s", batchID)
}

func (r *Receiver) recordQueueFirstReceiveTime(eventID, batchID string) {
	if r.RedisClient == nil {
		return
	}

	log.Printf("│ DEBUG: Recording QueueFirstReceiveTime for processor side")
	ctx := context.Background()
	metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)

	key := metricsStore.GetBatchKey(eventID, batchID)
	log.Printf("│ DEBUG: Redis key = %s", key)

	exists, err := r.RedisClient.Exists(ctx, key)
	if err != nil {
		log.Printf("│ ✗ Failed to check metric existence: %v", err)
		return
	}

	if exists == 0 {
		r.createNewMetricRecord(metricsStore, ctx, eventID, batchID)
	} else {
		r.updateExistingMetricRecord(metricsStore, ctx, eventID, batchID, key)
	}
}

func (r *Receiver) createNewMetricRecord(metricsStore *metrics.MetricsStore, ctx context.Context, eventID, batchID string) {
	log.Printf("│ DEBUG: Creating new metric record")
	// When the processor sees a batch first time, set only queue_receive_time; queue_start_time is set by producer at publish time
	err := metricsStore.UpdateMetricField(ctx, eventID, "queue_receive_time", batchID, time.Now())
	if err != nil {
		log.Printf("│ ✗ Failed to record queue receive time: %v", err)
	} else {
		log.Printf("│ ✓ Recorded queue receive time for event %s, batch %s", eventID, batchID)
	}
}

func (r *Receiver) updateExistingMetricRecord(metricsStore *metrics.MetricsStore, ctx context.Context, eventID, batchID, key string) {
	log.Printf("│ DEBUG: Metric already exists, checking if queue_receive_time is set")
	allFields, err := r.RedisClient.HGetAll(ctx, key)
	if err != nil {
		log.Printf("│ ✗ Failed to get metric fields: %v", err)
		return
	}

	currentTime := allFields["queue_receive_time"]
	log.Printf("│ DEBUG: Current queue_receive_time = '%s'", currentTime)
	if currentTime == "" || currentTime == "0001-01-01T00:00:00Z" {
		err = metricsStore.UpdateMetricField(ctx, eventID, "queue_receive_time", batchID, time.Now())
		if err != nil {
			log.Printf("│ ✗ Failed to update queue receive time: %v", err)
		} else {
			log.Printf("│ ✓ Updated queue receive time for event %s", eventID)
		}
	}
}

func (r *Receiver) processStandardMessage(d amqp.Delivery, side, appName, eventID, batchID, batchN string) {
	log.Printf("│ Processing standard text event for %s", appName)
	r.updateProgress(appName, batchID, api.StageProcessing, 20.0)

	// Validate batch metadata
	batchSize, _, ok := r.validateBatchMetadata(d, batchID)
	if !ok {
		return
	}

	// Process message body
	msgBody, ok := r.processMessageBody(d, batchID, batchSize)
	if !ok {
		return
	}

	// Determine output path based on side and app
	outputPath := r.getOutputPath(side, appName, batchID)
	if outputPath == "" {
		return
	}

	// Process files
	successCount, inFileName, inFileContent, spectrumFiles := r.processFiles(msgBody, side, appName, outputPath, batchSize)

	// Handle application-specific processing
	r.handleApplicationProcessing(side, appName, batchID, successCount, batchSize, inFileName, inFileContent, spectrumFiles)

	// Finalize batch processing
	r.finalizeBatchProcessing(d, side, appName, eventID, batchID, batchN, successCount, batchSize)
}

func (r *Receiver) validateBatchMetadata(d amqp.Delivery, batchID string) (int32, []string, bool) {
	// Log headers if present
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
		return 0, nil, false
	}

	filenamesHeader, ok := d.Headers["filenames"].(string)
	if !ok {
		log.Printf("│ ERROR: 'filenames' header missing or invalid")
		r.requeueWithLog(d, batchID)
		return 0, nil, false
	}

	filenames := strings.Split(filenamesHeader, ",")
	if len(filenames) != int(batchSize) {
		log.Printf("│ ERROR: Filenames count doesn't match batch_size")
		r.requeueWithLog(d, batchID)
		return 0, nil, false
	}

	//log.Printf("│ Processing batch of %d files:", batchSize)
	for i, filename := range filenames {
		log.Printf("│ %d. %s", i+1, filename)
	}

	return batchSize, filenames, true
}

func (r *Receiver) processMessageBody(d amqp.Delivery, batchID string, batchSize int32) (api.MessageBody, bool) {
	var msgBody api.MessageBody
	err := json.Unmarshal(d.Body, &msgBody)
	if err != nil {
		log.Printf("│ ERROR parsing message body: %v", err)
		r.requeueWithLog(d, batchID)
		return api.MessageBody{}, false
	}

	if len(msgBody.Files) != int(batchSize) {
		log.Printf("│ ERROR: Files count in body doesn't match batch_size")
		r.requeueWithLog(d, batchID)
		return api.MessageBody{}, false
	}

	return msgBody, true
}

func (r *Receiver) getOutputPath(side, appName, batchID string) string {
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
			return ""
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
			return ""
		}
	}
	return outputPath
}

func (r *Receiver) processFiles(msgBody api.MessageBody, side, appName, outputPath string, batchSize int32) (int, string, string, []api.DataFile) {
	successCount := 0
	var inFileName string
	var inFileContent string
	var spectrumFiles []api.DataFile

	// starlight := app.NewStarlight([]api.DataFile{}, r.Utils)
	ppxf := app.NewPPXF(r.Utils)

	for _, file := range msgBody.Files {
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden file: %s", file.Name)
			successCount++
			continue
		}

		if strings.HasSuffix(file.Name, ".in") {
			inFileName = file.Name
			inFileContent = string(file.Content)
			log.Printf("│ ✓ Found .in file: %s", file.Name)
			successCount++
			continue
		}

		if !strings.HasSuffix(file.Name, ".in") && !strings.HasPrefix(filepath.Base(file.Name), ".") {
			spectrumFiles = append(spectrumFiles, file)
		}

		if side == "producer" {
			successCount += r.handleProducerFile(file, outputPath, appName)
		} else {
			successCount += r.handleProcessorFile(file, outputPath, appName, ppxf)
		}
	}

	return successCount, inFileName, inFileContent, spectrumFiles
}

func (r *Receiver) handleProducerFile(file api.DataFile, outputPath, appName string) int {
	batchName := r.extractBatchNameFromFilename(file.Name)
	var uploadPath string

	if batchName != "" {
		uploadPath = filepath.Join(outputPath, batchName, file.Name)
	} else {
		uploadPath = filepath.Join(outputPath, file.Name)
	}

	folderPath := filepath.Dir(uploadPath)
	fileName := filepath.Base(uploadPath)

	if folderPath == "." {
		folderPath = ""
	}

	//log.Printf("│ DEBUG: S3 upload - folderPath: '%s', fileName: '%s'", folderPath, fileName)

	err := r.Bucket.UploadFileToBucket(folderPath, fileName, []byte(file.Content))
	if err != nil {
		log.Printf("│ ✗ Error uploading file %s to bucket: %v", uploadPath, err)
		return 0
	}

	log.Printf("│ ✓ Uploaded file to bucket: %s", uploadPath)
	return 1
}

func (r *Receiver) handleProcessorFile(file api.DataFile, outputPath, appName string, ppxf *app.PPXF) int {
	filename := filepath.Base(file.Name)
	filePath := filepath.Join(outputPath, filename)

	err := os.WriteFile(filePath, []byte(file.Content), 0644)
	if err != nil {
		log.Printf("│ ✗ Error writing file %s: %v", filePath, err)
		return 0
	}

	log.Printf("│ ✓ Wrote file: %s to %s", file.Name, filePath)

	if appName == "PPXF" && strings.HasSuffix(file.Name, ".fits") {
		ppxf.AddToProcessList(filename)
		log.Printf("│ ✓ Added pPXF file to process list: %s", filename)
	}

	return 1
}

func (r *Receiver) handleApplicationProcessing(side, appName, batchID string, successCount int, batchSize int32, inFileName, inFileContent string, spectrumFiles []api.DataFile) {
	starlight := app.NewStarlight([]api.DataFile{}, r.Utils)

	if successCount == int(batchSize) && appName == "STARLIGHT" && side == "processor" {
		if len(spectrumFiles) > 0 && inFileName == "" {
			r.handleStarlightGeneratedInFile(starlight, spectrumFiles, batchID)
		} else if inFileName != "" {
			r.handleStarlightReceivedInFile(starlight, inFileName, inFileContent, batchID)
		}
	}

	if successCount == int(batchSize) && appName == "PPXF" && side == "processor" {
		log.Printf("│ ✓ All pPXF files processed and added to process list")
		r.updateProgress(appName, batchID, api.StageAnalysis, 70.0)
	}
}

func (r *Receiver) handleStarlightGeneratedInFile(starlight *app.Starlight, spectrumFiles []api.DataFile, batchID string) {
	newInFileName, newInContent := starlight.UpdateInFile(spectrumFiles)
	if newInFileName != "" && newInContent != "" {
		starlight.UpdateToProcessList(newInFileName, []byte(newInContent))
		log.Printf("│ ✓ Generated and added .in file to processlist: %s with %d spectrum files from current batch",
			newInFileName, len(spectrumFiles))
		r.updateProgress("STARLIGHT", batchID, api.StageAnalysis, 70.0)
	} else {
		log.Printf("│ ⚠ Failed to generate .in file content")
	}
}

func (r *Receiver) handleStarlightReceivedInFile(starlight *app.Starlight, inFileName, inFileContent, batchID string) {
	starlight.UpdateToProcessList(inFileName, []byte(inFileContent))
	log.Printf("│ ✓ Added .in file from input side to processlist: %s", inFileName)
	r.updateProgress("STARLIGHT", batchID, api.StageAnalysis, 70.0)
}

func (r *Receiver) finalizeBatchProcessing(d amqp.Delivery, side, appName, eventID, batchID, batchN string, successCount int, batchSize int32) {
	if successCount == int(batchSize) {
		err := d.Ack(false)
		if err != nil {
			log.Printf("│ ERROR ack: %v", err)
		}
		log.Printf("│ ✔ Successfully processed all %d files", batchSize)

		if side == "producer" && r.RedisClient != nil {
			r.recordBatchEndTime(eventID, batchID)
		}
		if side == "processor" {
			if filenamesHeader, ok := d.Headers["filenames"].(string); ok {
				if err := r.createBatchInfoFile(appName, eventID, batchID, filenamesHeader); err != nil {
					log.Printf("│ ⚠ Failed to create batch info file: %v", err)
				}
			}
		}
		r.updateProgress(appName, batchID, api.StageComplete, 100.0)
	} else {
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, batchSize)
		err := d.Nack(false, true)
		if err != nil {
			log.Printf("│ ERROR nack: %v", err)
		}
		r.updateProgress(appName, batchID, api.StageError, 0.0)
	}

	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", batchID)
}

func (r *Receiver) recordBatchEndTime(eventID, batchID string) {
	metricsStore := metrics.NewMetricsStore(r.RedisClient, 168*time.Hour)
	err := metricsStore.UpdateMetricField(context.Background(), eventID, "batch_end_time", batchID, time.Now())
	if err != nil {
		log.Printf("│ ✗ Failed to record batch end time: %v", err)
	} else {
		log.Printf("│ ✓ Recorded batch end time for event %s", eventID)
	}
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

	//log.Printf("│ DEBUG: Extracting batch name from filename: %s (without extension: %s)", filename, name)

	// Check if it's a STARLIGHT output file pattern
	if strings.HasPrefix(name, "output_") && strings.Contains(name, "_LR-V") {
		// Extract the part between "output_" and "_LR-V"
		parts := strings.Split(name, "_LR-V")
		if len(parts) > 0 {
			prefixPart := parts[0]
			if strings.HasPrefix(prefixPart, "output_") {
				batchName := strings.TrimPrefix(prefixPart, "output_")
				//log.Printf("│ DEBUG: STARLIGHT batch name extracted: %s", batchName)
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

// extractCellNumberFromFilename extracts cell number from PPXF output files
// Example: "NGC7025_LR-V_final_cube_voronoi_cell_0_bestfit.fits" -> "0"
// Example: "NGC7025_LR-V_final_cube_voronoi_cell_12_galaxy.fits" -> "12"
func (r *Receiver) extractCellNumberFromFilename(filename string) string {
	// Regex pattern to match "cell_" followed by digits
	cellPattern := regexp.MustCompile(`cell_(\d+)`)
	matches := cellPattern.FindStringSubmatch(filename)

	if len(matches) >= 2 {
		cellNumber := matches[1]
		log.Printf("│ DEBUG: Extracted cell number '%s' from filename: %s", cellNumber, filename)
		return cellNumber
	}

	log.Printf("│ DEBUG: No cell number found in filename: %s", filename)
	return ""
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

// ProcessBinaryMessage handles binary events for PPXF (prevents .fits corruption)
func (r *Receiver) ProcessBinaryMessage(d amqp.Delivery, side string, appName string, batchID string) {
	processStart := time.Now()

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
			r.requeueWithLog(d, batchID)
			return
		}
	} else {
		switch appName {
		case "PPXF":
			outputPath = os.Getenv("EXPLORED_DIR_PPXF")
		default:
			log.Printf("│ ERROR: Unknown binary app: %s", appName)
			r.requeueWithLog(d, batchID)
			return
		}
	}

	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for binary app")
		r.requeueWithLog(d, batchID)
		return
	}

	// Unmarshal as BinaryMessageBody for binary events
	var binaryMsgBody api.BinaryMessageBody
	err := json.Unmarshal(d.Body, &binaryMsgBody)
	if err != nil {
		log.Printf("│ ERROR parsing binary message body: %v", err)
		r.requeueWithLog(d, batchID)
		return
	}

	if len(binaryMsgBody.Files) != int(batchSize) {
		log.Printf("│ ERROR: Binary files count in body doesn't match batch_size")
		r.requeueWithLog(d, batchID)
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
		r.updateProgress(appName, batchID, api.StageAnalysis, 70.0)
	} else {
		log.Printf("│ ⚠ Processed %d of %d binary files", successCount, batchSize)
	}

	processDuration := time.Since(processStart)
	log.Printf("\n■■■ BINARY BATCH END [%s] ■■■ Duration: %v\n", batchID, processDuration)
	d.Ack(false)
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
