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
	BatchSize        int
	BinaryBatch      []api.BinaryDataFile
	BinaryEventQueue chan api.BinaryEvent
	FileSource       FileSource
	Utils            common.UtilsInterface
	Side             string
	EventID          string
}

func NewBinaryProducer(batchSize int, fileSource FileSource, eventQueue chan api.BinaryEvent, utils common.UtilsInterface, side string, eventID string) *BinaryProducer {
	return &BinaryProducer{
		BatchSize:        batchSize,
		BinaryBatch:      make([]api.BinaryDataFile, 0, batchSize),
		BinaryEventQueue: eventQueue,
		FileSource:       fileSource,
		Utils:            utils,
		Side:             side,
		EventID:          eventID,
	}
}

// CreateBinaryEvent handles binary event processing for PPXF
func (bp *BinaryProducer) CreateBinaryEvent(appName string, side string, queue queue.QueueInterface, eventQueue chan api.BinaryEvent) {
	go func() {
		for event := range eventQueue {
			log.Printf("Sending binary event (ID: %s) with %d files\n", event.ID, len(event.Files))
			bp.SendBinaryEvent(event, appName, side, queue)
		}
	}()

	bp.ProcessBinaryFiles(appName)
}

// SendBinaryEvent sends binary events via RabbitMQ (handles .fits files safely)
func (bp *BinaryProducer) SendBinaryEvent(event api.BinaryEvent, appName string, side string, queue queue.QueueInterface) {
	binarySender := &sender.BinaryRabbitMQSender{}
	binarySender.SendBinaryEvent(event, appName, side, queue)
}

// AddBinaryFile handles binary files for PPXF
func (bp *BinaryProducer) AddBinaryFile(file api.BinaryDataFile, appName string) {
	bp.BinaryBatch = append(bp.BinaryBatch, file)
	if len(bp.BinaryBatch) >= bp.BatchSize {
		bp.SendBinaryBatch(appName)
	}
}

// SendBinaryBatch handles binary file batches for PPXF
func (bp *BinaryProducer) SendBinaryBatch(appName string) {
	if len(bp.BinaryBatch) > 0 {
		// Update progress to queued stage
		bp.updateBinaryProgress(appName, api.StageQueued, 10.0)

		event := api.BinaryEvent{
			ID:    bp.EventID,
			Files: bp.BinaryBatch,
		}
		bp.BinaryEventQueue <- event

		//bp.DeleteProcessedBinaryFiles()
		bp.BinaryBatch = make([]api.BinaryDataFile, 0, bp.BatchSize)
	}
}

/* // DeleteProcessedBinaryFiles handles file cleanup for binary batches
func (bp *BinaryProducer) DeleteProcessedBinaryFiles() {
	for _, file := range bp.BinaryBatch {
		err := bp.FileSource.DeleteFile(file.Name)
		if err != nil {
			log.Printf("Error deleting binary file %s: %v\n", file.Name, err)
		} else {
			log.Printf("Successfully processed binary file %s (moved to processed dir or already processed)", file.Name)
		}
	}
} */

// updateBinaryProgress sends progress updates for binary processing
func (bp *BinaryProducer) updateBinaryProgress(appName string, stage api.PipelineStage, progress float64) {
	// Extract dataset name from the first file in the batch
	if len(bp.BinaryBatch) == 0 {
		return
	}

	// For producer side, we use the app name as dataset ID
	datasetID := appName
	if bp.Side == "producer" {
		datasetID = appName + "_" + bp.EventID
	}

	request := api.ProgressUpdateRequest{
		DatasetID:   datasetID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
		FilesTotal:  len(bp.BinaryBatch),
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
		// NO STRING CONVERSION - keep as []byte to prevent corruption
		binaryFile := api.BinaryDataFile{
			Name:    filename,
			Content: content, // Keep as []byte - NO string(content)
			Size:    int64(len(content)),
		}
		bp.AddBinaryFile(binaryFile, appName)
	}

	bp.SendBinaryBatch(appName)
}
