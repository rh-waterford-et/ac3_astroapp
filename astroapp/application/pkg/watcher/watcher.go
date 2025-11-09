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

// readProcessList reads files from a processlist file
func (w *Watcher) readProcessList(path string) []string {
	if path == "" {
		return []string{}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}
		}
		log.Printf("Error reading processlist %s: %v", path, err)
		return []string{}
	}

	var files []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			files = append(files, line)
		}
	}
	return files
}

// findCompletedFiles returns files that were in previous but not in current (completed)
func findCompletedFiles(previous, current []string) []string {
	currentMap := make(map[string]bool)
	for _, f := range current {
		currentMap[f] = true
	}

	var completed []string
	for _, f := range previous {
		if !currentMap[f] {
			completed = append(completed, f)
		}
	}
	return completed
}

// extractDatasetFromFilename extracts dataset name from pPXF filename
// e.g., NGC7025_LR-V_final_cube_voronoi_cell_0.fits -> NGC7025
func extractDatasetFromFilename(filename string) string {
	parts := strings.Split(filename, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

// handleCompletedPPXFFile collects and sends back output files for a completed pPXF input
func (w *Watcher) handleCompletedPPXFFile(inputFilename, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) error {
	outputDir := os.Getenv("OUTPUT_DIR_PPXF")
	if outputDir == "" {
		outputDir = "/processing_data/ppxf/data/output"
	}

	log.Printf("│ Collecting pPXF outputs for: %s", inputFilename)

	// Extract base name (remove .fits extension)
	baseName := strings.TrimSuffix(inputFilename, ".fits")

	// Expected pPXF output file suffixes (5 files per input)
	expectedSuffixes := []string{
		"_kinematics_and_stellar_pops_info.txt",
		"_pPXF_fitting.pdf",
		"_residuals.fits",
		"_bestfit.fits",
		"_galaxy.fits",
	}

	// Collect output files
	var binaryFiles []api.BinaryDataFile
	for _, suffix := range expectedSuffixes {
		outputFilename := baseName + suffix
		outputPath := filepath.Join(outputDir, outputFilename)

		// Check if file exists
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			log.Printf("│ ⚠ Expected pPXF output not found: %s", outputFilename)
			continue
		}

		// Read file content (binary-safe for .fits and .pdf)
		content, err := os.ReadFile(outputPath)
		if err != nil {
			log.Printf("│ ✗ Error reading pPXF output %s: %v", outputFilename, err)
			continue
		}

		binaryFiles = append(binaryFiles, api.BinaryDataFile{
			Name:    outputFilename,
			Content: content,
		})

		log.Printf("│ ✓ Collected: %s (%d bytes)", outputFilename, len(content))
	}

	if len(binaryFiles) == 0 {
		return fmt.Errorf("no pPXF output files found for: %s", inputFilename)
	}

	log.Printf("│ Collected %d/%d expected pPXF output files", len(binaryFiles), len(expectedSuffixes))

	// Extract dataset name for batch organization
	dataset := extractDatasetFromFilename(inputFilename)

	// Create binary batch
	batchID := dataset
	jobID := fmt.Sprintf("ppxf-%s-%d", baseName, time.Now().Unix())

	binaryBatch := api.BinaryBatch{
		ID:    batchID,
		JobID: jobID,
		Files: binaryFiles,
	}

	// Send batch back to producer via queue (use binary sender for .fits integrity)
	binarySender := &sender.BinaryRabbitMQSender{
		Queue: queue,
		Utils: utils,
	}
	binarySender.SendBinaryBatch(binaryBatch, "PPXF", side, queue)

	log.Printf("│ ✓ Sent pPXF batch with %d files to producer (batch: %s)", len(binaryFiles), batchID)

	// Move processed files to processed directory
	processedDir := strings.Replace(outputDir, "/output", "/processed", 1)
	err := os.MkdirAll(processedDir, 0755)
	if err != nil {
		log.Printf("│ ⚠ Failed to create processed directory: %v", err)
		return nil // Don't fail the send operation
	}

	for _, file := range binaryFiles {
		oldPath := filepath.Join(outputDir, file.Name)
		newPath := filepath.Join(processedDir, file.Name)

		if err := os.Rename(oldPath, newPath); err != nil {
			log.Printf("│ ⚠ Failed to move %s to processed: %v", file.Name, err)
		} else {
			log.Printf("│ ✓ Moved %s to processed directory", file.Name)
		}
	}

	return nil
}

// handleCompletedVoronoiFile collects and uploads output files for a completed VORONOI input
// Uploads directly to S3 (no RabbitMQ message) to avoid size limits
func (w *Watcher) handleCompletedVoronoiFile(inputFilename, side string, utils common.UtilsInterface) error {
	// Create S3 bucket instance for direct uploads
	bucket := s3bucket.NewS3Bucket()
	outputDir := os.Getenv("OUTPUT_DIR_VORONOI")
	if outputDir == "" {
		outputDir = "/processing_data/voronoi/data/output"
	}

	log.Printf("│ Collecting Voronoi outputs for: %s", inputFilename)

	// Extract base name (remove .fits extension)
	baseName := strings.TrimSuffix(inputFilename, ".fits")

	// Discover all output files that start with the base name
	// VORONOI generates multiple output files with various suffixes
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("error reading output directory: %v", err)
	}

	var outputFiles []os.DirEntry
	for _, file := range files {
		// Check if file name starts with base name
		if strings.HasPrefix(file.Name(), baseName) {
			outputFiles = append(outputFiles, file)
		}
	}

	if len(outputFiles) == 0 {
		return fmt.Errorf("no Voronoi output files found for: %s", inputFilename)
	}

	log.Printf("│ Found %d Voronoi output files", len(outputFiles))

	// Extract dataset name for S3 organization
	dataset := extractDatasetFromFilename(inputFilename)

	// Upload each file directly to S3
	uploadedCount := 0
	for _, file := range outputFiles {
		// Skip directories (e.g., individual_spectra directory)
		if file.IsDir() {
			log.Printf("│ ⚠ Skipping directory: %s", file.Name())
			continue
		}

		filePath := filepath.Join(outputDir, file.Name())

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("│ ✗ Error reading Voronoi output %s: %v", file.Name(), err)
			continue
		}

		// Upload directly to S3: voronoi/output/{dataset}/{filename}
		s3Key := fmt.Sprintf("voronoi/output/%s/%s", dataset, file.Name())
		folderPath := filepath.Dir(s3Key)
		fileName := filepath.Base(s3Key)

		err = bucket.UploadFileToBucket(folderPath, fileName, content)
		if err != nil {
			log.Printf("│ ✗ Error uploading Voronoi output %s to S3: %v", file.Name(), err)
			continue
		}

		log.Printf("│ ✓ Uploaded: %s (%d bytes)", file.Name(), len(content))
		uploadedCount++
	}

	log.Printf("│ ✓ Uploaded %d Voronoi output files directly to S3 (no message sent)", uploadedCount)

	// Move processed files to processed directory
	processedDir := strings.Replace(outputDir, "/output", "/processed", 1)
	err = os.MkdirAll(processedDir, 0755)
	if err != nil {
		log.Printf("│ ⚠ Failed to create processed directory: %v", err)
		return nil // Don't fail the upload operation
	}

	for _, file := range outputFiles {
		if file.IsDir() {
			continue // Skip directories
		}
		oldPath := filepath.Join(outputDir, file.Name())
		newPath := filepath.Join(processedDir, file.Name())

		if err := os.Rename(oldPath, newPath); err != nil {
			log.Printf("│ ⚠ Failed to move %s to processed: %v", file.Name(), err)
		} else {
			log.Printf("│ ✓ Moved %s to processed directory", file.Name())
		}
	}

	return nil
}

func (w *Watcher) RunProducer(appName string, jobName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	// Add panic recovery to prevent pod crashes
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in RunProducer for app %s, job %s: %v", appName, jobName, r)
			log.Printf("Processing will continue for other jobs/datasets")
		}
	}()

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

		// Check if this app needs S3 reference processing (VORONOI)
		if appName == "VORONOI" {
			log.Printf("Using S3 reference processing for %s (avoids RabbitMQ size limits)", appName)
			s3ReferenceBatchQueue := make(chan api.S3ReferenceBatch, 100)
			s3ReferenceProducer := producer.NewS3ReferenceProducer(jobSize, fileSource, s3ReferenceBatchQueue, utils, side, batchID)
			s3ReferenceProducer.CreateS3ReferenceBatch(appName, side, queue, s3ReferenceBatchQueue)
		} else if api.IsAppBinary(appName) {
			// Check if this app needs binary processing (PPXF)
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
}

func (w *Watcher) RunForSingleFile(appName string, jobName string, fileName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient) {
	// Add panic recovery to prevent pod crashes
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in RunForSingleFile for app %s, job %s, file %s: %v", appName, jobName, fileName, r)
			log.Printf("Processing will continue for other files/jobs")
		}
	}()

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
	// Add panic recovery to prevent pod crashes
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in RunProcessor main loop: %v", r)
			log.Printf("RunProcessor has stopped - this is a critical error")
		}
	}()

	jobInfoDir := common.GetBatchInfoDir()

	// Track previous pPXF processlist state to detect completions
	var previousPPXFList []string
	processListPath := os.Getenv("PROCESS_LIST_PPXF")

	// Make pod-specific if POD_NAME is set
	if podName := os.Getenv("POD_NAME"); podName != "" && processListPath != "" {
		dir := filepath.Dir(processListPath)
		processListPath = filepath.Join(dir, fmt.Sprintf("processlist-%s.txt", podName))
		log.Printf("Monitoring pod-specific pPXF processlist: %s", processListPath)
	}

	// Track previous VORONOI processlist state to detect completions
	var previousVoronoiList []string
	voronoiProcessListPath := os.Getenv("PROCESS_LIST_VORONOI")

	// Make pod-specific if POD_NAME is set
	if podName := os.Getenv("POD_NAME"); podName != "" && voronoiProcessListPath != "" {
		dir := filepath.Dir(voronoiProcessListPath)
		voronoiProcessListPath = filepath.Join(dir, fmt.Sprintf("processlist-%s.txt", podName))
		log.Printf("Monitoring pod-specific VORONOI processlist: %s", voronoiProcessListPath)
	}

	log.Println("Checking for completed files...")
	log.Println("Starting pPXF processlist monitoring...")
	log.Println("Starting VORONOI processlist monitoring...")

	for {
		// Process Starlight batch_info files (existing system)
		files, err := os.ReadDir(jobInfoDir)
		if err != nil {
			fmt.Printf("Error reading directory: %v\n", err)
		} else {
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

		// NEW: Monitor pPXF processlist for completed files
		if processListPath != "" {
			currentPPXFList := w.readProcessList(processListPath)

			// Only check for completions after first iteration (when we have previous state)
			if previousPPXFList != nil {
				completedFiles := findCompletedFiles(previousPPXFList, currentPPXFList)

				for _, filename := range completedFiles {
					log.Printf("✓ pPXF completed processing: %s", filename)

					err := w.handleCompletedPPXFFile(filename, side, utils, queue, redisClient)
					if err != nil {
						log.Printf("✗ Error handling pPXF completion for %s: %v", filename, err)
					} else {
						log.Printf("✓ Successfully sent pPXF outputs for: %s", filename)
					}
				}
			}

			// Update previous state
			previousPPXFList = currentPPXFList
		}

		// NEW: Monitor VORONOI processlist for completed files
		if voronoiProcessListPath != "" {
			currentVoronoiList := w.readProcessList(voronoiProcessListPath)

			// Only check for completions after first iteration (when we have previous state)
			if previousVoronoiList != nil {
				completedFiles := findCompletedFiles(previousVoronoiList, currentVoronoiList)

				for _, filename := range completedFiles {
					log.Printf("✓ VORONOI completed processing: %s", filename)

					err := w.handleCompletedVoronoiFile(filename, side, utils)
					if err != nil {
						log.Printf("✗ Error handling VORONOI completion for %s: %v", filename, err)
					} else {
						log.Printf("✓ Successfully uploaded VORONOI outputs for: %s", filename)
					}
				}
			}

			// Update previous state
			previousVoronoiList = currentVoronoiList
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

func (w *Watcher) ProcessJob(appName string, side string, utils common.UtilsInterface, queue queue.QueueInterface, redisClient *metrics.RedisClient, fileSource producer.FileSource, batchID string) {
	log.Printf("Processing %s files...\n", appName)
	jobSize, err := strconv.Atoi(os.Getenv("JOB_SIZE"))
	if err != nil {
		log.Printf("Invalid job size for %s: %v\n", appName, err)
		return
	}

	// Check if this app needs S3 reference processing (VORONOI)
	if appName == "VORONOI" {
		log.Printf("Using S3 reference processing for %s (avoids RabbitMQ size limits)", appName)
		s3ReferenceBatchQueue := make(chan api.S3ReferenceBatch, 100)
		s3ReferenceProducer := producer.NewS3ReferenceProducer(jobSize, fileSource, s3ReferenceBatchQueue, utils, side, batchID)
		s3ReferenceProducer.CreateS3ReferenceBatch(appName, side, queue, s3ReferenceBatchQueue)
	} else if api.IsAppBinary(appName) {
		// Check if this app needs binary processing (PPXF)
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
