package producer

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/sender"
)

// FileSource defines operations for different file sources (local and S3)
type FileSource interface {
	ListFiles() ([]string, error)
	ReadFile(filename string) ([]byte, error)
	DeleteFile(filename string) error
}

// ProducerInterface defines the producer operations
type ProducerInterface interface {
	CreateEvent(appName string, side string, q queue.QueueInterface)
	ProcessFiles(appName string)
}

type Producer struct {
	BatchSize   int
	Batch       []api.DataFile
	EventQueue  chan api.Event
	FileSource  FileSource
	Utils       common.UtilsInterface
	Side        string
	EventID     string
	RedisClient *metrics.RedisClient
	Sender      sender.EventSender
}

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

func NewProducer(batchSize int, fileSource FileSource, eventQueue chan api.Event, utils common.UtilsInterface, side string, eventID string, redisClient *metrics.RedisClient) *Producer {
	return &Producer{
		BatchSize:  batchSize,
		Batch:      make([]api.DataFile, 0, batchSize),
		EventQueue: eventQueue,
		FileSource: fileSource,
		Utils:      utils,
		Side:       side,
		EventID:    eventID,	
		RedisClient: redisClient,
	}
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

var starlight app.StarlightInterface = &app.Starlight{
	Utils: &common.Utils{},
}

func (p *Producer) CreateEvent(appName string, side string, q queue.QueueInterface) {
	go func() {
		for event := range p.EventQueue {
			log.Printf("Sending event (ID: %s) with %d files\n", p.EventID, len(event.Files))
			p.Sender.SendEvent(event, appName, side, q)
		}
	}()

	p.ProcessFiles(appName)
}

func (p *Producer) AddFile(file api.DataFile, appName string) {
	p.Batch = append(p.Batch, file)
	if len(p.Batch) >= p.BatchSize {
		p.SendBatch(appName)
	}
}

// AddBinaryFile handles binary files for PPXF
func (bp *BinaryProducer) AddBinaryFile(file api.BinaryDataFile, appName string) {
	bp.BinaryBatch = append(bp.BinaryBatch, file)
	if len(bp.BinaryBatch) >= bp.BatchSize {
		bp.SendBinaryBatch(appName)
	}
}

func (p *Producer) SendBatch(appName string) {
	if len(p.Batch) > 0 {
		// Update the .in file before sending the batch
		if appName == "STARLIGHT" && p.Side == "producer" {
			inFileName, content := starlight.UpdateInFile(p.Batch)
			println(content)
			if inFileName != "" && content != "" {
				p.Batch = append(p.Batch, api.DataFile{Name: inFileName, Content: content})
			}
		}

		// Update progress to queued stage
		p.updateProgress(appName, api.StageQueued, 10.0)

		event := api.Event{
			ID:    p.EventID,
			Files: p.Batch,
		}
		p.EventQueue <- event

		if appName == "STARLIGHT" && p.Side == "producer" {
			p.Batch = starlight.RemoveInFileFromBatch(p.Batch)
		}

		p.DeleteProcessedFiles()
		p.Batch = make([]api.DataFile, 0, p.BatchSize)
	}
}

func (p *Producer) DeleteProcessedFiles() {
	for _, file := range p.Batch {
		err := p.FileSource.DeleteFile(file.Name)
		if err != nil {
			log.Printf("Error deleting file %s: %v\n", file.Name, err)
		} /* else {
			log.Printf("Successfully moved file %s to processed dir", file.Name)
		} */
	}
}

func (p *Producer) ProcessFiles(appName string) {
	files, err := p.FileSource.ListFiles()

	if err != nil {
		log.Printf("Failed listing files: %v", err)
		return
	}

	for _, filename := range files {
		content, err := p.FileSource.ReadFile(filename)
		if err != nil {
			log.Printf("Error reading file %s: %v\n", filename, err)
			continue
		}
		if strings.Contains(filename, ".batch_placeholder") ||
			strings.Contains(filename, ".dataset_placeholder") ||
			strings.Contains(filename, "mask.txt") {
			continue
		}
		p.AddFile(api.DataFile{Name: filename, Content: string(content)}, appName)
	}

	p.SendBatch(appName)
}

// updateProgress sends a progress update to the API server
func (p *Producer) updateProgress(appName string, stage api.PipelineStage, progress float64) {
	// Extract dataset name from the first file in the batch
	if len(p.Batch) == 0 {
		return
	}

	// For producer side, we use the app name as dataset ID
	datasetID := appName
	if p.Side == "producer" {
		datasetID = appName + "_" + p.EventID
	}

	request := api.ProgressUpdateRequest{
		DatasetID:   datasetID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
		FilesTotal:  len(p.Batch),
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

		bp.DeleteProcessedBinaryFiles()
		bp.BinaryBatch = make([]api.BinaryDataFile, 0, bp.BatchSize)
	}
}

// DeleteProcessedBinaryFiles handles file cleanup for binary batches
func (bp *BinaryProducer) DeleteProcessedBinaryFiles() {
	for _, file := range bp.BinaryBatch {
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
