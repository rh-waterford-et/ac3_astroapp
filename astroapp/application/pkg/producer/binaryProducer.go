package producer

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/sender"
)

// BinaryProducer handles binary files for PPXF
type BinaryProducer struct {
	JobSize          int
	BinaryJob        []api.BinaryDataFile
	BinaryBatchQueue chan api.BinaryBatch
	FileSource       FileSource
	Utils            common.UtilsInterface
	Side             string
	BatchID          string
}

func NewBinaryProducer(jobSize int, fileSource FileSource, batchQueue chan api.BinaryBatch, utils common.UtilsInterface, side string, batchID string) *BinaryProducer {
	// For binary processing (pPXF), always use jobSize=1
	// pPXF processes files individually, not in batches like Starlight
	binaryJobSize := 1

	return &BinaryProducer{
		JobSize:          binaryJobSize,
		BinaryJob:        make([]api.BinaryDataFile, 0, binaryJobSize),
		BinaryBatchQueue: batchQueue,
		FileSource:       fileSource,
		Utils:            utils,
		Side:             side,
		BatchID:          batchID,
	}
}

// CreateBinaryBatch handles binary batch processing for PPXF
func (bp *BinaryProducer) CreateBinaryBatch(appName string, side string, queue queue.QueueInterface, batchQueue chan api.BinaryBatch) {
	// Channel to signal when sender goroutine is done
	done := make(chan struct{})

	go func() {
		defer close(done) // Signal completion

		for batch := range batchQueue {
			log.Printf("Sending binary batch (ID: %s, JobID: %s) with %d files\n", batch.ID, batch.JobID, len(batch.Files))
			bp.SendBinaryBatch(batch, appName, side, queue)
		}
	}()

	bp.ProcessBinaryFiles(appName)

	// Close channel to signal no more batches
	close(batchQueue)

	// Wait for sender goroutine to finish processing all batches
	<-done
	log.Printf("Completed processing for binary job: %s", bp.BatchID)
}

// SendBinaryBatch sends binary batchs via RabbitMQ (handles .fits files safely)
func (bp *BinaryProducer) SendBinaryBatch(batch api.BinaryBatch, appName string, side string, queue queue.QueueInterface) {
	binarySender := &sender.BinaryRabbitMQSender{}
	binarySender.SendBinaryBatch(batch, appName, side, queue)
}

// AddBinaryFile handles binary files for PPXF
func (bp *BinaryProducer) AddBinaryFile(file api.BinaryDataFile, appName string) {
	bp.BinaryJob = append(bp.BinaryJob, file)
	if len(bp.BinaryJob) >= bp.JobSize {
		bp.SendBinaryJob(appName)
	}
}

// SendBinaryJob handles binary file jobes for PPXF
func (bp *BinaryProducer) SendBinaryJob(appName string) {
	if len(bp.BinaryJob) > 0 {
		// Update progress to queued stage
		bp.updateBinaryProgress(appName, api.StageQueued, 10.0)

		jobID := bp.Utils.GenerateUUID()
		batch := api.BinaryBatch{
			ID:    bp.BatchID,
			JobID: jobID,
			Files: bp.BinaryJob,
		}
		bp.BinaryBatchQueue <- batch

		bp.DeleteProcessedBinaryFiles()
		bp.BinaryJob = make([]api.BinaryDataFile, 0, bp.JobSize)
	}
}

// DeleteProcessedBinaryFiles handles file cleanup for binary jobes
func (bp *BinaryProducer) DeleteProcessedBinaryFiles() {
	for _, file := range bp.BinaryJob {
		err := bp.FileSource.DeleteFile(file.Name)
		if err != nil {
			log.Printf("Error deleting binary file %s: %v\n", file.Name, err)
		} else {
			log.Printf("Successfully moved binary file %s to processed dir", file.Name)
		}
	}
}

// updateBinaryProgress sends progress updates for binary processing
func (bp *BinaryProducer) updateBinaryProgress(appName string, stage api.PipelineStage, progress float64) {
	// Extract dataset name from the first file in the job
	if len(bp.BinaryJob) == 0 {
		return
	}

	// For producer side, we use the app name as dataset ID
	datasetID := appName
	if bp.Side == "producer" {
		datasetID = appName + "_" + bp.BatchID
	}

	request := api.ProgressUpdateRequest{
		DatasetID:   datasetID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
		FilesTotal:  len(bp.BinaryJob),
	}

	// Send progress update to local API server
	go func() {
		jsonData, err := json.Marshal(request)
		if err != nil {
			log.Printf("Error marshaling binary progress update: %v", err)
			return
		}

		apiURL := "http://localhost:8080/api/progress/update"
		if serverURL := os.Getenv("API_SERVER_URL"); serverURL != "" {
			apiURL = serverURL + "/api/progress/update"
		}

		resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error sending binary progress update: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Binary progress update returned non-200 status: %d", resp.StatusCode)
		}
	}()
}

// ProcessBinaryFiles handles PPXF files without string corruption
func (bp *BinaryProducer) ProcessBinaryFiles(appName string) {
	files, err := bp.FileSource.ListFiles()

	if err != nil {
		log.Printf("Failed listing binary files: %v", err)
		return
	}

	for _, filename := range files {
		content, err := bp.FileSource.ReadFile(filename)
		if err != nil {
			log.Printf("Error reading binary file %s: %v\n", filename, err)
			continue
		}
		// NO STRING CONVERSION - keep as []byte to prbatch corruption
		binaryFile := api.BinaryDataFile{
			Name:    filename,
			Content: content, // Keep as []byte - NO string(content)
			Size:    int64(len(content)),
		}
		bp.AddBinaryFile(binaryFile, appName)
	}

	bp.SendBinaryJob(appName)
}
