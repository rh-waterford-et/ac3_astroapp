package common

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

type FileData struct {
	Name    string `json:"Name"`
	Content string `json:"Content"`
}

type MessageBody struct {
	Files []FileData `json:"Files"`
}

type ConsumerInterface interface {
	ProcessMessage(d amqp.Delivery) error
	GetOutputPath() string
	HandleSpecialFile(file FileData) bool
}

type BaseConsumer struct {
	QueueName string
}

func NewBaseConsumer(queueName string) *BaseConsumer {
	return &BaseConsumer{QueueName: queueName}
}

func (c *BaseConsumer) GetOutputPath() string {
	switch c.QueueName {
	case "starlight":
		return os.Getenv("INPUT_DIR_STARLIGHT")
	case "ppfx":
		return os.Getenv("INPUT_DIR_PPFX")
	case "steckmap":
		return os.Getenv("INPUT_DIR_STECKMAP")
	default:
		return ""
	}
}

func (c *BaseConsumer) HandleSpecialFile(file FileData) bool {
	return false
}

func (c *BaseConsumer) ProcessMessage(d amqp.Delivery) error {
	batchID := c.QueueName + "-" + string(d.DeliveryTag)
	log.Printf("\n■■■ BATCH START [%s] ■■■", batchID)

	// Validate headers
	batchSize, ok := d.Headers["batch_size"].(int32)
	if !ok {
		log.Printf("│ ERROR: 'batch_size' header missing or invalid")
		return fmt.Errorf("invalid batch_size header")
	}

	filenamesHeader, ok := d.Headers["filenames"].(string)
	if !ok {
		log.Printf("│ ERROR: 'filenames' header missing or invalid")
		return fmt.Errorf("invalid filenames header")
	}

	filenames := strings.Split(filenamesHeader, ",")
	if len(filenames) != int(batchSize) {
		log.Printf("│ ERROR: Filenames count doesn't match batch_size")
		return fmt.Errorf("filenames count mismatch")
	}

	// Parse message body
	var msgBody MessageBody
	if err := json.Unmarshal(d.Body, &msgBody); err != nil {
		log.Printf("│ ERROR parsing message body: %v", err)
		return fmt.Errorf("message parse error: %v", err)
	}

	if len(msgBody.Files) != int(batchSize) {
		log.Printf("│ ERROR: Files count in body doesn't match batch_size")
		return fmt.Errorf("files count mismatch")
	}

	outputPath := c.GetOutputPath()
	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for queue")
		return fmt.Errorf("output path not configured")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		log.Printf("│ ERROR creating directory: %v", err)
		return fmt.Errorf("directory creation error: %v", err)
	}

	// Process files
	successCount := 0
	for _, file := range msgBody.Files {
		// Check if this is a special file handled by the specific consumer
		if c.HandleSpecialFile(file) {
			successCount++
			continue
		}

		// Normal file processing
		filePath := filepath.Join(outputPath, file.Name)
		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			log.Printf("│ ✗ Error writing file %s: %v", file.Name, err)
		} else {
			log.Printf("│ ✓ Wrote file: %s", file.Name)
			successCount++
		}
	}

	if successCount == int(batchSize) {
		log.Printf("│ ✔ Successfully processed all %d files", batchSize)
	} else {
		log.Printf("│ ⚠ Processed %d/%d files successfully", successCount, batchSize)
		return fmt.Errorf("partial processing")
	}

	log.Printf("■■■ BATCH COMPLETE [%s] ■■■", batchID)
	return nil
}
