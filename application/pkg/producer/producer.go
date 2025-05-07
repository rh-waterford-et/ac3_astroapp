package producer

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
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
}

func NewProducer(batchSize int, fileSource FileSource, eventQueue chan api.Event, utils common.UtilsInterface) *Producer {
	return &Producer{
		BatchSize:  batchSize,
		Batch:      make([]api.DataFile, 0, batchSize),
		EventQueue: eventQueue,
		FileSource: fileSource,
		Utils:      utils,
	}
}

var starlight app.StarlightInterface = &app.Starlight{
	Batch: make([]api.DataFile, 0),
	Utils: &common.Utils{}, // or whatever your utils implementation is
}
var send sender.EventSender = &sender.RabbitMQSender{}

// LocalFileSource handles local filesystem operations
type LocalFileSource struct {
	InputDir  string
	OutputDir string
}

func (l *LocalFileSource) ListFiles() ([]string, error) {
	entries, err := os.ReadDir(l.InputDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (l *LocalFileSource) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.InputDir, filename))
}

func (l *LocalFileSource) DeleteFile(filename string) error {
	return os.Remove(filepath.Join(l.InputDir, filename))
}

// S3FileSource handles S3 bucket operations
type S3FileSource struct {
	Bucket    s3bucket.S3BucketInterface
	AppName   string
	InputDir  string // S3 prefix
	OutputDir string // S3 prefix
}

func (s *S3FileSource) ListFiles() ([]string, error) {
	return s.Bucket.GetNewAssets(s.InputDir)
}

func (s *S3FileSource) ReadFile(filename string) ([]byte, error) {
	s3Client := s.Bucket.GetS3Client()
	bucketName := s.Bucket.GetBucketName()

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filepath.Join(s.InputDir, filename)),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(result.Body)
}

func (s *S3FileSource) DeleteFile(filename string) error {
	s3Client := s.Bucket.GetS3Client()
	bucketName := s.Bucket.GetBucketName()

	sourceKey := filepath.Join(s.InputDir, filename)
	destinationKey := filepath.Join("processed/", filename)

	// First copy the file to the new location
	_, err := s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		CopySource: aws.String(filepath.Join(bucketName, sourceKey)),
		Key:        aws.String(destinationKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy file to processed directory: %w", err)
	}

	// Then delete the original file
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete original file after copy: %w", err)
	}

	return nil
}

func (p *Producer) CreateEvent(appName string, side string, q queue.QueueInterface) {
	go func() {
		for event := range p.EventQueue {
			log.Printf("Sending event with %d files\n", len(event.Files))
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
		// Update the .in file before sending the batch
		if appName == "STARLIGHT" {
			inFileName, content := starlight.UpdateInFile()
			if inFileName != "" && content != "" {
				p.Batch = append(p.Batch, api.DataFile{Name: inFileName, Content: content})
			}
		}

		event := api.Event{Files: p.Batch}
		p.EventQueue <- event

		if appName == "STARLIGHT" {
			starlight.RemoveInFileFromBatch()
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

	// Send any remaining files in the batch
	p.SendBatch(appName)
}
