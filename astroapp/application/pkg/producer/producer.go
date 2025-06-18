package producer

import (
	"log"

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
		// Update the .in file before sending the batch
		if appName == "STARLIGHT" && p.Side == "producer" {
			inFileName, content := starlight.UpdateInFile(p.Batch)
			println(inFileName)
			println(content)
			if inFileName != "" && content != "" {
				p.Batch = append(p.Batch, api.DataFile{Name: inFileName, Content: content})
			}
		}

		event := api.Event{
			ID : p.EventID,
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
