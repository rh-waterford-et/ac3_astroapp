package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type FileUploadHandler struct {
	S3Bucket        s3bucket.S3BucketInterface // Consumer bucket (default/current)
	ProviderBucket  s3bucket.S3BucketInterface // Provider bucket for connector mode
	ProgressTracker *ProgressTracker
}

type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	FileKey string `json:"fileKey,omitempty"`
}

type ListDatasetsResponse struct {
	Success  bool     `json:"success"`
	Datasets []string `json:"datasets"`
}

type CreateDatasetRequest struct {
	DatasetName   string         `json:"datasetName"`
	AppType       string         `json:"appType"`
	PPXFConfig    *PPXFConfig    `json:"ppxfConfig,omitempty"`
	VoronoiConfig *VoronoiConfig `json:"voronoiConfig,omitempty"`
	ConnectorMode bool           `json:"connectorMode,omitempty"`
}

type PPXFConfig struct {
	Redshift       float64 `json:"redshift"`
	VelocityDisp   float64 `json:"velocityDisp"`
	WaveRangeStart int     `json:"waveRangeStart"`
	WaveRangeEnd   int     `json:"waveRangeEnd"`
	SPSName        string  `json:"spsName"`
}

type VoronoiConfig struct {
	Instrument                string  `json:"instrument"`
	TargetSN                  float64 `json:"targetSN"`
	Redshift                  float64 `json:"redshift"`
	WavelengthStart           int     `json:"wavelengthStart"`
	WavelengthEnd             int     `json:"wavelengthEnd"`
	SNMethod                  string  `json:"snMethod"`
	KnotsNumber               int     `json:"knotsNumber"`
	MinSN                     float64 `json:"minSN"`
	GenerateIndividualSpectra bool    `json:"generateIndividualSpectra"`
}

// Unified file listing response structure
type UnifiedFilesResponse struct {
	Success    bool          `json:"success"`
	Files      []DatasetFile `json:"files"`
	Message    string        `json:"message,omitempty"`
	Pagination struct {
		Offset  int  `json:"offset"`
		Limit   int  `json:"limit"`
		Total   int  `json:"total"`
		HasMore bool `json:"hasMore"`
	} `json:"pagination"`
}

type CreateDatasetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type DatasetFile struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Timestamp string `json:"timestamp"`
	Key       string `json:"key"`
}

type ListDatasetFilesResponse struct {
	Success bool          `json:"success"`
	Files   []DatasetFile `json:"files"`
	Message string        `json:"message,omitempty"`
}

type PaginatedDatasetFilesResponse struct {
	Success bool          `json:"success"`
	Files   []DatasetFile `json:"files"`
	Message string        `json:"message,omitempty"`
	// Pagination metadata
	Total   int  `json:"total"`
	Offset  int  `json:"offset"`
	Limit   int  `json:"limit"`
	HasMore bool `json:"hasMore"`
}

func NewFileUploadHandler(s3Bucket s3bucket.S3BucketInterface, progressTracker *ProgressTracker) *FileUploadHandler {
	// Create provider bucket instance for connector mode
	providerBucket := s3bucket.NewS3BucketWithName("test-provider")

	return &FileUploadHandler{
		S3Bucket:        s3Bucket,       // Consumer bucket (test-consumer)
		ProviderBucket:  providerBucket, // Provider bucket (test-provider)
		ProgressTracker: progressTracker,
	}
}

// isSystemFile checks if a file is a system/configuration file that shouldn't be shown in listings
func isSystemFile(filename string) bool {
	systemFiles := []string{
		"mask.txt",           // pPXF mask file
		"ppxf_config.json",   // pPXF user configuration file
		".batch_placeholder", // Old build remnant
		".gitkeep",           // Git keep files
		".DS_Store",          // macOS system files
		"Thumbs.db",          // Windows thumbnail cache
	}

	for _, sysFile := range systemFiles {
		if filename == sysFile {
			return true
		}
	}

	// Skip hidden files (starting with .)
	if strings.HasPrefix(filename, ".") {
		return true
	}

	return false
}

func (h *FileUploadHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		response := UploadResponse{
			Success: false,
			Message: "Failed to parse upload form",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get batch name from form (supporting both 'batch' and 'dataset' for backward compatibility)
	batchName := r.FormValue("batch")
	if batchName == "" {
		batchName = r.FormValue("dataset") // backward compatibility
	}

	if batchName == "" {
		log.Printf("Batch name parameter missing")
		w.WriteHeader(http.StatusBadRequest)
		response := UploadResponse{
			Success: false,
			Message: "Batch name parameter is required (use 'batch' or 'dataset')",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate batch name (basic validation for S3 key compatibility)
	if strings.Contains(batchName, "/") || strings.Contains(batchName, "\\") {
		w.WriteHeader(http.StatusBadRequest)
		response := UploadResponse{
			Success: false,
			Message: "Batch name cannot contain slashes",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from form (default to starlight)
	appType := r.FormValue("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Get connector mode from form (default to false)
	connectorMode := r.FormValue("connectorMode") == "true"

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		w.WriteHeader(http.StatusBadRequest)
		response := UploadResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get the file from form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from form: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		response := UploadResponse{
			Success: false,
			Message: "No file found in request",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	defer file.Close()

	// Validate file type
	fileName := fileHeader.Filename
	allowedExtensions := []string{".fits", ".txt", ".csv", ".log", ".in"}
	ext := strings.ToLower(filepath.Ext(fileName))

	isAllowed := false
	for _, allowedExt := range allowedExtensions {
		if ext == allowedExt {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		w.WriteHeader(http.StatusBadRequest)
		response := UploadResponse{
			Success: false,
			Message: fmt.Sprintf("File type %s not allowed. Supported types: %s", ext, strings.Join(allowedExtensions, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading file content: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		response := UploadResponse{
			Success: false,
			Message: "Failed to read file content",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Choose bucket based on connector mode
	var targetBucket s3bucket.S3BucketInterface
	var bucketType string
	if connectorMode {
		// Connector mode: upload to provider bucket
		targetBucket = h.ProviderBucket
		bucketType = "provider"
	} else {
		// Direct mode: upload to consumer bucket (current behavior)
		targetBucket = h.S3Bucket
		bucketType = "consumer"
	}

	log.Printf("Uploading file %s to %s bucket (connector mode: %v)", fileName, bucketType, connectorMode)

	// Upload to S3 using the new batch structure: app/input/batch_name/filename
	folderPath := fmt.Sprintf("%s/input/%s", appType, batchName)
	err = targetBucket.UploadFileToBucket(folderPath, fileName, content)
	if err != nil {
		log.Printf("Error uploading file to S3: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		response := UploadResponse{
			Success: false,
			Message: "Failed to upload file to S3",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Success response
	response := UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		FileKey: fmt.Sprintf("%s/%s", folderPath, fileName),
	}

	log.Printf("Successfully uploaded file: %s to batch %s in app %s", fileName, batchName, appType)
	json.NewEncoder(w).Encode(response)
}

func (h *FileUploadHandler) ListDatasets(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get application type from query parameter (default to starlight)
	appType := r.URL.Query().Get("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := ListDatasetsResponse{
			Success:  false,
			Datasets: []string{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check both input and output directories to find all datasets
	// A dataset exists if it has either an input or output directory
	inputPrefix := fmt.Sprintf("%s/input", appType)
	outputPrefix := fmt.Sprintf("%s/output", appType)

	// Get datasets from input directory
	inputDatasets, err := h.S3Bucket.GetBatchDirectories(inputPrefix)
	if err != nil {
		log.Printf("Error listing input directories for %s: %v", appType, err)
		inputDatasets = []string{} // Continue with empty list if input fails
	}

	// Get datasets from output directory
	outputDatasets, err := h.S3Bucket.GetBatchDirectories(outputPrefix)
	if err != nil {
		log.Printf("Error listing output directories for %s: %v", appType, err)
		outputDatasets = []string{} // Continue with empty list if output fails
	}

	// Merge both lists and remove duplicates using a map
	datasetMap := make(map[string]bool)
	for _, dataset := range inputDatasets {
		datasetMap[dataset] = true
	}
	for _, dataset := range outputDatasets {
		datasetMap[dataset] = true
	}

	// Convert map to sorted slice
	var batchDirectories []string
	for dataset := range datasetMap {
		batchDirectories = append(batchDirectories, dataset)
	}
	sort.Strings(batchDirectories)

	log.Printf("Found %d batch directories for %s (from %d input + %d output): %v",
		len(batchDirectories), appType, len(inputDatasets), len(outputDatasets), batchDirectories)

	response := ListDatasetsResponse{
		Success:  true,
		Datasets: batchDirectories,
	}
	json.NewEncoder(w).Encode(response)
}

func (h *FileUploadHandler) CreateDataset(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request
	var req CreateDatasetRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Error parsing create batch request: %v", err)
		response := CreateDatasetResponse{
			Success: false,
			Message: "Invalid request format",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate batch name
	if req.DatasetName == "" {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Batch name is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from request (default to starlight)
	appType := req.AppType
	if appType == "" {
		appType = "starlight" // default
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := CreateDatasetResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Sanitize batch name
	sanitizedName := strings.TrimSpace(req.DatasetName)
	if strings.Contains(sanitizedName, "/") || strings.Contains(sanitizedName, "\\") {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Batch name cannot contain slashes",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Define directories to create
	directories := []string{
		fmt.Sprintf("%s/input/%s/", appType, sanitizedName),
		fmt.Sprintf("%s/output/%s/", appType, sanitizedName),
		fmt.Sprintf("%s/processed/%s/", appType, sanitizedName),
	}

	// Determine which buckets to create directories in
	bucketsToCreate := []struct {
		bucket s3bucket.S3BucketInterface
		name   string
	}{
		{h.S3Bucket, "consumer"}, // Always create in consumer bucket
	}

	// In connector mode, also create directories in provider bucket
	if req.ConnectorMode {
		bucketsToCreate = append(bucketsToCreate, struct {
			bucket s3bucket.S3BucketInterface
			name   string
		}{h.ProviderBucket, "provider"})
		log.Printf("Connector mode enabled: creating directories in both provider and consumer buckets")
	} else {
		log.Printf("Direct mode: creating directories in consumer bucket only")
	}

	// Create directories in the appropriate buckets
	for _, bucketInfo := range bucketsToCreate {
		for _, dir := range directories {
			// Check if directory already exists
			_, err := bucketInfo.bucket.GetS3Client().HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketInfo.bucket.GetBucketName()),
				Key:    aws.String(dir),
			})

			if err == nil {
				log.Printf("Directory already exists in %s bucket: %s", bucketInfo.name, dir)
				continue
			}

			// If error is not "Not Found", return error
			if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NotFound" {
				log.Printf("Error checking directory existence in %s bucket: %v", bucketInfo.name, err)
				response := CreateDatasetResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to check directory %s in %s bucket", dir, bucketInfo.name),
				}
				json.NewEncoder(w).Encode(response)
				return
			}

			// Create directory by putting an empty object with trailing slash
			_, err = bucketInfo.bucket.GetS3Client().PutObject(&s3.PutObjectInput{
				Bucket: aws.String(bucketInfo.bucket.GetBucketName()),
				Key:    aws.String(dir),
			})
			if err != nil {
				log.Printf("Error creating directory in %s bucket: %v", bucketInfo.name, err)
				response := CreateDatasetResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to create directory %s in %s bucket", dir, bucketInfo.name),
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			log.Printf("Successfully created directory in %s bucket: %s", bucketInfo.name, dir)
		}
	}

	// Create pPXF config file if this is a pPXF dataset and config is provided
	if strings.ToLower(appType) == "ppxf" && req.PPXFConfig != nil {
		configJSON, err := json.MarshalIndent(req.PPXFConfig, "", "  ")
		if err != nil {
			log.Printf("Error marshaling pPXF config: %v", err)
			response := CreateDatasetResponse{
				Success: false,
				Message: "Failed to create pPXF configuration file",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Choose bucket for config file based on connector mode
		// Config file goes where the input files will be uploaded
		var configBucket s3bucket.S3BucketInterface
		var configBucketType string
		if req.ConnectorMode {
			configBucket = h.ProviderBucket
			configBucketType = "provider"
		} else {
			configBucket = h.S3Bucket
			configBucketType = "consumer"
		}

		// Upload config file to the appropriate bucket
		configKey := fmt.Sprintf("%s/input/%s/ppxf_config.json", appType, sanitizedName)
		_, err = configBucket.GetS3Client().PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(configBucket.GetBucketName()),
			Key:         aws.String(configKey),
			Body:        bytes.NewReader(configJSON),
			ContentType: aws.String("application/json"),
		})
		if err != nil {
			log.Printf("Error uploading pPXF config file to %s bucket: %v", configBucketType, err)
			response := CreateDatasetResponse{
				Success: false,
				Message: "Failed to upload pPXF configuration file",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		log.Printf("Successfully created pPXF config file in %s bucket: %s", configBucketType, configKey)
	}

	// Create Voronoi config file if this is a Voronoi dataset and config is provided
	if strings.ToLower(appType) == "voronoi" && req.VoronoiConfig != nil {
		configJSON, err := json.MarshalIndent(req.VoronoiConfig, "", "  ")
		if err != nil {
			log.Printf("Error marshaling Voronoi config: %v", err)
			response := CreateDatasetResponse{
				Success: false,
				Message: "Failed to create Voronoi configuration file",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Choose bucket for config file based on connector mode
		// Config file goes where the input files will be uploaded
		var configBucket s3bucket.S3BucketInterface
		var configBucketType string
		if req.ConnectorMode {
			configBucket = h.ProviderBucket
			configBucketType = "provider"
		} else {
			configBucket = h.S3Bucket
			configBucketType = "consumer"
		}

		// Upload config file to the appropriate bucket
		// Use dataset-specific filename: voronoi_config_{dataset}.json
		configKey := fmt.Sprintf("%s/input/%s/voronoi_config_%s.json", appType, sanitizedName, sanitizedName)
		_, err = configBucket.GetS3Client().PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(configBucket.GetBucketName()),
			Key:         aws.String(configKey),
			Body:        bytes.NewReader(configJSON),
			ContentType: aws.String("application/json"),
		})
		if err != nil {
			log.Printf("Error uploading Voronoi config file to %s bucket: %v", configBucketType, err)
			response := CreateDatasetResponse{
				Success: false,
				Message: "Failed to upload Voronoi configuration file",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		log.Printf("Successfully created Voronoi config file in %s bucket: %s", configBucketType, configKey)
	}

	// Success response with connector mode info
	var bucketInfo string
	if req.ConnectorMode {
		bucketInfo = " (directories created in both provider and consumer buckets)"
	} else {
		bucketInfo = " (directories created in consumer bucket)"
	}

	response := CreateDatasetResponse{
		Success: true,
		Message: fmt.Sprintf("Dataset '%s' created successfully for application '%s'%s", sanitizedName, appType, bucketInfo),
	}
	json.NewEncoder(w).Encode(response)
	log.Printf("Successfully created dataset: %s for application: %s (connector mode: %v)", sanitizedName, appType, req.ConnectorMode)
}

func (h *FileUploadHandler) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get dataset name from URL query parameter
	datasetName := r.URL.Query().Get("dataset")
	if datasetName == "" {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Dataset name parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from query parameter (default to starlight)
	appType := r.URL.Query().Get("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := CreateDatasetResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate dataset name
	if strings.Contains(datasetName, "/") || strings.Contains(datasetName, "\\") {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Invalid dataset name",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Delete all three directories for the dataset
	directories := []string{
		fmt.Sprintf("%s/input/%s", appType, datasetName),
		fmt.Sprintf("%s/output/%s", appType, datasetName),
		fmt.Sprintf("%s/processed/%s", appType, datasetName),
	}

	deletedDirs := 0
	for _, dir := range directories {
		err := h.S3Bucket.DeleteDirectory(dir)
		if err != nil {
			log.Printf("Warning: Failed to delete directory %s: %v", dir, err)
			// Continue with other directories even if one fails
		} else {
			log.Printf("Successfully deleted directory: %s", dir)
			deletedDirs++
		}
	}

	if deletedDirs == 0 {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Failed to delete any dataset directories",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Successfully deleted dataset: %s for application: %s (%d/%d directories)",
		datasetName, appType, deletedDirs, len(directories))

	response := CreateDatasetResponse{
		Success: true,
		Message: fmt.Sprintf("Dataset deleted successfully (%d/%d directories)", deletedDirs, len(directories)),
	}
	json.NewEncoder(w).Encode(response)
}

func (h *FileUploadHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get file key from URL query parameter
	fileKey := r.URL.Query().Get("key")
	if fileKey == "" {
		response := CreateDatasetResponse{
			Success: false,
			Message: "File key parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from query parameter (default to starlight)
	appType := r.URL.Query().Get("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := CreateDatasetResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Delete the specific file from S3
	err := h.S3Bucket.DeleteFile(fileKey)
	if err != nil {
		log.Printf("Error deleting file %s: %v", fileKey, err)
		response := CreateDatasetResponse{
			Success: false,
			Message: "Failed to delete file",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Successfully deleted file: %s", fileKey)

	response := CreateDatasetResponse{
		Success: true,
		Message: "File deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func (h *FileUploadHandler) ListDatasetFiles(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get batch name from URL query parameter
	batchName := r.URL.Query().Get("dataset")
	if batchName == "" {
		batchName = r.URL.Query().Get("batch") // also support 'batch' parameter
	}

	if batchName == "" {
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Batch name parameter is required (use 'dataset' or 'batch')",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from query parameter (default to starlight)
	appType := r.URL.Query().Get("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Parse pagination parameters
	page := 0  // default
	limit := 0 // default (0 means no pagination)

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate batch name
	if strings.Contains(batchName, "/") || strings.Contains(batchName, "\\") {
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Invalid batch name",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// List objects from both input and processed folders to show complete file list
	var allFiles []DatasetFile
	fileSeen := make(map[string]bool) // Track filenames to avoid duplicates

	// Check both input and processed folders
	folders := []struct {
		path string
		name string
	}{
		{fmt.Sprintf("%s/input/%s", appType, batchName), "input"},
		{fmt.Sprintf("%s/processed/%s", appType, batchName), "processed"},
	}

	for _, folder := range folders {
		log.Printf("DEBUG: Listing objects in %s folder: %s", folder.name, folder.path)
		objects, err := h.S3Bucket.GetS3Objects(folder.path)
		if err != nil {
			log.Printf("Warning: Error listing S3 objects in %s folder %s: %v", folder.name, folder.path, err)
			continue // Continue with other folder
		}
		log.Printf("DEBUG: Found %d objects in %s folder %s", len(objects), folder.name, folder.path)

		// Convert objects to DatasetFile structs
		for _, object := range objects {
			filename := object

			// Skip system/configuration files
			if isSystemFile(filename) {
				continue
			}

			// Skip if we've already seen this file (avoid duplicates)
			if fileSeen[filename] {
				continue
			}
			fileSeen[filename] = true

			// Get file metadata from S3
			fullObjectKey := fmt.Sprintf("%s/%s", folder.path, filename)
			metadata, err := h.S3Bucket.GetObjectMetadata(fullObjectKey)
			if err != nil {
				log.Printf("Warning: Could not get metadata for %s: %v", filename, err)
				// Still include the file with basic info
				allFiles = append(allFiles, DatasetFile{
					Name:      filename,
					Size:      0,
					Timestamp: "Unknown",
					Key:       fullObjectKey,
				})
				continue
			}

			allFiles = append(allFiles, DatasetFile{
				Name:      filename,
				Size:      metadata.Size,
				Timestamp: metadata.LastModified.Format("2006-01-02 15:04:05"),
				Key:       fullObjectKey,
			})
		}
	}

	var files []DatasetFile
	totalFiles := len(allFiles)

	if limit > 0 {
		// Apply pagination
		startIndex := page * limit
		endIndex := startIndex + limit

		// Ensure we don't go out of bounds
		if startIndex > totalFiles {
			startIndex = totalFiles
		}
		if endIndex > totalFiles {
			endIndex = totalFiles
		}

		// Get the paginated slice
		if startIndex < totalFiles {
			files = allFiles[startIndex:endIndex]
		}

		log.Printf("Paginated input files for batch %s (%s): returning %d files (page: %d, limit: %d, total: %d)",
			batchName, appType, len(files), page, limit, totalFiles)

		// Return paginated response
		paginatedResponse := struct {
			Success    bool          `json:"success"`
			Files      []DatasetFile `json:"files"`
			Message    string        `json:"message,omitempty"`
			Pagination struct {
				Page    int  `json:"page"`
				Limit   int  `json:"limit"`
				Total   int  `json:"total"`
				HasMore bool `json:"hasMore"`
			} `json:"pagination"`
		}{
			Success: true,
			Files:   files,
			Pagination: struct {
				Page    int  `json:"page"`
				Limit   int  `json:"limit"`
				Total   int  `json:"total"`
				HasMore bool `json:"hasMore"`
			}{
				Page:    page,
				Limit:   limit,
				Total:   totalFiles,
				HasMore: endIndex < totalFiles,
			},
		}
		json.NewEncoder(w).Encode(paginatedResponse)
	} else {
		// No pagination requested, return all files
		files = allFiles
		log.Printf("Found %d total files (input + processed) in batch %s for app %s", len(files), batchName, appType)

		response := ListDatasetFilesResponse{
			Success: true,
			Files:   files,
		}
		json.NewEncoder(w).Encode(response)
	}
}

func (h *FileUploadHandler) ListDatasetOutputFiles(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get batch name from URL query parameter
	batchName := r.URL.Query().Get("dataset")
	if batchName == "" {
		batchName = r.URL.Query().Get("batch") // also support 'batch' parameter
	}

	if batchName == "" {
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Batch name parameter is required (use 'dataset' or 'batch')",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from query parameter (default to starlight)
	appType := r.URL.Query().Get("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Parse pagination parameters
	page := 0  // default
	limit := 0 // default (0 means no pagination)

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate batch name
	if strings.Contains(batchName, "/") || strings.Contains(batchName, "\\") {
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Invalid batch name",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// List objects in the specific batch's output folder
	folderPath := fmt.Sprintf("%s/output/%s", appType, batchName)

	var files []DatasetFile

	if limit > 0 {
		// Use true server-side pagination - get all objects first to know total count
		allObjects, err := h.S3Bucket.GetS3Objects(folderPath)
		if err != nil {
			log.Printf("Error listing S3 objects for batch %s output in app %s: %v", batchName, appType, err)
			response := ListDatasetFilesResponse{
				Success: false,
				Files:   []DatasetFile{},
				Message: "Failed to list batch output files",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Filter out directory markers and system files - only count actual data files
		var actualFiles []string
		for _, object := range allObjects {
			if !strings.HasSuffix(object, "/") && !isSystemFile(object) {
				actualFiles = append(actualFiles, object)
			}
		}

		// Calculate pagination boundaries using actual file count
		totalFiles := len(actualFiles)
		allObjects = actualFiles // Use filtered list for pagination

		// Sort files for pPXF by directory number before pagination
		if strings.ToLower(appType) == "ppxf" {
			sort.Slice(allObjects, func(i, j int) bool {
				// Extract directory numbers from paths like "108/filename.fits"
				partsA := strings.Split(allObjects[i], "/")
				partsB := strings.Split(allObjects[j], "/")

				if len(partsA) == 0 || len(partsB) == 0 {
					return allObjects[i] < allObjects[j]
				}

				// Convert directory names to integers for proper numerical sorting
				dirA, errA := strconv.Atoi(partsA[0])
				dirB, errB := strconv.Atoi(partsB[0])

				// If parsing fails, fall back to string comparison
				if errA != nil || errB != nil {
					return allObjects[i] < allObjects[j]
				}

				// Compare as integers: 2 < 10 < 20 (not "10" < "2" < "20")
				return dirA < dirB
			})
		}

		startIndex := page * limit
		endIndex := startIndex + limit

		// Ensure we don't go out of bounds
		if startIndex > totalFiles {
			startIndex = totalFiles
		}
		if endIndex > totalFiles {
			endIndex = totalFiles
		}

		// Only get metadata for the files we need for this page
		paginatedObjects := allObjects[startIndex:endIndex]

		for _, object := range paginatedObjects {
			filename := object
			fullObjectKey := fmt.Sprintf("%s/%s", folderPath, filename)
			metadata, err := h.S3Bucket.GetObjectMetadata(fullObjectKey)
			if err != nil {
				log.Printf("Warning: Could not get metadata for %s: %v", filename, err)
				files = append(files, DatasetFile{
					Name:      filename,
					Size:      0,
					Timestamp: "Unknown",
					Key:       fullObjectKey,
				})
				continue
			}

			files = append(files, DatasetFile{
				Name:      filename,
				Size:      metadata.Size,
				Timestamp: metadata.LastModified.Format("2006-01-02 15:04:05"),
				Key:       fullObjectKey,
			})
		}

		log.Printf("Found %d output files in batch %s for app %s (showing %d-%d of %d)",
			len(files), batchName, appType, startIndex+1, endIndex, totalFiles)

		// Return paginated response
		paginatedResponse := struct {
			Success    bool          `json:"success"`
			Files      []DatasetFile `json:"files"`
			Message    string        `json:"message,omitempty"`
			Pagination struct {
				Page    int  `json:"page"`
				Limit   int  `json:"limit"`
				Total   int  `json:"total"`
				HasMore bool `json:"hasMore"`
			} `json:"pagination"`
		}{
			Success: true,
			Files:   files,
			Pagination: struct {
				Page    int  `json:"page"`
				Limit   int  `json:"limit"`
				Total   int  `json:"total"`
				HasMore bool `json:"hasMore"`
			}{
				Page:    page,
				Limit:   limit,
				Total:   totalFiles,
				HasMore: endIndex < totalFiles,
			},
		}
		json.NewEncoder(w).Encode(paginatedResponse)
		return
	} else {
		// Get all objects for backward compatibility
		objects, err := h.S3Bucket.GetS3Objects(folderPath)
		if err != nil {
			log.Printf("Error listing S3 objects for batch %s output in app %s: %v", batchName, appType, err)
			response := ListDatasetFilesResponse{
				Success: false,
				Files:   []DatasetFile{},
				Message: "Failed to list batch output files",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Sort files for pPXF by directory number before processing
		if strings.ToLower(appType) == "ppxf" {
			sort.Slice(objects, func(i, j int) bool {
				// Extract directory numbers from paths like "108/filename.fits"
				dirStrA := strings.Split(objects[i], "/")[0]
				dirStrB := strings.Split(objects[j], "/")[0]

				// Convert directory names to integers for proper numerical sorting
				dirA, errA := strconv.Atoi(dirStrA)
				dirB, errB := strconv.Atoi(dirStrB)

				// If parsing fails, fall back to string comparison
				if errA != nil || errB != nil {
					return objects[i] < objects[j]
				}

				// Compare as integers: 2 < 10 < 20 (not "10" < "2" < "20")
				return dirA < dirB
			})
		}

		// Convert all objects to DatasetFile structs, but skip directory markers and system files
		for _, object := range objects {
			// Skip directory markers (objects ending with "/")
			if strings.HasSuffix(object, "/") {
				continue
			}

			filename := object

			// Skip system/configuration files
			if isSystemFile(filename) {
				continue
			}
			fullObjectKey := fmt.Sprintf("%s/%s", folderPath, filename)
			metadata, err := h.S3Bucket.GetObjectMetadata(fullObjectKey)
			if err != nil {
				log.Printf("Warning: Could not get metadata for %s: %v", filename, err)
				files = append(files, DatasetFile{
					Name:      filename,
					Size:      0,
					Timestamp: "Unknown",
					Key:       fullObjectKey,
				})
				continue
			}

			files = append(files, DatasetFile{
				Name:      filename,
				Size:      metadata.Size,
				Timestamp: metadata.LastModified.Format("2006-01-02 15:04:05"),
				Key:       fullObjectKey,
			})
		}
	}

	log.Printf("Found %d output files in batch %s for app %s", len(files), batchName, appType)

	// Return regular response (backward compatibility)
	response := ListDatasetFilesResponse{
		Success: true,
		Files:   files,
	}
	json.NewEncoder(w).Encode(response)
}

// extractCellNumber extracts the numeric cell number from a PDF filename
// e.g., "0/NGC7025_LR-V_final_cube_voronoi_cell_0_pPXF_fitting.pdf" -> 0
func extractCellNumber(filename string) int {
	// Extract cell directory number from path like "0/filename.pdf"
	if parts := strings.Split(filename, "/"); len(parts) > 0 {
		if cellNum, err := strconv.Atoi(parts[0]); err == nil {
			return cellNum
		}
	}
	return 0 // fallback
}

// ListDatasetOutputFilesPaginated returns a paginated list of files in a dataset's output folder
func (h *FileUploadHandler) ListDatasetOutputFilesPaginated(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get batch name from URL query parameter
	batchName := r.URL.Query().Get("dataset")
	if batchName == "" {
		batchName = r.URL.Query().Get("batch") // also support 'batch' parameter
	}

	if batchName == "" {
		response := PaginatedDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Dataset name parameter is required",
			Total:   0,
			Offset:  0,
			Limit:   0,
			HasMore: false,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get application type from query parameter (default to starlight)
	appType := r.URL.Query().Get("app")
	if appType == "" {
		appType = "starlight" // default
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := PaginatedDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
			Total:   0,
			Offset:  0,
			Limit:   0,
			HasMore: false,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse pagination parameters
	limit := 50 // default
	offset := 0 // default

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Validate batch name
	if strings.Contains(batchName, "/") || strings.Contains(batchName, "\\") {
		response := PaginatedDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Invalid batch name",
			Total:   0,
			Offset:  offset,
			Limit:   limit,
			HasMore: false,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// List objects in the specific batch's output folder
	folderPath := fmt.Sprintf("%s/output/%s", appType, batchName)

	// Collect all PDF files first, then paginate the PDF list
	allPdfFiles := []DatasetFile{}
	continuationToken := ""

	// Get all S3 objects and filter for PDFs
	for {
		page, err := h.S3Bucket.GetS3ObjectsPaginated(folderPath, 1000, continuationToken) // Large batch to get all files
		if err != nil {
			log.Printf("Error listing S3 objects for batch %s output in app %s: %v", batchName, appType, err)
			response := PaginatedDatasetFilesResponse{
				Success: false,
				Files:   []DatasetFile{},
				Message: "Failed to list batch output files",
				Total:   0,
				Offset:  offset,
				Limit:   limit,
				HasMore: false,
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Filter and collect PDF files from this S3 page
		for _, object := range page.Objects {
			filename := object

			// Filter for PDF files in cell subdirectories (contain "/")
			if strings.Contains(filename, "/") && strings.HasSuffix(strings.ToLower(filename), ".pdf") {
				fullObjectKey := fmt.Sprintf("%s/%s", folderPath, filename)

				allPdfFiles = append(allPdfFiles, DatasetFile{
					Name:      filename,
					Size:      0,        // Size not critical for pagination
					Timestamp: "Recent", // Timestamp not critical for pagination
					Key:       fullObjectKey,
				})
			}
		}

		// Check if there are more S3 pages
		if !page.IsTruncated {
			break
		}
		continuationToken = page.NextContinuationToken
	}

	// Sort PDFs by cell number (numeric sort)
	sort.Slice(allPdfFiles, func(i, j int) bool {
		cellA := extractCellNumber(allPdfFiles[i].Name)
		cellB := extractCellNumber(allPdfFiles[j].Name)
		return cellA < cellB
	})

	// Apply pagination to the PDF list
	totalPdfs := len(allPdfFiles)
	startIndex := offset
	endIndex := offset + limit

	if startIndex >= totalPdfs {
		startIndex = totalPdfs
	}
	if endIndex > totalPdfs {
		endIndex = totalPdfs
	}

	paginatedPdfs := []DatasetFile{}
	if startIndex < totalPdfs {
		paginatedPdfs = allPdfFiles[startIndex:endIndex]
	}

	hasMore := endIndex < totalPdfs

	log.Printf("Smart PDF pagination for batch %s (%s): returning %d PDFs (offset: %d, limit: %d, total PDFs: %d, hasMore: %t)",
		batchName, appType, len(paginatedPdfs), offset, limit, totalPdfs, hasMore)

	response := PaginatedDatasetFilesResponse{
		Success: true,
		Files:   paginatedPdfs,
		Total:   totalPdfs,
		Offset:  offset,
		Limit:   limit,
		HasMore: hasMore,
	}
	json.NewEncoder(w).Encode(response)
}

// ListDatasetFilesUnified is a robust, unified endpoint for listing dataset files
// Supports input, processed, and output files with consistent offset-based pagination
func (h *FileUploadHandler) ListDatasetFilesUnified(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse required parameters
	dataset := strings.TrimSpace(r.URL.Query().Get("dataset"))
	fileType := strings.TrimSpace(r.URL.Query().Get("type"))
	appType := strings.TrimSpace(r.URL.Query().Get("app"))

	// Validate required parameters
	if dataset == "" {
		response := UnifiedFilesResponse{
			Success: false,
			Message: "Dataset parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if fileType == "" {
		response := UnifiedFilesResponse{
			Success: false,
			Message: "Type parameter is required (input, processed, output)",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if appType == "" {
		appType = "starlight" // default
	}

	// Validate file type
	validTypes := []string{"input", "processed", "output"}
	isValidType := false
	for _, validType := range validTypes {
		if strings.ToLower(fileType) == validType {
			fileType = validType
			isValidType = true
			break
		}
	}

	if !isValidType {
		response := UnifiedFilesResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid type. Allowed: %s", strings.Join(validTypes, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := UnifiedFilesResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid app type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse pagination parameters
	offset := 0
	limit := 50 // default page size

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	// Validate dataset name
	if strings.Contains(dataset, "/") || strings.Contains(dataset, "\\") {
		response := UnifiedFilesResponse{
			Success: false,
			Message: "Invalid dataset name",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get all objects from S3 and filter them
	var allObjects []string
	var err error

	switch fileType {
	case "input":
		// For input files, search both input and processed directories (like legacy API)
		inputPath := fmt.Sprintf("%s/input/%s", appType, dataset)
		processedPath := fmt.Sprintf("%s/processed/%s", appType, dataset)

		// Get files from input directory
		inputObjects, inputErr := h.S3Bucket.GetS3Objects(inputPath)
		if inputErr != nil {
			log.Printf("Warning: Could not list input objects for %s %s in app %s: %v", fileType, dataset, appType, inputErr)
			inputObjects = []string{}
		}

		// Get files from processed directory
		processedObjects, processedErr := h.S3Bucket.GetS3Objects(processedPath)
		if processedErr != nil {
			log.Printf("Warning: Could not list processed objects for %s %s in app %s: %v", fileType, dataset, appType, processedErr)
			processedObjects = []string{}
		}

		// Prefix files with their directory type for later key reconstruction
		for _, obj := range inputObjects {
			allObjects = append(allObjects, "input/"+obj)
		}
		for _, obj := range processedObjects {
			allObjects = append(allObjects, "processed/"+obj)
		}

	case "processed":
		// For processed files, only search processed directory
		folderPath := fmt.Sprintf("%s/processed/%s", appType, dataset)
		allObjects, err = h.S3Bucket.GetS3Objects(folderPath)
		if err != nil {
			log.Printf("Error listing S3 objects for %s %s in app %s: %v", fileType, dataset, appType, err)
			response := UnifiedFilesResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to list %s files", fileType),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

	case "output":
		// For output files, only search output directory
		folderPath := fmt.Sprintf("%s/output/%s", appType, dataset)
		allObjects, err = h.S3Bucket.GetS3Objects(folderPath)
		if err != nil {
			log.Printf("Error listing S3 objects for %s %s in app %s: %v", fileType, dataset, appType, err)
			response := UnifiedFilesResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to list %s files", fileType),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Filter out directory markers and system files
	var filteredFiles []string
	for _, object := range allObjects {
		if strings.HasSuffix(object, "/") {
			continue // Skip directory markers
		}

		// For input files with prefixes, extract the actual filename for system file check
		var actualFilename string
		if fileType == "input" {
			if strings.HasPrefix(object, "input/") {
				actualFilename = strings.TrimPrefix(object, "input/")
			} else if strings.HasPrefix(object, "processed/") {
				actualFilename = strings.TrimPrefix(object, "processed/")
			} else {
				actualFilename = object
			}
		} else {
			actualFilename = object
		}

		// Check if it's a system file using the actual filename
		if !isSystemFile(actualFilename) {
			filteredFiles = append(filteredFiles, object)
		}
	}

	// Apply stable sorting based on app type
	if strings.ToLower(appType) == "ppxf" && fileType == "output" {
		// Numerical sorting for pPXF output files by directory number
		sort.Slice(filteredFiles, func(i, j int) bool {
			partsA := strings.Split(filteredFiles[i], "/")
			partsB := strings.Split(filteredFiles[j], "/")

			if len(partsA) == 0 || len(partsB) == 0 {
				return filteredFiles[i] < filteredFiles[j]
			}

			// Convert directory names to integers for proper numerical sorting
			dirA, errA := strconv.Atoi(partsA[0])
			dirB, errB := strconv.Atoi(partsB[0])

			// If parsing fails, fall back to string comparison
			if errA != nil || errB != nil {
				return filteredFiles[i] < filteredFiles[j]
			}

			return dirA < dirB
		})
	} else {
		// Alphabetical sorting for other cases
		sort.Strings(filteredFiles)
	}

	// Calculate total and pagination boundaries
	totalFiles := len(filteredFiles)
	startIndex := offset
	endIndex := offset + limit

	// Ensure we don't go out of bounds
	if startIndex > totalFiles {
		startIndex = totalFiles
	}
	if endIndex > totalFiles {
		endIndex = totalFiles
	}

	// Extract the page of files we need
	var pageFiles []string
	if startIndex < totalFiles {
		pageFiles = filteredFiles[startIndex:endIndex]
	}

	// Get metadata for the files in this page
	var files []DatasetFile
	for _, object := range pageFiles {
		var fullObjectKey string
		var displayName string

		// Construct full object key based on file type
		switch fileType {
		case "input":
			// For input files, object includes the prefix (input/ or processed/)
			if strings.HasPrefix(object, "input/") {
				actualFile := strings.TrimPrefix(object, "input/")
				fullObjectKey = fmt.Sprintf("%s/input/%s/%s", appType, dataset, actualFile)
				displayName = actualFile
			} else if strings.HasPrefix(object, "processed/") {
				actualFile := strings.TrimPrefix(object, "processed/")
				fullObjectKey = fmt.Sprintf("%s/processed/%s/%s", appType, dataset, actualFile)
				displayName = actualFile
			} else {
				// Fallback
				fullObjectKey = fmt.Sprintf("%s/input/%s/%s", appType, dataset, object)
				displayName = object
			}
		case "processed":
			fullObjectKey = fmt.Sprintf("%s/processed/%s/%s", appType, dataset, object)
			displayName = object
		case "output":
			fullObjectKey = fmt.Sprintf("%s/output/%s/%s", appType, dataset, object)
			displayName = object
		}
		metadata, err := h.S3Bucket.GetObjectMetadata(fullObjectKey)
		if err != nil {
			log.Printf("Warning: Could not get metadata for %s: %v", object, err)
			files = append(files, DatasetFile{
				Name:      displayName,
				Size:      0,
				Timestamp: "Unknown",
				Key:       fullObjectKey,
			})
			continue
		}

		files = append(files, DatasetFile{
			Name:      displayName,
			Size:      metadata.Size,
			Timestamp: metadata.LastModified.Format("2006-01-02 15:04:05"),
			Key:       fullObjectKey,
		})
	}

	hasMore := endIndex < totalFiles

	log.Printf("Unified file listing for %s %s (%s): returning %d files (offset: %d, limit: %d, total: %d, hasMore: %t)",
		fileType, dataset, appType, len(files), offset, limit, totalFiles, hasMore)

	response := UnifiedFilesResponse{
		Success: true,
		Files:   files,
		Pagination: struct {
			Offset  int  `json:"offset"`
			Limit   int  `json:"limit"`
			Total   int  `json:"total"`
			HasMore bool `json:"hasMore"`
		}{
			Offset:  offset,
			Limit:   limit,
			Total:   totalFiles,
			HasMore: hasMore,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// Progress tracking endpoints

// GetDatasetProgress returns the progress for a specific dataset
func (h *FileUploadHandler) GetDatasetProgress(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get dataset ID from URL query parameter
	datasetID := r.URL.Query().Get("dataset_id")
	if datasetID == "" {
		response := ProgressResponse{
			Success: false,
			Message: "Dataset ID parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get progress from tracker
	progress, exists := h.ProgressTracker.GetProgress(datasetID)
	if !exists {
		response := ProgressResponse{
			Success: false,
			Message: "Dataset progress not found",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := ProgressResponse{
		Success:  true,
		Progress: progress,
	}
	json.NewEncoder(w).Encode(response)
}

// GetAllProgress returns the progress for all datasets
func (h *FileUploadHandler) GetAllProgress(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all progress from tracker
	allProgress := h.ProgressTracker.GetAllProgress()

	response := AllProgressResponse{
		Success:  true,
		Progress: allProgress,
	}
	json.NewEncoder(w).Encode(response)
}

// UpdateProgress updates the progress for a dataset (called by pipeline processes)
func (h *FileUploadHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var request ProgressUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding progress update request: %v", err)
		response := ProgressResponse{
			Success: false,
			Message: "Invalid request format",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if request.DatasetID == "" {
		response := ProgressResponse{
			Success: false,
			Message: "Dataset ID is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update progress
	h.ProgressTracker.UpdateProgress(request.DatasetID, request.Stage, request.Progress)

	response := ProgressResponse{
		Success: true,
		Message: "Progress updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// DownloadFile handles downloading files from S3
func (h *FileUploadHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	objectKey := r.URL.Query().Get("key")
	if objectKey == "" {
		http.Error(w, "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Downloading file with key: %s", objectKey)

	// Check if file exists first
	metadata, err := h.S3Bucket.GetObjectMetadata(objectKey)
	if err != nil {
		log.Printf("File metadata check failed for key '%s': %v", objectKey, err)
	} else {
		log.Printf("File exists - Size: %d bytes, LastModified: %s", metadata.Size, metadata.LastModified)
	}

	content, err := h.S3Bucket.DownloadFile(objectKey)
	if err != nil {
		log.Printf("Error downloading file '%s': %v", objectKey, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	filename := filepath.Base(objectKey)
	if strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		w.Header().Set("Content-Type", "application/pdf")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))

	_, err = w.Write(content)
	if err != nil {
		log.Printf("Error writing file content: %v", err)
	}
}

// DownloadAllFiles handles bulk downloading of all files from a dataset as a zip
func (h *FileUploadHandler) DownloadAllFiles(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	dataset := r.URL.Query().Get("dataset")
	appType := r.URL.Query().Get("app")
	fileType := r.URL.Query().Get("type") // input, processed, or output

	if dataset == "" {
		http.Error(w, "Missing 'dataset' parameter", http.StatusBadRequest)
		return
	}

	if appType == "" {
		appType = "starlight" // default
	}

	if fileType == "" {
		fileType = "output" // default to output files
	}

	// Validate app type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		http.Error(w, fmt.Sprintf("Invalid app type. Allowed: %s", strings.Join(allowedApps, ", ")), http.StatusBadRequest)
		return
	}

	// Validate file type
	allowedFileTypes := []string{"input", "processed", "output"}
	isValidFileType := false
	for _, ft := range allowedFileTypes {
		if strings.ToLower(fileType) == ft {
			fileType = ft
			isValidFileType = true
			break
		}
	}

	if !isValidFileType {
		http.Error(w, fmt.Sprintf("Invalid file type. Allowed: %s", strings.Join(allowedFileTypes, ", ")), http.StatusBadRequest)
		return
	}

	log.Printf("Bulk download request - dataset: %s, app: %s, type: %s", dataset, appType, fileType)

	// Construct S3 path
	var folderPath string
	switch fileType {
	case "input":
		folderPath = fmt.Sprintf("%s/input/%s", appType, dataset)
	case "processed":
		folderPath = fmt.Sprintf("%s/processed/%s", appType, dataset)
	case "output":
		folderPath = fmt.Sprintf("%s/output/%s", appType, dataset)
	}

	// Get all files from S3
	allObjects, err := h.S3Bucket.GetS3Objects(folderPath)
	if err != nil {
		log.Printf("Error listing S3 objects for %s %s in app %s: %v", fileType, dataset, appType, err)
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	// Filter out directory markers and system files
	var files []string
	for _, object := range allObjects {
		if strings.HasSuffix(object, "/") {
			continue // Skip directory markers
		}
		if !isSystemFile(object) {
			files = append(files, object)
		}
	}

	if len(files) == 0 {
		http.Error(w, "No files found to download", http.StatusNotFound)
		return
	}

	log.Printf("Found %d files to zip for dataset %s", len(files), dataset)

	// Create zip file in memory
	zipBuffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuffer)

	// Download each file from S3 and add to zip
	filesAdded := 0
	for _, fileName := range files {
		fullObjectKey := fmt.Sprintf("%s/%s", folderPath, fileName)

		log.Printf("Downloading file %d/%d: %s", filesAdded+1, len(files), fileName)

		content, err := h.S3Bucket.DownloadFile(fullObjectKey)
		if err != nil {
			log.Printf("Warning: Could not download file %s: %v", fileName, err)
			continue // Skip failed files
		}

		// Create file in zip
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			log.Printf("Warning: Could not create zip entry for %s: %v", fileName, err)
			continue
		}

		// Write content to zip
		_, err = fileWriter.Write(content)
		if err != nil {
			log.Printf("Warning: Could not write content for %s: %v", fileName, err)
			continue
		}

		filesAdded++
	}

	// Close zip writer
	err = zipWriter.Close()
	if err != nil {
		log.Printf("Error closing zip writer: %v", err)
		http.Error(w, "Failed to create zip file", http.StatusInternalServerError)
		return
	}

	if filesAdded == 0 {
		http.Error(w, "No files could be added to zip", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created zip with %d/%d files for dataset %s", filesAdded, len(files), dataset)

	// Set headers for zip download
	timestamp := time.Now().Format("20060102-150405")
	zipFileName := fmt.Sprintf("%s-%s-%s-%s.zip", appType, dataset, fileType, timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipBuffer.Len()))

	// Write zip to response
	_, err = w.Write(zipBuffer.Bytes())
	if err != nil {
		log.Printf("Error writing zip content: %v", err)
	}

	log.Printf("Successfully sent zip file %s (%d bytes)", zipFileName, zipBuffer.Len())
}

type ProcessDatasetRequest struct {
	Dataset       string `json:"dataset"`
	ProcessorType string `json:"processorType"`
}

type ProcessDatasetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *FileUploadHandler) ProcessDataset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessDatasetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Processing request for dataset: %s, processor: %s", req.Dataset, req.ProcessorType)

	appType := strings.ToUpper(req.ProcessorType)
	log.Printf("Processing trigger received for batch: %s, app: %s", req.Dataset, appType)

	// Make HTTP call to watcher container to trigger processing
	triggerURL := "http://localhost:8081/trigger-processing"
	triggerData := map[string]string{
		"dataset":   req.Dataset,
		"processor": req.ProcessorType,
	}

	jsonData, _ := json.Marshal(triggerData)
	log.Printf("Triggering processing: POST %s with data: %s", triggerURL, string(jsonData))

	resp, err := http.Post(triggerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error triggering processing: %v", err)
		response := ProcessDatasetResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to trigger processing: %v. Check if producer watcher is running on port 8081", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Trigger endpoint returned status %d: %s", resp.StatusCode, string(body))
		response := ProcessDatasetResponse{
			Success: false,
			Message: fmt.Sprintf("Trigger endpoint returned status %d", resp.StatusCode),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Successfully triggered processing for batch: %s", req.Dataset)

	response := ProcessDatasetResponse{
		Success: true,
		Message: fmt.Sprintf("Processing started for dataset %s", req.Dataset),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type ProcessSingleFileRequest struct {
	Dataset       string `json:"dataset"`
	FileName      string `json:"fileName"`
	ProcessorType string `json:"processorType"`
}

type ProcessSingleFileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *FileUploadHandler) ProcessSingleFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessSingleFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Single file processing request - dataset: %s, file: %s, processor: %s",
		req.Dataset, req.FileName, req.ProcessorType)

	appType := strings.ToUpper(req.ProcessorType)
	log.Printf("Single file processing trigger received - batch: %s, file: %s, app: %s",
		req.Dataset, req.FileName, appType)

	// Make HTTP call to watcher container to trigger single file processing
	triggerURL := "http://localhost:8081/trigger-single-file"
	triggerData := map[string]string{
		"dataset":   req.Dataset,
		"fileName":  req.FileName,
		"processor": req.ProcessorType,
	}

	jsonData, _ := json.Marshal(triggerData)
	resp, err := http.Post(triggerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error triggering single file processing: %v", err)
		http.Error(w, "Failed to trigger single file processing", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	log.Printf("Successfully triggered single file processing - batch: %s, file: %s", req.Dataset, req.FileName)

	response := ProcessSingleFileResponse{
		Success: true,
		Message: fmt.Sprintf("Processing started for file %s in dataset %s", req.FileName, req.Dataset),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type DeleteAllFilesResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	DeletedCount int    `json:"deletedCount,omitempty"`
}

func (h *FileUploadHandler) DeleteAllFiles(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from query
	dataset := r.URL.Query().Get("dataset")
	appType := r.URL.Query().Get("app")
	fileType := r.URL.Query().Get("type") // "input" or "output"

	if dataset == "" {
		response := DeleteAllFilesResponse{
			Success: false,
			Message: "Dataset parameter is required",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if appType == "" {
		appType = "starlight" // default
	}

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "voronoi", "steckmap"}
	isValidApp := false
	for _, app := range allowedApps {
		if strings.ToLower(appType) == app {
			appType = app
			isValidApp = true
			break
		}
	}

	if !isValidApp {
		response := DeleteAllFilesResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid application type. Allowed: %s", strings.Join(allowedApps, ", ")),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if fileType != "input" && fileType != "output" {
		response := DeleteAllFilesResponse{
			Success: false,
			Message: "Type parameter must be 'input' or 'output'",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Determine directories to delete
	var directories []string
	if fileType == "input" {
		// Delete both input and processed directories
		directories = []string{
			fmt.Sprintf("%s/input/%s", appType, dataset),
			fmt.Sprintf("%s/processed/%s", appType, dataset),
		}
	} else {
		// Delete output directory
		directories = []string{
			fmt.Sprintf("%s/output/%s", appType, dataset),
		}
	}

	totalDeleted := 0
	var errors []string

	// Delete each directory
	for _, dir := range directories {
		log.Printf("Starting bulk delete for directory: %s", dir)

		// Use optimized bulk delete with pagination
		deletedCount, err := h.deleteDirectoryWithPagination(dir)
		if err != nil {
			log.Printf("Error deleting directory %s: %v", dir, err)
			errors = append(errors, fmt.Sprintf("%s: %v", dir, err))
			continue
		}

		totalDeleted += deletedCount
		log.Printf("Successfully deleted %d files from %s", deletedCount, dir)
	}

	// Prepare response
	var response DeleteAllFilesResponse
	if len(errors) > 0 {
		response = DeleteAllFilesResponse{
			Success:      false,
			Message:      fmt.Sprintf("Partial success. Deleted %d files. Errors: %v", totalDeleted, errors),
			DeletedCount: totalDeleted,
		}
		w.WriteHeader(http.StatusPartialContent)
	} else if totalDeleted == 0 {
		response = DeleteAllFilesResponse{
			Success:      true,
			Message:      "No files found to delete",
			DeletedCount: 0,
		}
	} else {
		response = DeleteAllFilesResponse{
			Success:      true,
			Message:      fmt.Sprintf("Successfully deleted %d files", totalDeleted),
			DeletedCount: totalDeleted,
		}
	}

	json.NewEncoder(w).Encode(response)
}

// deleteDirectoryWithPagination deletes all files in a directory using ListObjectsV2 with pagination
// Returns the number of files deleted
func (h *FileUploadHandler) deleteDirectoryWithPagination(directoryPath string) (int, error) {
	// Ensure the directory path ends with a slash
	if !strings.HasSuffix(directoryPath, "/") {
		directoryPath = directoryPath + "/"
	}

	totalDeleted := 0
	var continuationToken *string
	batchSize := 1000 // S3 allows up to 1000 objects per delete batch

	for {
		// List objects with pagination
		listInput := &s3.ListObjectsV2Input{
			Bucket:            aws.String(h.S3Bucket.GetBucketName()),
			Prefix:            aws.String(directoryPath),
			MaxKeys:           aws.Int64(1000), // List up to 1000 at a time
			ContinuationToken: continuationToken,
		}

		resp, err := h.S3Bucket.GetS3Client().ListObjectsV2(listInput)
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to list objects in directory %s: %v", directoryPath, err)
		}

		// If no objects found, we're done
		if len(resp.Contents) == 0 {
			break
		}

		// Prepare delete batch
		var deleteObjects []*s3.ObjectIdentifier
		for _, object := range resp.Contents {
			deleteObjects = append(deleteObjects, &s3.ObjectIdentifier{
				Key: object.Key,
			})
		}

		// Delete in batches of 1000 (S3 limit)
		for i := 0; i < len(deleteObjects); i += batchSize {
			end := i + batchSize
			if end > len(deleteObjects) {
				end = len(deleteObjects)
			}

			batchObjects := deleteObjects[i:end]

			deleteResp, err := h.S3Bucket.GetS3Client().DeleteObjects(&s3.DeleteObjectsInput{
				Bucket: aws.String(h.S3Bucket.GetBucketName()),
				Delete: &s3.Delete{
					Objects: batchObjects,
					Quiet:   aws.Bool(true),
				},
			})

			if err != nil {
				return totalDeleted, fmt.Errorf("failed to delete objects batch: %v", err)
			}

			// Count successfully deleted objects
			deletedInBatch := len(batchObjects)
			if len(deleteResp.Errors) > 0 {
				// Some objects failed to delete
				deletedInBatch = len(batchObjects) - len(deleteResp.Errors)
				for _, delErr := range deleteResp.Errors {
					log.Printf("Warning: Failed to delete %s: %s", *delErr.Key, *delErr.Message)
				}
			}

			totalDeleted += deletedInBatch
			log.Printf("Deleted batch: %d files (total so far: %d)", deletedInBatch, totalDeleted)
		}

		// Check if there are more objects to list
		if !aws.BoolValue(resp.IsTruncated) {
			break
		}

		// Set continuation token for next iteration
		continuationToken = resp.NextContinuationToken
	}

	return totalDeleted, nil
}

type RestartQueueResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *FileUploadHandler) RestartRabbitMQ(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Restart queue request received - restarting RabbitMQ, consumer, and producer deployments")

	// Use Kubernetes Go client to restart deployments
	// This uses in-cluster config (automatically available in pods)
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Error getting in-cluster config: %v", err)
		response := RestartQueueResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to connect to Kubernetes: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Error creating Kubernetes client: %v", err)
		response := RestartQueueResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create Kubernetes client: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	namespace := "astroapp"
	deployments := []string{"rabbitmq", "consumer", "ucm-producer-deployment"}

	var errors []string
	var successes []string

	// Create context with timeout for Kubernetes operations
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	for _, deploymentName := range deployments {
		// Get deployment
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			log.Printf("Error getting deployment %s: %v", deploymentName, err)
			errors = append(errors, fmt.Sprintf("%s: %v", deploymentName, err))
			continue
		}

		// Add annotation to trigger rollout restart
		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

		// Update deployment
		_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Error restarting deployment %s: %v", deploymentName, err)
			errors = append(errors, fmt.Sprintf("%s: %v", deploymentName, err))
			continue
		}

		log.Printf("Successfully restarted deployment: %s", deploymentName)
		successes = append(successes, deploymentName)
	}

	var response RestartQueueResponse
	if len(errors) > 0 {
		response = RestartQueueResponse{
			Success: false,
			Message: fmt.Sprintf("Some deployments failed to restart. Successes: %v. Errors: %v", successes, errors),
		}
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		response = RestartQueueResponse{
			Success: true,
			Message: fmt.Sprintf("Successfully restarted deployments: %v", successes),
		}
	}

	json.NewEncoder(w).Encode(response)
}
