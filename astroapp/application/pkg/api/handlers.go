package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type FileUploadHandler struct {
	S3Bucket s3bucket.S3BucketInterface
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
}

type CreateDatasetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewFileUploadHandler(s3Bucket s3bucket.S3BucketInterface) *FileUploadHandler {
	return &FileUploadHandler{
		S3Bucket: s3Bucket,
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
		response := UploadResponse{
			Success: false,
			Message: "Failed to parse upload form",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get dataset name from form
	dataset := r.FormValue("dataset")
	if dataset == "" {
		log.Printf("Dataset parameter missing")
		response := UploadResponse{
			Success: false,
			Message: "Dataset parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate dataset name (basic validation for S3 key compatibility)
	if strings.Contains(dataset, "/") || strings.Contains(dataset, "\\") {
		response := UploadResponse{
			Success: false,
			Message: "Dataset name cannot contain slashes",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get the file from form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from form: %v", err)
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
		response := UploadResponse{
			Success: false,
			Message: "Failed to read file content",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Upload to S3 using existing function with dataset-specific path
	folderPath := fmt.Sprintf("starlight/input/%s", dataset)
	err = h.S3Bucket.UploadFileToBucket(folderPath, fileName, content)
	if err != nil {
		log.Printf("Error uploading file to S3: %v", err)
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
	
	log.Printf("Successfully uploaded file: %s to %s", fileName, folderPath)
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

	// List objects in starlight/input/ to find dataset directories
	objects, err := h.S3Bucket.GetS3Objects("starlight/input")
	if err != nil {
		log.Printf("Error listing S3 objects: %v", err)
		response := ListDatasetsResponse{
			Success:  false,
			Datasets: []string{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Extract unique dataset names from object keys
	datasetMap := make(map[string]bool)
	for _, object := range objects {
		// Object format should be like: dataset_name/filename
		parts := strings.SplitN(object, "/", 2)
		if len(parts) >= 1 && parts[0] != "" {
			datasetMap[parts[0]] = true
		}
	}

	// Convert map to slice
	datasets := make([]string, 0, len(datasetMap))
	for dataset := range datasetMap {
		datasets = append(datasets, dataset)
	}

	log.Printf("Found %d datasets: %v", len(datasets), datasets)

	response := ListDatasetsResponse{
		Success:  true,
		Datasets: datasets,
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
		log.Printf("Error parsing create dataset request: %v", err)
		response := CreateDatasetResponse{
			Success: false,
			Message: "Invalid request format",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate dataset name
	if req.DatasetName == "" {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Dataset name is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Sanitize dataset name
	sanitizedName := strings.TrimSpace(req.DatasetName)
	if strings.Contains(sanitizedName, "/") || strings.Contains(sanitizedName, "\\") {
		response := CreateDatasetResponse{
			Success: false,
			Message: "Dataset name cannot contain slashes",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create placeholder files to establish the dataset directories in S3 (both input and output)
	inputFolderPath := fmt.Sprintf("starlight/input/%s", sanitizedName)
	outputFolderPath := fmt.Sprintf("starlight/output/%s", sanitizedName)
	placeholderContent := []byte(fmt.Sprintf("# Dataset: %s\n# Created: %s\n# This is a placeholder file to establish the dataset directory.\n", 
		sanitizedName, 
		fmt.Sprintf("%d", time.Now().Unix())))
	
	// Create input directory
	err = h.S3Bucket.UploadFileToBucket(inputFolderPath, ".dataset_placeholder", placeholderContent)
	if err != nil {
		log.Printf("Error creating dataset input placeholder: %v", err)
		response := CreateDatasetResponse{
			Success: false,
			Message: "Failed to create dataset input directory in S3",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Create output directory
	err = h.S3Bucket.UploadFileToBucket(outputFolderPath, ".dataset_placeholder", placeholderContent)
	if err != nil {
		log.Printf("Error creating dataset output placeholder: %v", err)
		response := CreateDatasetResponse{
			Success: false,
			Message: "Failed to create dataset output directory in S3",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Successfully created dataset: %s", sanitizedName)
	response := CreateDatasetResponse{
		Success: true,
		Message: "Dataset created successfully",
	}
	json.NewEncoder(w).Encode(response)
} 