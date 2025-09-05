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

func (w *Watcher) RunProducer(appName string, jobName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {

	inputDirEnv := "EXPLORED_" + appName
	outputDirEnv := "OUTPUT_" + appName
	processedDirEnv := "PROCESSED_" + appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)
	processedDir := os.Getenv(processedDirEnv)
	jobSize, err := strconv.Atoi(os.Getenv("JOB_SIZE"))
	if err != nil {
		log.Printf("Invalid job size for %s: %v\n", appName, err)
		return
	}

	var fileSource producer.FileSource
	var length = 0
	jobCounts := make(map[string]int)
	var batchID string

	switch side {
	case "producer":
		watcher := s3bucket.NewS3Watcher()
		fileSource = &producer.S3FileSource{
			Bucket:    watcher.Bucket,
			AppName:   appName,
			InputDir:  inputDir,
			OutputDir: outputDir,
			JobName:   jobName,
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
					jobName := parts[0]
					batchID = jobName
					jobCounts[jobName]++
				}
			}
			for job, count := range jobCounts {
				log.Printf("  Batch %s: %d files", job, count)
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

	// Process files if we have jobes (producer) or files (processor)
	shouldProcess := false
	if side == "producer" && len(jobCounts) > 0 {
		shouldProcess = true
	} else if side == "processor" && length > 0 {
		shouldProcess = true
	}

	if shouldProcess {
		log.Printf("Processing %s files...\n", appName)

		// Check if this app needs binary processing (PPXF)
		if api.IsAppBinary(appName) {
			log.Printf("Using binary processing for %s (prbatchs .fits corruption)", appName)
			binaryBatchQueue := make(chan api.BinaryBatch, 10)
			binaryProducer := producer.NewBinaryProducer(jobSize, fileSource, binaryBatchQueue, utils, side, batchID)
			binaryProducer.CreateBinaryBatch(appName, side, queue, binaryBatchQueue)
		} else {
			log.Printf("Using standard text processing for %s", appName)
			batchQueue := make(chan api.Batch, 10)
			standardProducer := producer.NewProducer(jobSize, fileSource, batchQueue, utils, side, batchID, redisClient)
			standardProducer.CreateBatch(appName, side, queue)
		}
	}
}

func (w *Watcher) RunForSingleFile(appName string, jobName string, fileName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {

	inputDirEnv := "EXPLORED_" + appName
	outputDirEnv := "OUTPUT_" + appName
	processedDirEnv := "PROCESSED_" + appName

	inputDir := os.Getenv(inputDirEnv)
	outputDir := os.Getenv(outputDirEnv)
	processedDir := os.Getenv(processedDirEnv)

	var fileSource producer.FileSource
	var batchID string = fmt.Sprintf("%s-%s", jobName, fileName)

	switch side {
	case "producer":
		watcher := s3bucket.NewS3Watcher()
		fileSource = &producer.SingleFileSource{
			Bucket:       watcher.Bucket,
			AppName:      appName,
			InputDir:     inputDir,     // Check input directory first
			ProcessedDir: processedDir, // Also check processed directory
			OutputDir:    outputDir,
			JobName:      jobName,
			FileName:     fileName,
		}
		files, err := fileSource.ListFiles()
		if err != nil {
			log.Printf("Error getting single file for %s: %v", appName, err)
			return
		}

		if len(files) == 0 {
			log.Printf("No file found: %s in job %s", fileName, jobName)
			return
		}

		log.Printf("Processing single file: %s in job %s", fileName, jobName)

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
	w.ProcessJob(appName, side, utils, queue, redisClient, fileSource, batchID)
}

func (w *Watcher) RunProcessor(side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	jobInfoDir := os.Getenv("BATCH_INFO_DIR")

	log.Println("Checking for completed files...")

	for {

		files, err := os.ReadDir(jobInfoDir)
		if err != nil {
			fmt.Printf("Error reading directory: %v\n", err)
		} else {
			//log.Printf("DEBUG: Found %d files in %s", len(files), jobInfoDir)
			for _, file := range files {
				//log.Printf("DEBUG: Processing file: %s", file.Name())
				filePath := filepath.Join(jobInfoDir, file.Name())
				//log.Printf("DEBUG: File path: %s", filePath)
				if err := w.processJobFile(filePath, side, utils, queue, redisClient); err != nil {
					log.Printf("Error processing job file %s: %v\n", filePath, err)
					continue
				}
				log.Printf("DEBUG: Successfully processed job file %s, removing file", filePath)
				os.Remove(filePath)
			}
		}

		// Check for pPXF output files (existing system)
		//w.RunForJob("PPXF", "NGC7025", side, utils, queue, redisClient)

		// Sleep for 10 seconds before next iteration
		time.Sleep(10 * time.Second)
	}
}

func (w *Watcher) processJobFile(filePath, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) error {
	log.Printf("DEBUG: Processing job file: %s", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	jobID := strings.SplitN(filepath.Base(filePath), ".", 2)[0]

	scanner := bufio.NewScanner(file)

	// Read input directory (first line)
	if !scanner.Scan() {
		return fmt.Errorf("file is empty - missing input directory")
	}
	inputDir := strings.TrimSpace(scanner.Text())
	appName := strings.ToUpper(strings.SplitN(inputDir, "/", 2)[0])
	// Read batch ID (second line)
	if !scanner.Scan() {
		return fmt.Errorf("missing batch ID")
	}
	batchID := strings.TrimSpace(scanner.Text())

	// Read file list (third line)
	if !scanner.Scan() {
		return fmt.Errorf("missing file list")
	}
	fileList := strings.Split(strings.TrimSpace(scanner.Text()), ",")

	processedDir := strings.Replace(inputDir, "/output", "/processed", 1)
	processedDir = filepath.Join(processedDir, batchID)
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fmt.Errorf("failed to create processed directory: %w", err)
	}
	job := make([]api.DataFile, 0, len(fileList))
	for len(fileList) > 0 {
		remaining := []string{}

		for _, fileName := range fileList {
			fileName = strings.TrimSpace(fileName)
			sourcePath := filepath.Join(inputDir, fileName)
			if _, err := os.Stat(sourcePath); err == nil {
				content, err := os.ReadFile(sourcePath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", fileName, err)
				}
				job = append(job, api.DataFile{Name: fileName, Content: string(content)})
			} else {
				remaining = append(remaining, fileName)
			}
		}
		fileList = remaining
	}

	if len(fileList) == 0 {
		batch := api.Batch{
			ID:    batchID,
			JobID: jobID,
			Files: job,
		}
		// Initialize sender
		sender := sender.NewRabbitMQSender(queue, utils, redisClient)
		sender.SendBatch(batch, appName, side, queue)
		// Only move files after successful sending
		for _, file := range job {
			sourcePath := filepath.Join(inputDir, file.Name)
			destPath := filepath.Join(processedDir, file.Name)
			if err := os.Rename(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to move file %s: %w", file.Name, err)
			}
		}

	}
	//log.Printf("DEBUG: Successfully processed job file: %s", filePath)
	return nil
}

func (w *Watcher) ProcessJob(appName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient, fileSource producer.FileSource, batchID string) {
	log.Printf("Processing %s files...\n", appName)
	jobSize, err := strconv.Atoi(os.Getenv("JOB_SIZE"))
	if err != nil {
		log.Printf("Invalid job size for %s: %v\n", appName, err)
		return
	}

	// Check if this app needs binary processing (PPXF)
	if api.IsAppBinary(appName) {
		log.Printf("Using binary processing for %s (prbatchs .fits corruption)", appName)
		binaryBatchQueue := make(chan api.BinaryBatch, 10)
		binaryProducer := producer.NewBinaryProducer(jobSize, fileSource, binaryBatchQueue, utils, side, batchID)
		binaryProducer.CreateBinaryBatch(appName, side, queue, binaryBatchQueue)
	} else {
		log.Printf("Using standard text processing for %s", appName)
		batchQueue := make(chan api.Batch, 10)
		standardProducer := producer.NewProducer(jobSize, fileSource, batchQueue, utils, side, batchID, redisClient)
		standardProducer.CreateBatch(appName, side, queue)
	}
}
