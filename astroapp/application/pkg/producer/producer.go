package producer

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

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
	GetInputDir() string   // Returns the input directory/path
	GetBucketName() string // Returns the S3 bucket name (empty string for local filesystem)
}

// ProducerInterface defines the producer operations
type ProducerInterface interface {
	CreateBatch(appName string, side string, q queue.QueueInterface)
	ProcessFiles(appName string)
}

type Producer struct {
	JobSize     int
	Job         []api.DataFile
	BatchQueue  chan api.Batch
	FileSource  FileSource
	Utils       common.UtilsInterface
	Side        string
	BatchID     string
	RedisClient *metrics.RedisClient
	Sender      sender.BatchSender
}

func NewProducer(jobSize int, fileSource FileSource, batchQueue chan api.Batch, utils common.UtilsInterface, side string, batchID string, redisClient *metrics.RedisClient) *Producer {
	// Initialize sender with Redis client
	var senderInstance sender.BatchSender
	if redisClient != nil {
		senderInstance = sender.NewRabbitMQSender(nil, utils, redisClient)
	} else {
		senderInstance = &sender.RabbitMQSender{
			Queue:       nil,
			Utils:       utils,
			RedisClient: nil,
		}
	}

	return &Producer{
		JobSize:     jobSize,
		Job:         make([]api.DataFile, 0, jobSize),
		BatchQueue:  batchQueue,
		FileSource:  fileSource,
		Utils:       utils,
		Side:        side,
		BatchID:     batchID,
		RedisClient: redisClient,
		Sender:      senderInstance,
	}
}

var starlight app.StarlightInterface = &app.Starlight{
	Utils: &common.Utils{},
}

func (p *Producer) CreateBatch(appName string, side string, q queue.QueueInterface) {
	// Validate queue interface early
	if q == nil {
		log.Printf("ERROR: Queue interface is nil in CreateBatch for app %s, side %s", appName, side)
		log.Printf("WARNING: Batches will be processed but not sent to RabbitMQ - jobs will be lost!")
	}

	// Channel to signal when sender goroutine is done
	done := make(chan struct{})

	go func() {
		// Panic recovery to prevent pod crashes
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in batch sender goroutine: %v", r)
				log.Printf("Stack trace available in container logs")
				// Don't exit - allow the watcher to continue processing other datasets
			}
			close(done) // Signal completion
		}()

		for batch := range p.BatchQueue {
			log.Printf("Sending batch (ID: %s, JobID: %s) with %d files\n", p.BatchID, batch.JobID, len(batch.Files))

			// Additional safety: wrap SendBatch in its own recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC RECOVERED while sending batch %s (JobID: %s): %v", batch.ID, batch.JobID, r)
						log.Printf("This batch will be lost. Consider reprocessing dataset %s", batch.ID)
					}
				}()

				p.Sender.SendBatch(batch, appName, side, q)
			}()
		}
	}()

	p.ProcessFiles(appName)

	// Close channel to signal no more batches
	close(p.BatchQueue)

	// Wait for sender goroutine to finish processing all batches
	<-done
	log.Printf("Completed processing for job: %s", p.BatchID)
}

func (p *Producer) AddFile(file api.DataFile, appName string) {
	p.Job = append(p.Job, file)
	if len(p.Job) >= p.JobSize {
		p.SendJob(appName)
	}
}

func (p *Producer) SendJob(appName string) {
	if len(p.Job) > 0 {
		// Update the .in file before sending the Job
		if appName == "STARLIGHT" && p.Side == "producer" {
			inFileName, content := starlight.UpdateInFile(p.Job)
			//println(content)
			if inFileName != "" && content != "" {
				p.Job = append(p.Job, api.DataFile{Name: inFileName, Content: content})
			}
		}

		// Update progress to queued stage
		p.updateProgress(appName, api.StageQueued, 10.0)

		jobID := p.Utils.GenerateUUID()
		if p.Side == "processor" {
			// Set JobID to match the directory name
			jobID = p.FileSource.(*LocalFileSource).GetBaseInputDir()
		}
		batch := api.Batch{
			ID:    p.BatchID,
			JobID: jobID,
			Files: p.Job,
		}
		p.BatchQueue <- batch

		if appName == "STARLIGHT" && p.Side == "producer" {
			p.Job = starlight.RemoveInFileFromJob(p.Job)
		}

		p.DeleteProcessedFiles()
		p.Job = make([]api.DataFile, 0, p.JobSize)
	}
}

func (p *Producer) DeleteProcessedFiles() {
	for _, file := range p.Job {
		err := p.FileSource.DeleteFile(file.Name)
		if err != nil {
			log.Printf("Error deleting file %s: %v\n", file.Name, err)
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

	p.SendJob(appName)
}

// updateProgress sends a progress update to the API server
func (p *Producer) updateProgress(appName string, stage api.PipelineStage, progress float64) {
	// Extract dataset name from the first file in the Job
	if len(p.Job) == 0 {
		return
	}

	// For producer side, we use the app name as dataset ID
	datasetID := appName
	if p.Side == "producer" {
		datasetID = appName + "_" + p.BatchID
	}

	request := api.ProgressUpdateRequest{
		DatasetID:   datasetID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
		FilesTotal:  len(p.Job),
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
