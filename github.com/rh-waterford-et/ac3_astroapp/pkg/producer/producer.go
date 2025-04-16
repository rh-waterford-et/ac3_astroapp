package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/utils"
)

type DataFile struct {
	Name    string
	Content string
}

type Event struct {
	Files []DataFile
}

type ProducerInterface interface {
	AddFile(file DataFile, appName string)
	SendBatch(appName string)
	ReadFiles(appName string)
	DeleteProcessedFiles()
	SendEvent(event Event, appName string, q queue.QueueInterface)
	RunApp(appName string)
	CreateEvent()
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

var starlight *starlight.Starlight = &starlight.Starlight{}

func (p *Producer) RunApp(appName string) {
	inputDirEnv := "INPUT_DIR_" + appName
	outputDirEnv := "OUTPUT_DIR_" + appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)

	if inputDir == "" || outputDir == "" {
		log.Printf("%s directories not set\n", appName)
		return
	}

	files, err := os.ReadDir(inputDir)
	if err != nil {
		log.Printf("Error reading %s input directory: %v\n", appName, err)
		return
	}

	batchSize, err := strconv.Atoi(os.Getenv("BATCH_SIZE"))
	if err != nil {
		log.Printf("Invalid batch size for %s: %v\n", appName, err)
		return
	}

	if len(files) > 0 {
		log.Printf("Processing %s files...\n", appName)
		// Initialize the queue connection
		q, err := queue.NewRabbitMQConnection()
		if err != nil {
			log.Printf("Failed to connect to RabbitMQ: %v\n", err)
			return
		}
		defer q.Close()

		p.CreateEvent(inputDir, outputDir, appName, batchSize, q)
	} else {
		log.Printf("No files found in %s directories\n", appName)
	}
}
func (p *Producer) CreateEvent(inputDir string, outputDir string, appName string, batchSize int, q queue.QueueInterface) {

	eventQueue := make(chan Event, 10)

	producer := NewProducer(batchSize, inputDir, outputDir, eventQueue)

	go func() {

		for event := range eventQueue {
			log.Printf("Sent event with %d files\n", len(event.Files))
			p.SendEvent(event, appName, q)
		}
	}()

	producer.ReadFiles(appName)
	producer.SendBatch(appName)

}

func (p *Producer) AddFile(file DataFile, appName string) {
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
			//log.Printf(inFileName)
			//log.Printf(content)
			if inFileName != "" && content != "" {
				p.Batch = append(p.Batch, DataFile{Name: inFileName, Content: content})
			}
		}
		event := Event{Files: p.Batch}
		p.EventQueue <- event

		if appName == "starlight" {
			starlight.RemoveInFileFromBatch()
		}
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
			p.AddFile(DataFile{Name: file.Name(), Content: string(content)}, appName)
		}
	}
}

func (p *Producer) SendEvent(event Event, appName string, q queue.QueueInterface) {
	u := &utils.Utils{}
	err := q.Connect()
	if err != nil {

		u.FailOnError(err, "Failed to connect to RabbitMQ")
	}
	defer q.Close()

	err = q.DeclareQueue(appName)
	if err != nil {
		u.FailOnError(err, fmt.Sprintf("Failed to declare queue %s", appName))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		u.FailOnError(err, "Failed to marshal event")
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
		u.FailOnError(err, "Failed to publish message")
	}

	log.Printf(" [x] Sent batch with %d files\n", len(event.Files))
	log.Printf("     Files: %s\n", strings.Join(filenames, ", "))
}
