package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/receiver"
)

const usage = `
	usage: ucm receiver | ucm producer

	command line examples 

	# execute prodcuer
	ucm producer

	# execute receiver
	ucm receiver
`

func main() {

	if len(os.Args) <= 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	switch os.Args[1] {

	case "producer":
		err := LaunchProducer()
		if err != nil {
			os.Exit(1)
		}
	case "receiver":
		err := LaunchReceiver()
		if err != nil {
			os.Exit(1)
		}
	}
}

func LaunchReceiver() error {
	log.Printf("------------------ Starting receiver() ---------------------")
	// Initialize directories first
	utils := &common.Utils{}
	if err := utils.EnsureDirectoriesExist(); err != nil {
		log.Fatalf("Directory initialization failed: %v", err)
	}
	// Create RabbitMQ connection
	queue, err := queue.NewRabbitMQConnection()
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ connection: %v", err)
	}

	appQueues := []string{"starlight", "ppfx", "steckmap"}
	receiver := receiver.NewReceiver(queue, appQueues, utils)
	receiver.Start()

	// Create process list file if specified
	processListPath := os.Getenv("PROCESS_LIST")
	log.Print(processListPath)
	if processListPath != "" {
		parentDir := filepath.Dir(processListPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for process list: %w", err)
		}
		if _, err := os.Stat(processListPath); os.IsNotExist(err) {
			file, err := os.Create(processListPath)
			if err != nil {
				return fmt.Errorf("failed to create process list file: %w", err)
			}
			file.Close()
			log.Printf("Created process list file: %s", processListPath)
		} else if err != nil {
			return fmt.Errorf("failed to check process list file: %w", err)
		}
	}
	return nil
}

func LaunchProducer() error {
	utils := &common.Utils{}

	log.Printf("------------------ Starting producer() ---------------------")
	if err := utils.EnsureDirectoriesExist(); err != nil {
		log.Fatalf("Directory initialization failed: %v", err)
	}

	appQueues := []string{"starlight", "ppfx", "steckmap"}
	app := app.NewAppConfig(appQueues, 10*time.Second, utils)
	for {
		app.RunApps()
		log.Println("Checking for new files...")
		time.Sleep(10 * time.Second)
	}
}
