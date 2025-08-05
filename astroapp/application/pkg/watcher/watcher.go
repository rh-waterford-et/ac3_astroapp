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
)

type WatcherInterface interface {
	Run(appName string, side string, utils common.UtilsInterface) error
	RunProcessor()
}

type Watcher struct{}

func NewWatcher() *Watcher {
	return &Watcher{}
}

func (w *Watcher) RunForBatch(appName string, batchName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	inputDirEnv := "EXPLORED_" + appName
	outputDirEnv := "OUTPUT_" + appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)
	
	
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
	}

	// Process files if we have batches (producer) or files (processor)
	if len(batchCounts) > 0 {
		w.ProcessBatch(appName, side, utils, queue, redisClient, fileSource, eventID)
	}
}

func (w *Watcher) RunProcessor(side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	batchInfoDir := os.Getenv("BATCH_INFO_DIR")

	log.Println("Checking for completed files...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			files, err := os.ReadDir(batchInfoDir)
			if err != nil {
				fmt.Printf("Error reading directory: %v\n", err)
				continue
			}

			for _, file := range files {
				if file.IsDir() {
					continue
				}

				filePath := filepath.Join(batchInfoDir, file.Name())
				err := w.processBatchFile(filePath, side, utils, queue, redisClient)
				if err != nil {
					fmt.Printf("Error processing batch file %s: %v\n", filePath, err)
					continue
				}
				os.Remove(filePath)
			}
		}
	}
}

func (w *Watcher) processBatchFile(filePath string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	if !scanner.Scan() {
		return fmt.Errorf("file is empty")
	}

	inputDir := strings.TrimSpace(scanner.Text())
	parts := strings.Split(inputDir, "/")
    var appName string
    if len(parts) > 0 {
        appName = parts[0]
    } else {
        return fmt.Errorf("invalid inputDir format: %s", inputDir)
    }
	if !scanner.Scan() {
		return fmt.Errorf("file list is missing")
	}
	fileList := strings.Split(strings.TrimSpace(scanner.Text()), ",")

	fileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	processDir := filepath.Join(inputDir, fileName)
	if err := os.MkdirAll(processDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	for len(fileList) > 0 {
		for i := 0; i < len(fileList); i++ {
			fileName := strings.TrimSpace(fileList[i])

			sourcePath := filepath.Join(inputDir, fileName)
			destPath := filepath.Join(processDir, fileName)
	
			if _, err := os.Stat(sourcePath); err == nil {
				if err := os.Rename(sourcePath, destPath); err != nil {
					return fmt.Errorf("failed to move file %s: %w", sourcePath, err)
				}
				fileList = append(fileList[:i], fileList[i+1:]...)
				i--
			}
		}
	}

	if len(fileList) == 0 {
		processedDir := strings.Replace(inputDir, "/output", "/processed", 1)

		fileSource := &producer.LocalFileSource{
			InputDir:     processDir,
			ProcessedDir: processedDir,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			return fmt.Errorf("error reading input directory: %w", err)
		}
		length := len(files)
		var eventID string
		if length > 0 {
			for _, file := range files {
				parts := strings.Split(file, "/")
				if len(parts) >= 2 {
					eventID = parts[1]
				}
			}
		}
		log.Printf("Found %d files in %s: %v", length, inputDir, files)

		
		if length > 0 {
			w.ProcessBatch(appName, side, utils, queue, redisClient, fileSource, eventID)
		}
		os.RemoveAll(processDir)
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
		binaryEventQueue := make(chan api.BinaryEvent, 10)
		binaryProducer := producer.NewBinaryProducer(batchSize, fileSource, binaryEventQueue, utils, side, eventID)
		binaryProducer.CreateBinaryEvent(appName, side, queue, binaryEventQueue)
	} else {
		eventQueue := make(chan api.Event, 10)
		standardProducer := producer.NewProducer(batchSize, fileSource, eventQueue, utils, side, eventID, redisClient)
		standardProducer.CreateEvent(appName, side, queue)
	}
}