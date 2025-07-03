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
}

func NewServer() *Server {
	s3Bucket := s3bucket.NewS3Bucket()
	fileUploadHandler := api.NewFileUploadHandler(s3Bucket)
	
	return &Server{
		fileUploadHandler: fileUploadHandler,
	}
}

func (s *Server) setupRoutes() {
	// File upload endpoint
	http.HandleFunc("/api/upload", s.fileUploadHandler.UploadFile)
	
	// List datasets endpoint
	http.HandleFunc("/api/datasets", s.fileUploadHandler.ListDatasets)
	
	// Create dataset endpoint
	http.HandleFunc("/api/datasets/create", s.fileUploadHandler.CreateDataset)
	
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
	log.Printf("Create dataset endpoint: http://localhost:%s/api/datasets/create", port)
	log.Printf("Health check endpoint: http://localhost:%s/api/health", port)
	
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
} 