package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type FileUploadHandler struct {
	S3Bucket        s3bucket.S3BucketInterface
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
	DatasetName string `json:"datasetName"`
	AppType     string `json:"appType"`
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

func NewFileUploadHandler(s3Bucket s3bucket.S3BucketInterface, progressTracker *ProgressTracker) *FileUploadHandler {
	return &FileUploadHandler{
		S3Bucket:        s3Bucket,
		ProgressTracker: progressTracker,
	}
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

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "steckmap"}
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

	// Upload to S3 using the new batch structure: app/input/batch_name/filename
	folderPath := fmt.Sprintf("%s/input/%s", appType, batchName)
	err = h.S3Bucket.UploadFileToBucket(folderPath, fileName, content)
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
	allowedApps := []string{"starlight", "ppxf", "steckmap"}
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

	// Use the new GetBatchDirectories method to get all batch directories
	inputPrefix := fmt.Sprintf("%s/input", appType)
	batchDirectories, err := h.S3Bucket.GetBatchDirectories(inputPrefix)
	if err != nil {
		log.Printf("Error listing batch directories for %s: %v", appType, err)
		response := ListDatasetsResponse{
			Success:  false,
			Datasets: []string{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Found %d batch directories for %s: %v", len(batchDirectories), appType, batchDirectories)

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
    allowedApps := []string{"starlight", "ppxf", "steckmap"}
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

    // Create each directory in S3
    for _, dir := range directories {
        // Check if directory already exists
        _, err := h.S3Bucket.GetS3Client().HeadObject(&s3.HeadObjectInput{
            Bucket: aws.String(h.S3Bucket.GetBucketName()),
            Key:    aws.String(dir),
        })

        if err == nil {
            log.Printf("Directory already exists: %s", dir)
            continue
        }

        // If error is not "Not Found", return error
        if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NotFound" {
            log.Printf("Error checking directory existence: %v", err)
            response := CreateDatasetResponse{
                Success: false,
                Message: fmt.Sprintf("Failed to check directory %s", dir),
            }
            json.NewEncoder(w).Encode(response)
            return
        }

        // Create directory by putting an empty object with trailing slash
        _, err = h.S3Bucket.GetS3Client().PutObject(&s3.PutObjectInput{
            Bucket: aws.String(h.S3Bucket.GetBucketName()),
            Key:    aws.String(dir),
        })
        if err != nil {
            log.Printf("Error creating directory: %v", err)
            response := CreateDatasetResponse{
                Success: false,
                Message: fmt.Sprintf("Failed to create directory %s", dir),
            }
            json.NewEncoder(w).Encode(response)
            return
        }
        log.Printf("Successfully created directory: %s", dir)
    }

    // Success response
    response := CreateDatasetResponse{
        Success: true,
        Message: fmt.Sprintf("Dataset '%s' created successfully for application '%s'", sanitizedName, appType),
    }
    json.NewEncoder(w).Encode(response)
    log.Printf("Successfully created dataset: %s for application: %s", sanitizedName, appType)
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
	allowedApps := []string{"starlight", "ppxf", "steckmap"}
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
	allowedApps := []string{"starlight", "ppxf", "steckmap"}
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

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "steckmap"}
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

	// List objects in the specific batch's processed folder
	folderPath := fmt.Sprintf("%s/processed/%s", appType, batchName)
	log.Printf("DEBUG: Listing objects in processed folder: %s", folderPath)
	objects, err := h.S3Bucket.GetS3Objects(folderPath)
	if err != nil {
		log.Printf("Error listing S3 objects for batch %s processed in app %s: %v", batchName, appType, err)
		response := ListDatasetFilesResponse{
			Success: false,
			Files:   []DatasetFile{},
			Message: "Failed to list batch processed files",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	log.Printf("DEBUG: Found %d objects in processed folder %s", len(objects), folderPath)

	// Convert objects to DatasetFile structs
	files := make([]DatasetFile, 0)
	for _, object := range objects {
		// GetS3Objects returns just the filename (without the full path)
		filename := object


		// Get file metadata from S3
		fullObjectKey := fmt.Sprintf("%s/%s", folderPath, filename)
		metadata, err := h.S3Bucket.GetObjectMetadata(fullObjectKey)
		if err != nil {
			log.Printf("Warning: Could not get metadata for %s: %v", filename, err)
			// Still include the file with basic info
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

	log.Printf("Found %d processed files in batch %s for app %s", len(files), batchName, appType)

	response := ListDatasetFilesResponse{
		Success: true,
		Files:   files,
	}
	json.NewEncoder(w).Encode(response)
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

	// Validate application type
	allowedApps := []string{"starlight", "ppxf", "steckmap"}
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

	// Convert objects to DatasetFile structs
	files := make([]DatasetFile, 0)
	for _, object := range objects {
		// GetS3Objects returns just the filename (without the full path)
		filename := object


		// Get file metadata from S3
		fullObjectKey := fmt.Sprintf("%s/%s", folderPath, filename)
		metadata, err := h.S3Bucket.GetObjectMetadata(fullObjectKey)
		if err != nil {
			log.Printf("Warning: Could not get metadata for %s: %v", filename, err)
			// Still include the file with basic info
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

	log.Printf("Found %d output files in batch %s for app %s", len(files), batchName, appType)

	response := ListDatasetFilesResponse{
		Success: true,
		Files:   files,
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
