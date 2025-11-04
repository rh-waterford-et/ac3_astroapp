package watcher

import (
	"fmt"
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
	RunProcessor(side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient)
	RunProducer(appName string, jobName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient)
	RunForSingleFile(appName string, jobName string, fileName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient)
}

type Watcher struct{}

func NewWatcher() *Watcher {
	return &Watcher{}
}

func (w *Watcher) RunProducer(appName string, batchName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	defer w.recoverPanic(fmt.Sprintf("RunProducer for app %s, batch %s", appName, batchName))

	inputDir, outputDir, processedDir := getAppDirs(appName)
	fileSource, batchID, jobCounts, err := w.initFileSource(appName, batchName, side, inputDir, outputDir, processedDir)
	if err != nil {
		log.Println(err)
		return
	}

	if side == "producer" && len(jobCounts) > 0 {
		log.Printf("Processing %s files...\n", appName)
		w.ProcessJob(appName, side, utils, queue, redisClient, fileSource, batchID)
	}

}

func (w *Watcher) RunForSingleFile(appName string, batchName string, fileName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	defer w.recoverPanic(fmt.Sprintf("RunForSingleFile for app %s, batch %s, file %s", appName, batchName, fileName))

	inputDir, outputDir, processedDir := getAppDirs(appName)
	batchID := fmt.Sprintf("%s-%s", batchName, fileName)

	fileSource, err := w.initSingleFileSource(appName, batchName, fileName, side, inputDir, outputDir, processedDir)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Processing single %s file...\n", appName)
	w.ProcessJob(appName, side, utils, queue, redisClient, fileSource, batchID)
}


func (w *Watcher) recoverPanic(context string) {
	if r := recover(); r != nil {
		log.Printf("PANIC RECOVERED in %s: %v", context, r)
		log.Println("Processing will continue for other batches/datasets")
	}
}

func getAppDirs(appName string) (string, string, string) {
	return os.Getenv("EXPLORED_"+appName), os.Getenv("OUTPUT_"+appName), os.Getenv("PROCESSED_"+appName)
}

func (w *Watcher) initFileSource(appName, batchName, side, inputDir, outputDir, processedDir string) (producer.FileSource, string, map[string]int, error) {
	if side != "producer" {
		return nil, "", nil, fmt.Errorf("invalid side: %s", side)
	}

	watcher := s3bucket.NewS3Watcher()
	fileSource := &producer.S3FileSource{
		Bucket:    watcher.Bucket,
		AppName:   appName,
		InputDir:  inputDir,
		OutputDir: outputDir,
		ProcessedDir: processedDir,
		BatchName:   batchName,
	}

	files, err := fileSource.ListFiles()
	if err != nil {
		return nil, "", nil, fmt.Errorf("error getting new assets for %s: %v", appName, err)
	}

	jobCounts := make(map[string]int)
	var batchID string

	for _, file := range files {
		parts := strings.Split(file, "/")
		if len(parts) > 0 {
			job := parts[0]
			batchID = job
			jobCounts[job]++
		}
	}

	for job, count := range jobCounts {
		log.Printf("  Batch %s: %d files", job, count)
	}

	return fileSource, batchID, jobCounts, nil
}

func (w *Watcher) initSingleFileSource(appName, batchName, fileName, side, inputDir, outputDir, processedDir string) (producer.FileSource, error) {
	if side != "producer" {
		return nil, fmt.Errorf("invalid side: %s", side)
	}

	watcher := s3bucket.NewS3Watcher()
	fileSource := &producer.SingleFileSource{
		Bucket:       watcher.Bucket,
		AppName:      appName,
		InputDir:     inputDir,
		ProcessedDir: processedDir,
		OutputDir:    outputDir,
		BatchName:    batchName,
		FileName:     fileName,
	}

	files, err := fileSource.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("error getting single file for %s: %v", appName, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no file found: %s in batch %s", fileName, batchName)
	}

	log.Printf("Processing single file: %s in batch %s", fileName, batchName)
	return fileSource, nil
}

func (w *Watcher) ProcessJob(appName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient, fileSource producer.FileSource, batchID string) {
	log.Printf("Processing %s files...\n", appName)
	jobSize, err := strconv.Atoi(os.Getenv("JOB_SIZE"))
	if err != nil {
		log.Printf("Invalid job size for %s: %v\n", appName, err)
		return
	}

	if api.IsAppBinary(appName) {
		log.Printf("Using binary processing for %s (prbatchs .fits corruption)", appName)
		binaryBatchQueue := make(chan api.BinaryBatch, 100)
		binaryProducer := producer.NewBinaryProducer(jobSize, fileSource, binaryBatchQueue, utils, side, batchID)
		binaryProducer.CreateBinaryBatch(appName, side, queue, binaryBatchQueue)
	} else {
		log.Printf("Using standard text processing for %s", appName)
		batchQueue := make(chan api.Batch, 100)
		standardProducer := producer.NewProducer(jobSize, fileSource, batchQueue, utils, side, batchID, redisClient)
		standardProducer.CreateBatch(appName, side, queue)
	}
}
