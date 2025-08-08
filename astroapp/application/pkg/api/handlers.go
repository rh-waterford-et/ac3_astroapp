package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
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

		// Filter out directory markers (objects ending with "/") - only count actual files
		var actualFiles []string
		for _, object := range allObjects {
			if !strings.HasSuffix(object, "/") {
				actualFiles = append(actualFiles, object)
			}
		}

		// Calculate pagination boundaries using actual file count
		totalFiles := len(actualFiles)
		allObjects = actualFiles // Use filtered list for pagination
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

		// Convert all objects to DatasetFile structs, but skip directory markers
		for _, object := range objects {
			// Skip directory markers (objects ending with "/")
			if strings.HasSuffix(object, "/") {
				continue
			}

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
	resp, err := http.Post(triggerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error triggering processing: %v", err)
		http.Error(w, "Failed to trigger processing", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	log.Printf("Successfully triggered processing for batch: %s", req.Dataset)

	response := ProcessDatasetResponse{
		Message: fmt.Sprintf("Processing started for dataset %s", req.Dataset),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
