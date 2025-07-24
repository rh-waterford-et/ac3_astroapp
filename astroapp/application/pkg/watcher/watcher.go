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
        "github.com/rh-waterford-et/ac3_astroapp/pkg/sender"
)

type WatcherInterface interface {
        Run(appName string, side string, utils common.UtilsInterface) error
}

type Watcher struct{}

func NewWatcher() *Watcher {
        return &Watcher{}
}

func (w *Watcher) Run(appName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {

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
                        //      log.Printf("Found %d files in %s input directory across all batches: %v", length, appName, files)
                        for _, file := range files {
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

        // Process files if we have batches (producer) or files (processor)
        shouldProcess := false
        if side == "producer" && len(batchCounts) > 0 {
                shouldProcess = true
        } else if side == "processor" && length > 0 {
                shouldProcess = true
        }

        if shouldProcess {
                log.Printf("Processing %s files...\n", appName)

                // Check if this app needs binary processing (PPXF)
                if api.IsAppBinary(appName) {
                        log.Printf("Using binary processing for %s (prevents .fits corruption)", appName)
                        binaryEventQueue := make(chan api.BinaryEvent, 10)
                        binaryProducer := producer.NewBinaryProducer(batchSize, fileSource, binaryEventQueue, utils, side, eventID)
                        w.CreateBinaryEvent(appName, side, queue, binaryProducer, binaryEventQueue)
                } else {
                        log.Printf("Using standard text processing for %s", appName)
                        eventQueue := make(chan api.Event, 10)
                        standardProducer := producer.NewProducer(batchSize, fileSource, eventQueue, utils, side, eventID, redisClient)
                        standardProducer.CreateEvent(appName, side, queue)
                }
        } /* else {
                log.Printf("No files found in %s directories\n", appName)
        }*/
}

// CreateBinaryEvent handles binary event processing for PPXF
func (w *Watcher) CreateBinaryEvent(appName string, side string, queue queue.QueueInterface, binaryProducer *producer.BinaryProducer, eventQueue chan api.BinaryEvent) {
        go func() {
                for event := range eventQueue {
                        log.Printf("Sending binary event (ID: %s) with %d files\n", event.ID, len(event.Files))
                        w.SendBinaryEvent(event, appName, side, queue)
                }
        }()

        binaryProducer.ProcessBinaryFiles(appName)
}

// SendBinaryEvent sends binary events via RabbitMQ (handles .fits files safely)
func (w *Watcher) SendBinaryEvent(event api.BinaryEvent, appName string, side string, queue queue.QueueInterface) {
        binarySender := &sender.BinaryRabbitMQSender{}
        binarySender.SendBinaryEvent(event, appName, side, queue)
}
