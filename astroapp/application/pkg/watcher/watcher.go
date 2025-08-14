package watcher

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	RunProcessor(side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient)
}

type Watcher struct{}

func NewWatcher() *Watcher {
	return &Watcher{}
}

func (w *Watcher) RunForBatch(appName string, batchName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {

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
			BatchName: batchName,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error getting new assets for %s: %v", appName, err)
			return
		}
		length = len(files)
		if length > 0 {
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
			binaryProducer.CreateBinaryEvent(appName, side, queue, binaryEventQueue)
		} else {
			log.Printf("Using standard text processing for %s", appName)
			eventQueue := make(chan api.Event, 10)
			standardProducer := producer.NewProducer(batchSize, fileSource, eventQueue, utils, side, eventID, redisClient)
			standardProducer.CreateEvent(appName, side, queue)
		}
	}
}

func (w *Watcher) RunForSingleFile(appName string, batchName string, fileName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {

	inputDirEnv := "EXPLORED_" + appName
	outputDirEnv := "OUTPUT_" + appName
	processedDirEnv := "PROCESSED_" + appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)
	processedDir := os.Getenv(processedDirEnv)

	var fileSource producer.FileSource
	var eventID string = fmt.Sprintf("%s-%s", batchName, fileName)

	switch side {
	case "producer":
		watcher := s3bucket.NewS3Watcher()
		fileSource = &producer.SingleFileSource{
			Bucket:       watcher.Bucket,
			AppName:      appName,
			InputDir:     inputDir,     // Check input directory first
			ProcessedDir: processedDir, // Also check processed directory
			OutputDir:    outputDir,
			BatchName:    batchName,
			FileName:     fileName,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error getting single file for %s: %v", appName, err)
			return
		}

		if len(files) == 0 {
			log.Printf("No file found: %s in batch %s", fileName, batchName)
			return
		}

		log.Printf("Processing single file: %s in batch %s", fileName, batchName)

	case "processor":
		fileSource = &producer.LocalFileSource{
			InputDir:     inputDir,
			ProcessedDir: processedDir,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error reading %s input directory: %v\n", appName, err)
			return
		}
		log.Printf("Found %d files in %s for processing", len(files), inputDir)

	default:
		log.Printf("Invalid side: %s\n", side)
		return
	}

	log.Printf("Processing single %s file...\n", appName)

	// Check if this app needs binary processing (PPXF)
	w.ProcessBatch(appName, side, utils, queue, redisClient, fileSource, eventID)
}

func (w *Watcher) RunProcessor(side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	batchInfoDir := os.Getenv("BATCH_INFO_DIR")

	log.Println("Checking for completed files...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Process batch_info files for STARLIGHT (Kate's system)
		files, err := os.ReadDir(batchInfoDir)
		if err != nil {
			fmt.Printf("Error reading directory: %v\n", err)
		} else {
			log.Printf("DEBUG: Found %d files in %s", len(files), batchInfoDir)
			for _, file := range files {
				if file.IsDir() {
					continue
				}

				filePath := filepath.Join(batchInfoDir, file.Name())
				if err := w.processBatchFile(filePath, side, utils, queue, redisClient); err != nil {
					log.Printf("Error processing batch file %s: %v\n", filePath, err)
					continue
				}
				log.Printf("DEBUG: Successfully processed batch file %s, removing file", filePath)
				os.Remove(filePath)
			}
		}

		// Check for pPXF output files (existing system)
		//w.RunForBatch("PPXF", "NGC7025", side, utils, queue, redisClient)
	}
}
func (w *Watcher) processBatchFile(filePath, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	batchID := strings.SplitN(filepath.Base(filePath), ".", 2)[0]

	scanner := bufio.NewScanner(file)

	// Read input directory (first line)
	if !scanner.Scan() {
		return fmt.Errorf("file is empty - missing input directory")
	}
	inputDir := strings.TrimSpace(scanner.Text())
	appName := strings.SplitN(inputDir, "/", 2)[0]
	// Read event ID (second line)
	if !scanner.Scan() {
		return fmt.Errorf("missing event ID")
	}
	eventID := strings.TrimSpace(scanner.Text())

	// Read file list (third line)
	if !scanner.Scan() {
		return fmt.Errorf("missing file list")
	}
	fileList := strings.Split(strings.TrimSpace(scanner.Text()), ",")

	processedDir := strings.Replace(inputDir, "/output", "/processed", 1)
	processedDir = filepath.Join(processedDir, eventID)
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fmt.Errorf("failed to create processed directory: %w", err)
	}
	batch := make([]api.DataFile, 0, len(fileList))
	for len(fileList) > 0 {
		remaining := []string{}

		for _, fileName := range fileList {
			fileName = strings.TrimSpace(fileName)
			sourcePath := filepath.Join(inputDir, fileName)
			log.Printf("Reading file %s from %s", fileName, sourcePath)
			if _, err := os.Stat(sourcePath); err == nil {
				content, err := os.ReadFile(sourcePath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", fileName, err)
				}
				batch = append(batch, api.DataFile{Name: fileName, Content: string(content)})
			} else {
				remaining = append(remaining, fileName)
			}
		}
		fileList = remaining
	}

	if len(fileList) == 0 {
		event := api.Event{
			ID:      eventID,
			BatchID: batchID,
			Files:   batch,
		}
		// Initialize sender
		sender := sender.NewRabbitMQSender(queue, utils, redisClient)
		sender.SendEvent(event, appName, side, queue)
		// Only move files after successful sending
		for _, file := range batch {
			sourcePath := filepath.Join(inputDir, file.Name)
			destPath := filepath.Join(processedDir, file.Name)
			if err := os.Rename(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to move file %s: %w", file.Name, err)
			}
		}
	}
	return nil
}

func (w *Watcher) ProcessBatch(appName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient, fileSource producer.FileSource, eventID string) {
	log.Printf("Processing %s files...\n", appName)
	batchSize, err := strconv.Atoi(os.Getenv("BATCH_SIZE"))
	if err != nil {
		log.Printf("Invalid batch size for %s: %v\n", appName, err)
		return
	}

	// Check if this app needs binary processing (PPXF)
	if api.IsAppBinary(appName) {
		log.Printf("Using binary processing for %s (prevents .fits corruption)", appName)
		binaryEventQueue := make(chan api.BinaryEvent, 10)
		binaryProducer := producer.NewBinaryProducer(batchSize, fileSource, binaryEventQueue, utils, side, eventID)
		binaryProducer.CreateBinaryEvent(appName, side, queue, binaryEventQueue)
	} else {
		log.Printf("Using standard text processing for %s", appName)
		eventQueue := make(chan api.Event, 10)
		standardProducer := producer.NewProducer(batchSize, fileSource, eventQueue, utils, side, eventID, redisClient)
		standardProducer.CreateEvent(appName, side, queue)
	}
}
