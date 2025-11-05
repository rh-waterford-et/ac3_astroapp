package watcher

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/sender"
)

func (w *Watcher) RunProcessor(side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	// Add panic recovery to prevent pod crashes
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in RunProcessor main loop: %v", r)
			log.Printf("RunProcessor has stopped - this is a critical error")
		}
	}()

	jobInfoDir := os.Getenv("BATCH_INFO_DIR")

	for {
		//log.Printf("DEBUG: Reading directory: %s", jobInfoDir)
		files, err := os.ReadDir(jobInfoDir)
		if err != nil {
			fmt.Printf("Error reading directory: %v\n", err)
		} else {
			//log.Printf("DEBUG: Found %d files in directory: %s", len(files), jobInfoDir)
			for _, file := range files {
				// Wrap each file processing in panic recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("PANIC RECOVERED while processing job file %s: %v", file.Name(), r)
							log.Printf("Skipping this file and continuing with others")
						}
					}()

					filePath := filepath.Join(jobInfoDir, file.Name())
					if err := w.processJobFile(filePath, side, utils, queue, redisClient); err != nil {
						log.Printf("Error processing job file %s: %v\n", filePath, err)
						return
					}
					log.Printf("DEBUG: Successfully processed job file %s, removing file", filePath)
					os.Remove(filePath)
				}()
			}
		}

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
				time.Sleep(2 * time.Second)
				content, err := os.ReadFile(sourcePath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", fileName, err)
				}
				job = append(job, api.DataFile{Name: fileName, Content: string(content)})

				destPath := filepath.Join(processedDir, fileName)
				if err := os.Rename(sourcePath, destPath); err != nil {
					return fmt.Errorf("failed to move file %s: %w", fileName, err)
				}

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

	}
	//log.Printf("DEBUG: Successfully processed job file: %s", filePath)
	return nil
}
