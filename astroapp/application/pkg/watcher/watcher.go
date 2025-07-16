package watcher

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
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

func (w *Watcher) Run(appName string, side string, utils common.UtilsInterface, queue queue.QueueInterface) {
	// Initialize Redis client
	var redisClient *metrics.RedisClient
	var err error
	if os.Getenv("REDIS_HOST") != "" {
		redisClient, err = metrics.NewRedisConnection()
		if err != nil {
			log.Printf("Failed to connect to Redis: %v", err)
			log.Printf("Continuing without Redis metrics tracking")
		} else {
			log.Printf("Connected to Redis for metrics tracking")
		}
	}

	inputDirEnv := "EXPLORED_" + appName
	outputDirEnv := "OUTPUT_" + appName
	processedDirEnv := "PROCESSED_" + appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)
	processedDir := os.Getenv(processedDirEnv)
	batchSize, err := strconv.Atoi(os.Getenv("BATCH_SIZE"))
	if err != nil {
		log.Printf("Invalid batch size for %s: %v\n", appName, err)
		return
	}

	var fileSource producer.FileSource
	var length = 0
	batchCounts := make(map[string]int)
	var eventID string

	switch side {
	case "producer":
		watcher := s3bucket.NewS3Watcher()
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
		if length > 0 {
			log.Printf("Found %d files in %s input directory across all batches: %v", length, appName, files)
			for _, file := range files {
				if strings.Contains(file, ".batch_placeholder") ||
					strings.Contains(file, ".dataset_placeholder") {
					continue
				}
				parts := strings.Split(file, "/")
				if len(parts) >= 1 {
					batchName := parts[0]
					eventID = batchName
					batchCounts[batchName]++
				}
			}
			for batch, count := range batchCounts {
				log.Printf("  Batch %s: %d files", batch, count)
			}
		}
	case "processor":
		fileSource = &producer.LocalFileSource{
			InputDir:     inputDir,
			OutputDir:    outputDir,
			ProcessedDir: processedDir,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error reading %s input directory: %v\n", appName, err)
			return
		}
		length = len(files)
		log.Printf("Found %d files in %s: %v", length, inputDir, files)
	default:
		log.Printf("Invalid side: %s\n", side)
		return
	}

	if len(batchCounts) > 0 {
		log.Printf("Processing %s files...\n", appName)
		eventQueue := make(chan api.Event, 10)
		producer := producer.NewProducer(batchSize, fileSource, eventQueue, utils, side, eventID, redisClient)
		producer.CreateEvent(appName, side, queue)
	}
}
