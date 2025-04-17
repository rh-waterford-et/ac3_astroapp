package app

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/producer"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
)

type AppRunner interface {
	RunApps()
}
type AppConfig struct {
	Apps          []string
	CheckInterval time.Duration
	Utils         common.UtilsInterface
}

func NewAppConfig(apps []string, checkInterval time.Duration, utils common.UtilsInterface) *AppConfig {
	return &AppConfig{
		Apps:          apps,
		CheckInterval: checkInterval,
		Utils:         utils,
	}
}

func (r *AppConfig) RunApps() {
	for {
		for _, appName := range r.Apps {
			RunApp(appName, r.Utils)
		}
		log.Println("Checking for new files...")
		time.Sleep(r.CheckInterval)
	}
}

func RunApp(appName string, utils common.UtilsInterface) {
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
		eventQueue := make(chan api.Event, 10)
		producer := producer.NewProducer(batchSize, inputDir, outputDir, eventQueue, utils)

		producer.CreateEvent(appName, q)

	} else {
		log.Printf("No files found in %s directories\n", appName)
	}
}
