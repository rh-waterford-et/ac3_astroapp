package server

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type Server struct {
	fileUploadHandler *api.FileUploadHandler
	progressTracker   *api.ProgressTracker
}

func NewServer() *Server {
	s3Bucket := s3bucket.NewS3Bucket()

	// Initialize progress tracker
	progressTracker := api.NewProgressTracker()

	fileUploadHandler := api.NewFileUploadHandler(s3Bucket, progressTracker)

	return &Server{
		fileUploadHandler: fileUploadHandler,
		progressTracker:   progressTracker,
	}
}

func (s *Server) setupRoutes() {
	// File upload endpoint
	http.HandleFunc("/api/upload", s.fileUploadHandler.UploadFile)

	// List datasets endpoint
	http.HandleFunc("/api/datasets", s.fileUploadHandler.ListDatasets)

	// List dataset files endpoint
	http.HandleFunc("/api/datasets/files", s.fileUploadHandler.ListDatasetFiles)

	// List dataset output files endpoint
	http.HandleFunc("/api/datasets/output-files", s.fileUploadHandler.ListDatasetOutputFiles)

	// List dataset output files with pagination endpoint
	http.HandleFunc("/api/datasets/output-files-paginated", s.fileUploadHandler.ListDatasetOutputFilesPaginated)

	// Create dataset endpoint
	http.HandleFunc("/api/datasets/create", s.fileUploadHandler.CreateDataset)

	// Delete dataset endpoint
	http.HandleFunc("/api/datasets/delete", s.fileUploadHandler.DeleteDataset)

	// Process dataset endpoint
	http.HandleFunc("/api/datasets/process", s.fileUploadHandler.ProcessDataset)

	// Delete file endpoint
	http.HandleFunc("/api/files/delete", s.fileUploadHandler.DeleteFile)

	// Download file endpoint
	log.Printf("🔧 Registering download endpoint: /api/files/download")
	http.HandleFunc("/api/files/download", s.fileUploadHandler.DownloadFile)
	log.Printf("✅ Download endpoint registered successfully")

	// Progress tracking endpoints
	http.HandleFunc("/api/progress", s.fileUploadHandler.GetDatasetProgress)
	http.HandleFunc("/api/progress/all", s.fileUploadHandler.GetAllProgress)
	http.HandleFunc("/api/progress/update", s.fileUploadHandler.UpdateProgress)

	// Health check endpoint
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})
}

func (s *Server) Start() {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	s.setupRoutes()

	log.Printf("Starting HTTP server on port %s", port)
	log.Printf("File upload endpoint: http://localhost:%s/api/upload", port)
	log.Printf("List datasets endpoint: http://localhost:%s/api/datasets", port)
	log.Printf("List dataset files endpoint: http://localhost:%s/api/datasets/files", port)
	log.Printf("List dataset output files endpoint: http://localhost:%s/api/datasets/output-files", port)
	log.Printf("Create dataset endpoint: http://localhost:%s/api/datasets/create", port)
	log.Printf("Delete dataset endpoint: http://localhost:%s/api/datasets/delete", port)
	log.Printf("Delete file endpoint: http://localhost:%s/api/files/delete", port)
	log.Printf("Download file endpoint: http://localhost:%s/api/files/download", port)
	log.Printf("Process dataset endpoint: http://localhost:%s/api/datasets/process", port)
	log.Printf("Progress tracking endpoints: http://localhost:%s/api/progress/*", port)
	log.Printf("Health check endpoint: http://localhost:%s/api/health", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
