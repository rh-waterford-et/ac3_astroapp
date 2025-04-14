package common

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
)

type DataFile struct {
	Name    string
	Content string
}

type Event struct {
	Files []DataFile
}

type ProducerInterface interface {
	AddFile(file DataFile)
	SendBatch()
	ReadFiles()
	DeleteProcessedFiles()
	SendEvent()
}

type Producer struct {
	BatchSize  int
	Batch      []DataFile
	EventQueue chan Event
	InputDir   string
	OutputDir  string
}

func NewProducer(batchSize int, inputDir, outputDir string, eventQueue chan Event) *Producer {
	return &Producer{
		BatchSize:  batchSize,
		Batch:      make([]DataFile, 0, batchSize),
		EventQueue: eventQueue,
		InputDir:   inputDir,
		OutputDir:  outputDir,
	}
}

var utils common.UtilsInterface = &common.Utils{}

func (p *Producer) AddFile(file DataFile) {
	p.Batch = append(p.Batch, file)
	if len(p.Batch) >= p.BatchSize {
		p.SendBatch()
	}
}

func (p *Producer) SendBatch() {
	if len(p.Batch) > 0 {
		event := Event{Files: p.Batch}
		p.EventQueue <- event
		p.DeleteProcessedFiles()
		p.Batch = make([]DataFile, 0, p.BatchSize)
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

func (p *Producer) ReadFiles() {
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
			p.AddFile(DataFile{Name: file.Name(), Content: string(content)})
		}
	}
}

func send(event Event, appName string, q common.QueueInterface) {
	err := q.Connect()
	if err != nil {
		utils.FailOnError("Failed to connect to RabbitMQ: %v", err)
	}
	defer q.Close()

	err = q.DeclareQueue(appName)
	if err != nil {
		utils.FailOnError("Failed to declare queue %s: %v", appName, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		utils.FailOnError("Failed to marshal event: %v", err)
	}

	headers := make(amqp.Table)
	headers["batch_size"] = len(event.Files)

	var filenames []string
	for _, f := range event.Files {
		filenames = append(filenames, f.Name)
	}
	headers["filenames"] = strings.Join(filenames, ",")

	err = q.Publish(ctx, appName, eventJSON, headers)
	if err != nil {
		utils.FailOnError("Failed to publish message: %v", err)
	}

	log.Printf(" [x] Sent batch with %d files\n", len(event.Files))
	log.Printf("     Files: %s\n", strings.Join(filenames, ", "))
}
