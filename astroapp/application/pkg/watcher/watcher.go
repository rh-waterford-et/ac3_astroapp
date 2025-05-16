package watcher

import (
	"log"
	"os"
	"strconv"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/producer"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type WatcherInterface interface {
	Run(appName string, side string, utils common.UtilsInterface) error
}

type Watcher struct{}

func NewWatcher() *Watcher {
	return &Watcher{}

}

func (w *Watcher) Run(appName string, side string, utils common.UtilsInterface) {
	inputDirEnv := "EXPLORED_" + appName
	outputDirEnv := "OUTPUT_" + appName
	processedDirEnv := "PROCESSED_"+ appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)
	processedDir := os.Getenv(processedDirEnv)

	if inputDir == "" || outputDir == "" {
		log.Printf("%s directories not set\n", appName)
		return
	}

	batchSize, err := strconv.Atoi(os.Getenv("BATCH_SIZE"))
	if err != nil {
		log.Printf("Invalid batch size for %s: %v\n", appName, err)
		return
	}

	var fileSource producer.FileSource
	var length = 0

	switch side {
	case "producer":
		watcher := s3bucket.NewS3Watcher()
		/* // Initialize known assets on first run
		if len(watcher.KnownAssets) == 0 {
			watcher.Bucket.InitializeKnownAssets(inputDir)
		} */
		fileSource = &producer.S3FileSource{
			Bucket:    watcher.Bucket,
			AppName:   appName,
			InputDir:  inputDir,
			OutputDir: outputDir,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error getting new assets for %s: %v", appName, err)
			return
		}
		length = len(files)
	case "processor":
		fileSource = &producer.LocalFileSource{
			InputDir:  inputDir,
			OutputDir: outputDir,
			ProcessedDir: processedDir,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error reading %s input directory: %v\n", appName, err)
			return
		}
		length = len(files)
	default:
		log.Printf("Invalid side: %s\n", side)
		return
	}

	if length > 0 {
		log.Printf("Processing %s files...\n", appName)
		q, err := queue.NewRabbitMQConnection()
		if err != nil {
			log.Printf("Failed to connect to RabbitMQ: %v\n", err)
			return
		}
		defer q.Close()

		eventQueue := make(chan api.Event, 10)
		producer := producer.NewProducer(batchSize, fileSource, eventQueue, utils)
		producer.CreateEvent(appName, side, q)
	} else {
		log.Printf("No files found in %s directories\n", appName)
	}
}
