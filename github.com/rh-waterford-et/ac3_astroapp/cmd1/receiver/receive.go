package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
)

func main() {
	log.Printf("------------------ Starting receive() ---------------------")
	// Initialize directories first
	if err := ensureDirectoriesExist(); err != nil {
		log.Fatalf("Directory initialization failed: %v", err)
	}
	// Create RabbitMQ connection
	queue, err := common.NewRabbitMQConnection()
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ connection: %v", err)
	}

	appQueues := []string{"starlight", "ppfx", "steckmap"}

	receiver := common.NewReceiver(queue, appQueues)
	receiver.Start()

}

func ensureDirectoriesExist() error {

	requiredDirs := []string{
		os.Getenv("INPUT_DIR_STARLIGHT"),
		os.Getenv("OUTPUT_DIR_STARLIGHT"),
		os.Getenv("INPUT_DIR_PPFX"),
		os.Getenv("OUTPUT_DIR_PPFX"),
		os.Getenv("INPUT_DIR_STECKMAP"),
		os.Getenv("OUTPUT_DIR_STECKMAP"),
		os.Getenv("IN_FILE_PATH"),
	}

	// Create all required directories
	for _, dir := range requiredDirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
		log.Printf("Verified directory: %s", dir)
	}

	// Create process list file if specified
	processListPath := os.Getenv("PROCESS_LIST")
	log.Printf(processListPath)
	if processListPath != "" {
		parentDir := filepath.Dir(processListPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for process list: %v", err)
		}
		if _, err := os.Stat(processListPath); os.IsNotExist(err) {
			file, err := os.Create(processListPath)
			if err != nil {
				return fmt.Errorf("failed to create process list file: %v", err)
			}
			file.Close()
			log.Printf("Created process list file: %s", processListPath)
		} else if err != nil {
			return fmt.Errorf("failed to check process list file: %v", err)
		}
	}

	return nil
}
