package api

import "time"

// Progress tracking data structures
type PipelineStage string

const (
	StageReady      PipelineStage = "ready"
	StageQueued     PipelineStage = "queued"
	StageProcessing PipelineStage = "processing"
	StageAnalysis   PipelineStage = "analysis"
	StageComplete   PipelineStage = "complete"
	StageError      PipelineStage = "error"
)

type DatasetProgress struct {
	DatasetID      string        `json:"dataset_id"`
	DatasetName    string        `json:"dataset_name"`
	Stage          PipelineStage `json:"stage"`
	Progress       float64       `json:"progress"` // 0-100
	FilesTotal     int           `json:"files_total"`
	FilesProcessed int           `json:"files_processed"`
	JobesTotal     int           `json:"jobes_total"`
	JobesProcessed int           `json:"jobes_processed"`
	StartTime      time.Time     `json:"start_time"`
	LastUpdated    time.Time     `json:"last_updated"`
	ErrorMessage   string        `json:"error_message,omitempty"`
}

// Progress tracking responses
type ProgressResponse struct {
	Success  bool             `json:"success"`
	Progress *DatasetProgress `json:"progress,omitempty"`
	Message  string           `json:"message,omitempty"`
}

type AllProgressResponse struct {
	Success  bool                        `json:"success"`
	Progress map[string]*DatasetProgress `json:"progress"`
	Message  string                      `json:"message,omitempty"`
}

// Progress update requests
type ProgressUpdateRequest struct {
	DatasetID   string        `json:"dataset_id"`
	DatasetName string        `json:"dataset_name"`
	Stage       PipelineStage `json:"stage"`
	Progress    float64       `json:"progress"`
	FilesTotal  int           `json:"files_total,omitempty"`
	JobInfo     *JobInfo      `json:"job_info,omitempty"`
}

type JobInfo struct {
	JobID      string `json:"job_id"`
	JobSize    int    `json:"job_size"`
	IsComplete bool   `json:"is_complete"`
}

type DataFile struct {
	Name    string `json:"Name"`
	Content string `json:"Content"`
}

// BinaryDataFile for handling binary files like .fits (PPXF) without string corruption
type BinaryDataFile struct {
	Name    string `json:"Name"`
	Content []byte `json:"Content"`
	Size    int64  `json:"Size"`
}

// IsAppBinary determines if an app processes binary files
func IsAppBinary(appName string) bool {
	switch appName {
	case "PPXF":
		return true
	case "STARLIGHT", "STECKMAP":
		return false
	default:
		return false // Default to text processing
	}
}

type MessageBody struct {
	Files []DataFile `json:"Files"`
}

// BinaryMessageBody for handling binary files
type BinaryMessageBody struct {
	Files []BinaryDataFile `json:"Files"`
}

type Batch struct {
	ID    string     `json:"ID"`
	JobID string     `json:"JobID"`
	Files []DataFile `json:"Files"`
}

// BinaryBatch for handling binary files
type BinaryBatch struct {
	ID    string           `json:"ID"`
	JobID string           `json:"JobID"`
	Files []BinaryDataFile `json:"Files"`
}
