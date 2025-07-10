package api

import "time"

// Progress tracking data structures
type PipelineStage string

const (
	StageReady       PipelineStage = "ready"
	StageQueued      PipelineStage = "queued"
	StageProcessing  PipelineStage = "processing"
	StageAnalysis    PipelineStage = "analysis"
	StageComplete    PipelineStage = "complete"
	StageError       PipelineStage = "error"
)

type DatasetProgress struct {
	DatasetID        string        `json:"dataset_id"`
	DatasetName      string        `json:"dataset_name"`
	Stage            PipelineStage `json:"stage"`
	Progress         float64       `json:"progress"`         // 0-100
	FilesTotal       int           `json:"files_total"`
	FilesProcessed   int           `json:"files_processed"`
	BatchesTotal     int           `json:"batches_total"`
	BatchesProcessed int           `json:"batches_processed"`
	StartTime        time.Time     `json:"start_time"`
	LastUpdated      time.Time     `json:"last_updated"`
	ErrorMessage     string        `json:"error_message,omitempty"`
}

// Progress tracking responses
type ProgressResponse struct {
	Success bool             `json:"success"`
	Progress *DatasetProgress `json:"progress,omitempty"`
	Message string           `json:"message,omitempty"`
}

type AllProgressResponse struct {
	Success  bool                         `json:"success"`
	Progress map[string]*DatasetProgress  `json:"progress"`
	Message  string                       `json:"message,omitempty"`
}

// Progress update requests
type ProgressUpdateRequest struct {
	DatasetID   string        `json:"dataset_id"`
	DatasetName string        `json:"dataset_name"`
	Stage       PipelineStage `json:"stage"`
	Progress    float64       `json:"progress"`
	FilesTotal  int           `json:"files_total,omitempty"`
	BatchInfo   *BatchInfo    `json:"batch_info,omitempty"`
}

type BatchInfo struct {
	BatchID     string `json:"batch_id"`
	BatchSize   int    `json:"batch_size"`
	IsComplete  bool   `json:"is_complete"`
}

type DataFile struct {
	Name    string `json:"Name"`
	Content string `json:"Content"`
}

type MessageBody struct {
	Files []DataFile `json:"Files"`
}

type Event struct {
	ID    string     `json:"ID"`
	Files []DataFile `json:"Files"`
}
