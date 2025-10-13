package receiver

import (
	"encoding/json"
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
	jobSize, filenames, ok := r.validateJobMetadata(d, jobID)
	if !ok {
		return
	}

	jobSizeMB := r.calculateJobSizeMB(d.Body, filenames)

	// Record job size in metrics

	if side == "processor" && jobSizeMB > 0 && r.RedisClient != nil {
		r.recordJobSize(batchID, jobID, jobSizeMB)
	}
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

	jobSizeMB := r.calculateBinaryJobSizeMB(d.Body, filenames)
	if batchID, ok := d.Headers["batch_id"].(string); ok && jobSizeMB > 0 && r.RedisClient != nil && side == "processor" {
		r.recordJobSize(batchID, jobID, jobSizeMB)
	}

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
					ppxf.AddToProcessList(filename)
					log.Printf("│ ✓ Added pPXF binary file to process list: %s", filename)
				}
			}
		}
	}

	if successCount == int(jobSize) {
		log.Printf("│ ✓ Successfully processed all %d binary files", successCount)
		// Update progress to analysis stage for pPXF
		r.updateProgress(appName, jobID, api.StageAnalysis, 70.0)
	} else {
		log.Printf("│ ⚠ Processed %d of %d binary files", successCount, jobSize)
	}

	processDuration := time.Since(processStart)
	log.Printf("\n■■■ BINARY BATCH END [%s] ■■■ Duration: %v\n", jobID, processDuration)
	d.Ack(false)
}
