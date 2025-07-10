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
	BatchSize  int
	Batch      []api.DataFile
	EventQueue chan api.Event
	FileSource FileSource
	Utils      common.UtilsInterface
	Side       string
	EventID    string
}

func NewProducer(batchSize int, fileSource FileSource, eventQueue chan api.Event, utils common.UtilsInterface, side string) *Producer {
	return &Producer{
		BatchSize:  batchSize,
		Batch:      make([]api.DataFile, 0, batchSize),
		EventQueue: eventQueue,
		FileSource: fileSource,
		Utils:      utils,
		Side:       side,
		EventID:    utils.GenerateUUID(),
	}

}

var starlight app.StarlightInterface = &app.Starlight{
	Utils: &common.Utils{},
}
var send sender.EventSender = &sender.RabbitMQSender{}

func (p *Producer) CreateEvent(appName string, side string, q queue.QueueInterface) {
	go func() {
		for event := range p.EventQueue {
			log.Printf("Sending event (ID: %s) with %d files\n", p.EventID, len(event.Files))
			send.SendEvent(event, appName, side, q)
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

func (p *Producer) SendBatch(appName string) {
	if len(p.Batch) > 0 {
		// For STARLIGHT producer side, check if we have actual spectrum files, not just placeholder files
		if appName == "STARLIGHT" && p.Side == "producer" {
			// Count non-placeholder files in the batch
			spectrumFileCount := 0
			for _, file := range p.Batch {
				if !strings.Contains(file.Name, ".batch_placeholder") &&
					!strings.Contains(file.Name, ".dataset_placeholder") &&
					!strings.HasSuffix(file.Name, ".in") {
					spectrumFileCount++
				}
			}

			// Only proceed if we have actual spectrum files
			if spectrumFileCount == 0 {
				log.Printf("Skipping batch with only placeholder files - no spectrum files found")
				// Still delete the processed files (move placeholder to processed)
				p.DeleteProcessedFiles()
				// Clear the batch
				p.Batch = make([]api.DataFile, 0, p.BatchSize)
				return
			}
		}

		// Update the .in file before sending the batch
		if appName == "STARLIGHT" && p.Side == "producer" {
			inFileName, content := starlight.UpdateInFile(p.Batch)
			println(inFileName)
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
		} else {
			log.Printf("Successfully moved file %s to processed dir", file.Name)
		}
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
