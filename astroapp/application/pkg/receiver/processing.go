package receiver

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
)

func (r *Receiver) processFiles(msgBody api.MessageBody, side, appName, outputPath string) (int, string, string, []api.DataFile) {
	successCount := 0
	var inFileName string
	var inFileContent string
	var spectrumFiles []api.DataFile

	// starlight := app.NewStarlight([]api.DataFile{}, r.Utils)
	ppxf := app.NewPPXF(r.Utils)

	for _, file := range msgBody.Files {
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden file: %s", file.Name)
			successCount++
			continue
		}

		if strings.HasSuffix(file.Name, ".in") {
			inFileName = file.Name
			inFileContent = string(file.Content)
			log.Printf("│ ✓ Found .in file: %s", file.Name)
			successCount++
			continue
		}

		if !strings.HasSuffix(file.Name, ".in") && !strings.HasPrefix(filepath.Base(file.Name), ".") {
			spectrumFiles = append(spectrumFiles, file)
		}

		if side == "producer" {
			successCount += r.handleProducerFile(file, outputPath)
		} else {
			successCount += r.handleProcessorFile(file, outputPath, appName, ppxf)
		}
	}

	return successCount, inFileName, inFileContent, spectrumFiles
}

func (r *Receiver) handleProducerFile(file api.DataFile, outputPath string) int {
	batchName := r.extractBatchNameFromFilename(file.Name)
	var uploadPath string

	if batchName != "" {
		uploadPath = filepath.Join(outputPath, batchName, file.Name)
	} else {
		uploadPath = filepath.Join(outputPath, file.Name)
	}

	folderPath := filepath.Dir(uploadPath)
	fileName := filepath.Base(uploadPath)

	if folderPath == "." {
		folderPath = ""
	}

	//log.Printf("│ DEBUG: S3 upload - folderPath: '%s', fileName: '%s'", folderPath, fileName)

	err := r.Bucket.UploadFileToBucket(folderPath, fileName, []byte(file.Content))
	if err != nil {
		log.Printf("│ ✗ Error uploading file %s to bucket: %v", uploadPath, err)
		return 0
	}

	log.Printf("│ ✓ Uploaded file to bucket: %s", uploadPath)
	return 1
}

func (r *Receiver) handleProcessorFile(file api.DataFile, outputPath, appName string, ppxf *app.PPXF) int {
	filename := filepath.Base(file.Name)
	filePath := filepath.Join(outputPath, filename)

	err := os.WriteFile(filePath, []byte(file.Content), 0644)
	if err != nil {
		log.Printf("│ ✗ Error writing file %s: %v", filePath, err)
		return 0
	}

	log.Printf("│ ✓ Wrote file: %s to %s", file.Name, filePath)

	if appName == "PPXF" && strings.HasSuffix(file.Name, ".fits") {
		ppxf.AddToProcessList(filename)
		log.Printf("│ ✓ Added pPXF file to process list: %s", filename)
	}

	return 1
}

func (r *Receiver) processStandardMessage(d amqp.Delivery, side, appName, batchID, jobID string) {
	log.Printf("│ Processing standard text batch for %s", appName)
	r.updateProgress(appName, jobID, api.StageProcessing, 20.0)

	// Validate batch metadata
	jobSize, _, ok := r.validateJobMetadata(d, jobID)
	if !ok {
		return
	}

	// Job size is now recorded on producer side before sending to queue

	// Process message body
	msgBody, ok := r.processMessageBody(d, jobID, jobSize)
	if !ok {
		log.Printf("processMessageBody not OK, exiting?")
		return //THIS IS BAD, Throw Error!!
	}

	// Determine output path based on side and app
	outputPath := r.getOutputPath(side, appName)
	if outputPath == "" {
		log.Printf("outputPath could not be determined?")
		return //THIS IS BAD, Throw Error!!
	}

	// Process files
	log.Printf("processFiles")
	successCount, inFileName, inFileContent, spectrumFiles := r.processFiles(msgBody, side, appName, outputPath)

	// Handle application-specific processing
	r.handleApplicationProcessing(side, appName, jobID, successCount, jobSize, inFileName, inFileContent, spectrumFiles)

	// Finalize batch processing
	r.finalizeJobProcessing(d, side, appName, batchID, jobID, successCount, jobSize)
}

// ProcessBinaryMessage handles binary events for PPXF (prevents .fits corruption)
func (r *Receiver) ProcessBinaryMessage(d amqp.Delivery, side string, appName string, jobID string) {
	processStart := time.Now()

	// Update progress to processing stage
	r.updateProgress(appName, jobID, api.StageProcessing, 20.0)
	jobSize, filenames, ok := r.validateJobMetadata(d, jobID)
	if !ok {
		return
	}

	// Job size is now recorded on producer side before sending to queue

	log.Printf("│ Processing binary batch of %d files:", jobSize)
	for i, filename := range filenames {
		log.Printf("│ %d. %s", i+1, filename)
	}

	// Get output path for binary files
	outputPath := r.getOutputPath(side, appName)
	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for binary app")
		r.requeueWithLog(d, jobID)
		return
	}

	// Unmarshal as BinaryMessageBody for binary events
	var binaryMsgBody api.BinaryMessageBody
	err := json.Unmarshal(d.Body, &binaryMsgBody)
	if err != nil {
		log.Printf("│ ERROR parsing binary message body: %v", err)
		r.requeueWithLog(d, jobID)
		return
	}

	if len(binaryMsgBody.Files) != int(jobSize) {
		log.Printf("│ ERROR: Binary files count in body doesn't match batch_size")
		r.requeueWithLog(d, jobID)
		return
	}

	successCount := 0
	for _, file := range binaryMsgBody.Files {
		// Skip hidden files (files starting with .)
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden binary file: %s", file.Name)
			successCount++
			continue
		}

		if side == "producer" {
			// Handle S3 upload for binary files
			batchName := r.extractBatchNameFromFilename(file.Name)
			var uploadPath string

			// Special handling for PPXF: organize by cell number
			if appName == "PPXF" {
				log.Printf("│ DEBUG: Processing PPXF file for cell organization: %s", file.Name)
				cellNumber := r.extractCellNumberFromFilename(file.Name)
				log.Printf("│ DEBUG: Cell number extracted: '%s' (batchName: '%s')", cellNumber, batchName)
				if batchName != "" && cellNumber != "" {
					// Path: /ppxf/output/<batch_name>/<cell_number>/filename
					uploadPath = filepath.Join(outputPath, batchName, cellNumber, file.Name)
					log.Printf("│ DEBUG: Using cell-organized path: %s", uploadPath)
				} else if batchName != "" {
					// Fallback: /ppxf/output/<batch_name>/filename
					uploadPath = filepath.Join(outputPath, batchName, file.Name)
					log.Printf("│ DEBUG: Using batch fallback path (missing cell): %s", uploadPath)
				} else {
					// Last resort: flat structure
					uploadPath = filepath.Join(outputPath, file.Name)
					log.Printf("│ DEBUG: Using flat fallback path: %s", uploadPath)
				}
			} else {
				// Standard logic for other apps (Starlight, etc.)
				if batchName != "" {
					uploadPath = filepath.Join(outputPath, batchName, file.Name)
				} else {
					uploadPath = filepath.Join(outputPath, file.Name)
				}
			}

			folderPath := filepath.Dir(uploadPath)
			fileName := filepath.Base(uploadPath)

			if folderPath == "." {
				folderPath = ""
			}

			log.Printf("│ DEBUG: S3 binary upload - folderPath: '%s', fileName: '%s'", folderPath, fileName)

			// Create directory structure if folderPath exists
			if folderPath != "" {
				err := r.Bucket.CreateDirectory(folderPath)
				if err != nil {
					log.Printf("│ ⚠ Error creating directory %s: %v", folderPath, err)
					// Continue with upload even if directory creation fails
				}
			}

			// Upload binary content directly (no string conversion corruption)
			err := r.Bucket.UploadFileToBucket(folderPath, fileName, file.Content)
			if err != nil {
				log.Printf("│ ✗ Error uploading binary file %s to bucket: %v", uploadPath, err)
			} else {
				log.Printf("│ ✓ Uploaded binary file to bucket: %s (%d bytes)", uploadPath, len(file.Content))
				successCount++
			}
		} else {
			// Handle local file write for binary files - NO STRING CONVERSION
			filename := filepath.Base(file.Name)
			filePath := filepath.Join(outputPath, filename)

			// Write binary content directly (preserves .fits integrity)
			err := os.WriteFile(filePath, file.Content, 0644)
			if err != nil {
				log.Printf("│ ✗ Error writing binary file %s: %v", filePath, err)
			} else {
				log.Printf("│ ✓ Wrote binary file: %s to %s (%d bytes)", file.Name, filePath, len(file.Content))
				successCount++

				// For pPXF, add each .fits file to the process list immediately
				if appName == "PPXF" && strings.HasSuffix(file.Name, ".fits") {
					ppxf := app.NewPPXF(r.Utils)
					err := ppxf.AddToProcessList(filename)
					if err != nil {
						// Processlist not empty - requeue message for later
						log.Printf("│ ⚠ Cannot add to processlist (list not empty): %s", filename)
						log.Printf("│ ⚠ Message will be requeued for later processing")
						processDuration := time.Since(processStart)
						log.Printf("\n■■■ BINARY BATCH REQUEUED [%s] ■■■ Duration: %v\n", jobID, processDuration)
						// NACK with requeue - message goes back to queue
						d.Nack(false, true) // requeue=true
						// Keep flag TRUE - prevents immediate re-pull of same message
						// Flag will be reset when processlist becomes empty
						return
					}
					log.Printf("│ ✓ Added pPXF binary file to process list: %s", filename)
					// Keep ProcessingMessage = true
					// Will be reset by receiver when processlist becomes empty
				}
			}
		}
	}

	if successCount == int(jobSize) {
		log.Printf("│ ✓ Successfully processed all %d binary files", successCount)
		// Update progress to analysis stage for pPXF
		r.updateProgress(appName, jobID, api.StageAnalysis, 70.0)
		log.Printf("│ ✓ Files added to pPXF processlist")
	} else {
		log.Printf("│ ⚠ Processed %d of %d binary files", successCount, jobSize)
	}

	processDuration := time.Since(processStart)
	log.Printf("\n■■■ BINARY BATCH END [%s] ■■■ Duration: %v\n", jobID, processDuration)
	d.Ack(false)
}

// downloadFileWithRetry downloads a file from S3 with exponential backoff retry
func (r *Receiver) downloadFileWithRetry(s3Key, bucketName string, maxRetries int) ([]byte, error) {
	var lastErr error
	baseDelay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			log.Printf("│ ⚠ Retry attempt %d/%d for S3 download: %s (waiting %v)", attempt, maxRetries, s3Key, delay)
			time.Sleep(delay)
		}

		// Use the bucket's DownloadFile method
		// The receiver's bucket should already be configured with the correct bucket name
		content, err := r.Bucket.DownloadFile(s3Key)
		if err == nil {
			if attempt > 0 {
				log.Printf("│ ✓ S3 download succeeded on attempt %d", attempt+1)
			}
			return content, nil
		}

		lastErr = err
		log.Printf("│ ⚠ S3 download attempt %d/%d failed: %v", attempt+1, maxRetries+1, err)
	}

	return nil, fmt.Errorf("failed to download %s after %d retries: %v", s3Key, maxRetries+1, lastErr)
}

// ProcessS3ReferenceMessage handles S3 reference events for VORONOI (downloads from S3 with retry)
func (r *Receiver) ProcessS3ReferenceMessage(d amqp.Delivery, side string, appName string, jobID string) {
	processStart := time.Now()

	// Update progress to processing stage
	r.updateProgress(appName, jobID, api.StageProcessing, 20.0)
	jobSize, filenames, ok := r.validateJobMetadata(d, jobID)
	if !ok {
		return
	}

	log.Printf("│ Processing S3 reference batch of %d files:", jobSize)
	for i, filename := range filenames {
		log.Printf("│ %d. %s", i+1, filename)
	}

	// Get output path for files
	outputPath := r.getOutputPath(side, appName)
	if outputPath == "" {
		log.Printf("│ ERROR: Output directory not configured for app")
		r.requeueWithLog(d, jobID)
		return
	}

	// Unmarshal as S3ReferenceMessageBody
	var refMsgBody api.S3ReferenceMessageBody
	err := json.Unmarshal(d.Body, &refMsgBody)
	if err != nil {
		log.Printf("│ ERROR parsing S3 reference message body: %v", err)
		r.requeueWithLog(d, jobID)
		return
	}

	if len(refMsgBody.Files) != int(jobSize) {
		log.Printf("│ ERROR: S3 reference files count in body doesn't match batch_size")
		r.requeueWithLog(d, jobID)
		return
	}

	successCount := 0
	for _, file := range refMsgBody.Files {
		// Skip hidden files
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden file: %s", file.Name)
			successCount++
			continue
		}

		if side == "processor" {
			// Processor side: Download from S3, write locally, add to processlist
			filename := filepath.Base(file.Name)

			// Extract dataset name from S3 key (format: voronoi/input/{dataset}/filename)
			// For VORONOI, we need dataset name to save config per-dataset
			var datasetName string
			if appName == "VORONOI" {
				keyParts := strings.Split(file.S3Key, "/")
				// S3 key format: voronoi/input/{dataset}/filename
				if len(keyParts) >= 3 && keyParts[0] == "voronoi" && keyParts[1] == "input" {
					datasetName = keyParts[2]
				}
			}

			// For VORONOI config files, save with dataset-specific name
			var filePath string
			// Check if this is a VORONOI config file (either voronoi_config.json or voronoi_config_{dataset}.json)
			isVoronoiConfig := appName == "VORONOI" &&
				(strings.HasPrefix(filename, "voronoi_config") && strings.HasSuffix(filename, ".json"))

			if isVoronoiConfig && datasetName != "" {
				// Always save as voronoi_config_{dataset}.json to avoid conflicts between datasets
				configFilename := fmt.Sprintf("voronoi_config_%s.json", datasetName)
				filePath = filepath.Join(outputPath, configFilename)
				log.Printf("│ Saving VORONOI config with dataset-specific name: %s (from %s)", configFilename, filename)
			} else {
				filePath = filepath.Join(outputPath, filename)
			}

			// RETRY LOGIC: Download file from S3 with retries
			content, err := r.downloadFileWithRetry(file.S3Key, file.S3Bucket, 3)
			if err != nil {
				log.Printf("│ ✗ Error downloading file %s from S3 after retries: %v", file.S3Key, err)
				// Continue to next file, will NACK at end if successCount < jobSize
				continue
			}

			// Write downloaded content to local filesystem
			err = os.WriteFile(filePath, content, 0644)
			if err != nil {
				log.Printf("│ ✗ Error writing file %s: %v", filePath, err)
				continue
			}

			log.Printf("│ ✓ Downloaded and wrote file: %s to %s (%d bytes)", file.Name, filePath, len(content))
			successCount++

			// CRITICAL: Only add to processlist AFTER successful download and write
			// For VORONOI, download config file directly (skip queue) and add datacube to processlist
			if appName == "VORONOI" && strings.HasSuffix(file.Name, ".fits") && datasetName != "" {
				// Download config file directly from S3 (skip queue to avoid ordering issues)
				// Config file uses dataset-specific naming: voronoi_config_{dataset}.json
				configS3Key := fmt.Sprintf("voronoi/input/%s/voronoi_config_%s.json", datasetName, datasetName)
				configFilename := fmt.Sprintf("voronoi_config_%s.json", datasetName)
				configFilePath := filepath.Join(outputPath, configFilename)

				// Check if config file already exists locally
				if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
					log.Printf("│ Downloading VORONOI config file directly from S3: %s (bucket: %s)", configS3Key, file.S3Bucket)
					configContent, err := r.downloadFileWithRetry(configS3Key, file.S3Bucket, 3)
					if err != nil {
						log.Printf("│ ⚠ Failed to download config file %s from bucket %s: %v", configS3Key, file.S3Bucket, err)
						log.Printf("│ ⚠ Message will be requeued for later processing")
						processDuration := time.Since(processStart)
						log.Printf("\n■■■ S3 REFERENCE BATCH REQUEUED [%s] ■■■ Duration: %v\n", jobID, processDuration)
						d.Nack(false, true) // requeue=true
						return
					}

					if len(configContent) == 0 {
						log.Printf("│ ⚠ Config file downloaded but is empty: %s", configS3Key)
						log.Printf("│ ⚠ Message will be requeued for later processing")
						processDuration := time.Since(processStart)
						log.Printf("\n■■■ S3 REFERENCE BATCH REQUEUED [%s] ■■■ Duration: %v\n", jobID, processDuration)
						d.Nack(false, true) // requeue=true
						return
					}

					// Write config file to local filesystem
					err = os.WriteFile(configFilePath, configContent, 0644)
					if err != nil {
						log.Printf("│ ⚠ Failed to write config file %s: %v", configFilePath, err)
						log.Printf("│ ⚠ Message will be requeued for later processing")
						processDuration := time.Since(processStart)
						log.Printf("\n■■■ S3 REFERENCE BATCH REQUEUED [%s] ■■■ Duration: %v\n", jobID, processDuration)
						d.Nack(false, true) // requeue=true
						return
					}

					// Verify the file was written correctly
					if stat, err := os.Stat(configFilePath); err != nil || stat.Size() == 0 {
						log.Printf("│ ⚠ Config file write verification failed: %v (size: %d)", err, stat.Size())
						log.Printf("│ ⚠ Message will be requeued for later processing")
						processDuration := time.Since(processStart)
						log.Printf("\n■■■ S3 REFERENCE BATCH REQUEUED [%s] ■■■ Duration: %v\n", jobID, processDuration)
						d.Nack(false, true) // requeue=true
						return
					}

					log.Printf("│ ✓ Downloaded and wrote config file: %s (%d bytes) to %s", configFilename, len(configContent), configFilePath)
				} else {
					log.Printf("│ ✓ Config file already exists: %s", configFilePath)
				}

				// Now add to processlist (config file is guaranteed to exist)
				voronoi := app.NewVoronoi(r.Utils)
				err := voronoi.AddToProcessListWithDataset(filename, datasetName)
				if err != nil {
					// Processlist not empty - requeue message for later
					log.Printf("│ ⚠ Cannot add to processlist: %s (reason: %v)", filename, err)
					log.Printf("│ ⚠ Message will be requeued for later processing")
					processDuration := time.Since(processStart)
					log.Printf("\n■■■ S3 REFERENCE BATCH REQUEUED [%s] ■■■ Duration: %v\n", jobID, processDuration)
					d.Nack(false, true) // requeue=true
					return
				}
				log.Printf("│ ✓ Added Voronoi datacube to process list: %s (dataset: %s)", filename, datasetName)
			}
		} else {
			// Producer side: Files should already be in S3 (from watcher upload)
			// Just acknowledge - no action needed
			log.Printf("│ ✓ S3 reference file already in S3: %s", file.S3Key)
			successCount++
		}
	}

	// Finalize processing
	if successCount == int(jobSize) {
		log.Printf("│ ✓ Successfully processed all %d S3 reference files", successCount)
		r.updateProgress(appName, jobID, api.StageAnalysis, 70.0)
	} else {
		log.Printf("│ ⚠ Processed %d of %d S3 reference files", successCount, jobSize)
	}

	processDuration := time.Since(processStart)
	log.Printf("\n■■■ S3 REFERENCE BATCH END [%s] ■■■ Duration: %v\n", jobID, processDuration)

	if successCount == int(jobSize) {
		d.Ack(false)
	} else {
		// NACK and requeue if not all files succeeded
		d.Nack(false, true)
	}
}
