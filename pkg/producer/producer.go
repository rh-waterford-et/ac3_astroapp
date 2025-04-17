package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/starlightApp"
)

// TODO: Consider Single responsibility principle and Open/Close principles
// - reduce functions
// - create a new interface
type ProducerInterface interface {
	AddFile(file api.DataFile, appName string)
	SendBatch(appName string)
	ReadFiles(appName string)
	DeleteProcessedFiles()
	SendEvent(event api.Event, appName string, q queue.QueueInterface)
	CreateEvent()
}

type Producer struct {
	BatchSize  int
	Batch      []api.DataFile
	EventQueue chan api.Event
	InputDir   string
	OutputDir  string
	Utils      common.UtilsInterface
}

func NewProducer(batchSize int, inputDir, outputDir string, eventQueue chan api.Event, utils common.UtilsInterface) *Producer {
	return &Producer{
		BatchSize:  batchSize,
		Batch:      make([]api.DataFile, 0, batchSize),
		EventQueue: eventQueue,
		InputDir:   inputDir,
		OutputDir:  outputDir,
		Utils:      utils,
	}
}

var starlight starlightApp.StarlightInterface = &starlightApp.Starlight{}

func (p *Producer) CreateEvent(appName string, q queue.QueueInterface) {
	go func() {
		for event := range p.EventQueue {
			log.Printf("Sent event with %d files\n", len(event.Files))
			p.SendEvent(event, appName, q)
		}
	}()

	p.ReadFiles(appName)
	p.SendBatch(appName)

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
		if appName == "starlight" {
			inFileName, content := starlight.UpdateInFile()
			if inFileName != "" && content != "" {
				p.Batch = append(p.Batch, api.DataFile{Name: inFileName, Content: content})
			}
		}
		event := api.Event{Files: p.Batch}
		p.EventQueue <- event

		if appName == "starlight" {
			starlight.RemoveInFileFromBatch()
		}
		p.DeleteProcessedFiles()
		p.Batch = make([]api.DataFile, 0, p.BatchSize)
	}
}

func (p *Producer) DeleteProcessedFiles() {
	for _, file := range p.Batch {
		filePath := filepath.Join(p.InputDir, file.Name)
		err := os.Remove(filePath)
		if err != nil {
			log.Printf("Error deleting file %s: %v\n", file.Name, err)
		}
	}
}

func (p *Producer) ReadFiles(appName string) {
	files, err := os.ReadDir(p.InputDir)
	if err != nil {
		log.Printf("Failed reading input directory: %v", err)
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(p.InputDir, file.Name()))
			if err != nil {
				log.Printf("Error reading file %s: %v\n", file.Name(), err)
				continue
			}
			p.AddFile(api.DataFile{Name: file.Name(), Content: string(content)}, appName)
		}
	}
}

func (p *Producer) SendEvent(event api.Event, appName string, q queue.QueueInterface) {
	err := q.Connect()
	if err != nil {
		p.Utils.FailOnError("Failed to connect to RabbitMQ", err)
	}
	defer q.Close()

	err = q.DeclareQueue(appName)
	if err != nil {
		p.Utils.FailOnError(fmt.Sprintf("Failed to declare queue %s", appName), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		p.Utils.FailOnError("Failed to marshal event", err)
	}

	headers := make(amqp.Table)
	headers["batch_size"] = len(event.Files)

	filenames := []string{}
	for _, f := range event.Files {
		filenames = append(filenames, f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")

	err = q.Publish(ctx, appName, eventJSON, headers)
	if err != nil {
		p.Utils.FailOnError("Failed to publish message: %v", err)
	}

	log.Printf(" [x] Sent batch with %d files\n", len(event.Files))
	log.Printf("     Files: %s\n", strings.Join(filenames, ", "))
}
