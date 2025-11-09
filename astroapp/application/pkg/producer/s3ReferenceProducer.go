package producer

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/sender"
)

// S3ReferenceProducer handles large files via S3 references for VORONOI
// Sends S3 metadata instead of file content to avoid RabbitMQ size limits
type S3ReferenceProducer struct {
	JobSize               int
	S3ReferenceJob        []api.S3ReferenceDataFile
	S3ReferenceBatchQueue chan api.S3ReferenceBatch
	FileSource            FileSource
	Utils                 common.UtilsInterface
	Side                  string
	BatchID               string
}

func NewS3ReferenceProducer(jobSize int, fileSource FileSource, batchQueue chan api.S3ReferenceBatch, utils common.UtilsInterface, side string, batchID string) *S3ReferenceProducer {
	// For S3 reference processing (VORONOI), always use jobSize=1
	// VORONOI processes files individually, not in batches like Starlight
	referenceJobSize := 1

	return &S3ReferenceProducer{
		JobSize:               referenceJobSize,
		S3ReferenceJob:        make([]api.S3ReferenceDataFile, 0, referenceJobSize),
		S3ReferenceBatchQueue: batchQueue,
		FileSource:            fileSource,
		Utils:                 utils,
		Side:                  side,
		BatchID:               batchID,
	}
}

// CreateS3ReferenceBatch handles S3 reference batch processing for VORONOI
func (sp *S3ReferenceProducer) CreateS3ReferenceBatch(appName string, side string, queue queue.QueueInterface, batchQueue chan api.S3ReferenceBatch) {
	// Channel to signal when sender goroutine is done
	done := make(chan struct{})

	go func() {
		defer close(done) // Signal completion

		for batch := range batchQueue {
			log.Printf("Sending S3 reference batch (ID: %s, JobID: %s) with %d files\n", batch.ID, batch.JobID, len(batch.Files))
			sp.SendS3ReferenceBatch(batch, appName, side, queue)
		}
	}()

	sp.ProcessS3ReferenceFiles(appName)

	// Close channel to signal no more batches
	close(batchQueue)

	// Wait for sender goroutine to finish processing all batches
	<-done
	log.Printf("Completed processing for S3 reference job: %s", sp.BatchID)
}

// SendS3ReferenceBatch sends S3 reference batches via RabbitMQ
func (sp *S3ReferenceProducer) SendS3ReferenceBatch(batch api.S3ReferenceBatch, appName string, side string, queue queue.QueueInterface) {
	binarySender := &sender.BinaryRabbitMQSender{}
	binarySender.SendS3ReferenceBatch(batch, appName, side, queue)
}

// AddS3ReferenceFile adds S3 reference to job
func (sp *S3ReferenceProducer) AddS3ReferenceFile(file api.S3ReferenceDataFile, appName string) {
	sp.S3ReferenceJob = append(sp.S3ReferenceJob, file)
	if len(sp.S3ReferenceJob) >= sp.JobSize {
		sp.SendS3ReferenceJob(appName)
	}
}

// SendS3ReferenceJob sends S3 reference job
func (sp *S3ReferenceProducer) SendS3ReferenceJob(appName string) {
	if len(sp.S3ReferenceJob) > 0 {
		// Update progress to queued stage
		sp.updateS3ReferenceProgress(appName, api.StageQueued, 10.0)

		jobID := sp.Utils.GenerateUUID()
		batch := api.S3ReferenceBatch{
			ID:    sp.BatchID,
			JobID: jobID,
			Files: sp.S3ReferenceJob,
		}
		sp.S3ReferenceBatchQueue <- batch

		sp.DeleteProcessedS3ReferenceFiles()
		sp.S3ReferenceJob = make([]api.S3ReferenceDataFile, 0, sp.JobSize)
	}
}

// DeleteProcessedS3ReferenceFiles handles file cleanup for S3 reference jobs
func (sp *S3ReferenceProducer) DeleteProcessedS3ReferenceFiles() {
	for _, file := range sp.S3ReferenceJob {
		err := sp.FileSource.DeleteFile(file.Name)
		if err != nil {
			log.Printf("Error deleting S3 reference file %s: %v\n", file.Name, err)
		} else {
			log.Printf("Successfully moved S3 reference file %s to processed dir", file.Name)
		}
	}
}

// updateS3ReferenceProgress sends progress updates for S3 reference processing
func (sp *S3ReferenceProducer) updateS3ReferenceProgress(appName string, stage api.PipelineStage, progress float64) {
	// Extract dataset name from the first file in the job
	if len(sp.S3ReferenceJob) == 0 {
		return
	}

	// For producer side, we use the app name as dataset ID
	datasetID := appName
	if sp.Side == "producer" {
		datasetID = appName + "_" + sp.BatchID
	}

	request := api.ProgressUpdateRequest{
		DatasetID:   datasetID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
		FilesTotal:  len(sp.S3ReferenceJob),
	}

	// Send progress update to local API server
	go func() {
		jsonData, err := json.Marshal(request)
		if err != nil {
			log.Printf("Error marshaling S3 reference progress update: %v", err)
			return
		}

		apiURL := "http://localhost:8080/api/progress/update"
		if serverURL := os.Getenv("API_SERVER_URL"); serverURL != "" {
			apiURL = serverURL + "/api/progress/update"
		}

		resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error sending S3 reference progress update: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("S3 reference progress update returned non-200 status: %d", resp.StatusCode)
		}
	}()
}

// ProcessS3ReferenceFiles handles VORONOI files by creating S3 references
func (sp *S3ReferenceProducer) ProcessS3ReferenceFiles(appName string) {
	files, err := sp.FileSource.ListFiles()

	if err != nil {
		log.Printf("Failed listing S3 reference files: %v", err)
		return
	}

	for _, filename := range files {
		// Skip config files - they are downloaded directly by the receiver when processing datacubes
		// This avoids queue ordering issues and ensures config is always available with the datacube
		if strings.HasPrefix(filepath.Base(filename), "voronoi_config") && strings.HasSuffix(filename, ".json") {
			log.Printf("Skipping config file (will be downloaded directly by receiver): %s", filename)
			continue
		}

		// Create S3 reference instead of downloading content
		s3Ref := sp.createS3ReferenceFile(filename, appName)
		sp.AddS3ReferenceFile(s3Ref, appName)
	}

	sp.SendS3ReferenceJob(appName)
}

// createS3ReferenceFile constructs S3ReferenceDataFile from filename
func (sp *S3ReferenceProducer) createS3ReferenceFile(filename, appName string) api.S3ReferenceDataFile {
	// Get S3 key: InputDir/filename
	inputDir := sp.FileSource.GetInputDir()
	s3Key := filepath.Join(inputDir, filename)
	bucketName := sp.FileSource.GetBucketName()

	return api.S3ReferenceDataFile{
		Name:     filepath.Base(filename),
		S3Key:    s3Key,
		S3Bucket: bucketName,
		Size:     0, // Not needed for functionality
	}
}
