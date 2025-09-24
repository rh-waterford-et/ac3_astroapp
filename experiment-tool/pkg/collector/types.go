package collector

import (
	"fmt"
	"time"
)

// LiveMetrics represents system state at a point in time
type LiveMetrics struct {
	Timestamp         time.Time `json:"timestamp"`
	ProcessorCount    int       `json:"processor_count"`
	ExperimentID      string    `json:"experiment_id"`
	ActiveJobs        int       `json:"active_jobs"`
	CompletedJobs     int       `json:"completed_jobs"`
	QueueDepth        int       `json:"queue_depth"`
	AvgProcessingTime float64   `json:"avg_processing_time_seconds"`
	Throughput        float64   `json:"throughput_jobs_per_minute"`
}

// BatchMetrics represents aggregated metrics for a completed batch
type BatchMetrics struct {
	ExperimentID       string        `json:"experiment_id"`
	BatchID            string        `json:"batch_id"`
	ProcessorCount     int           `json:"processor_count"`
	JobCount           int           `json:"job_count"`
	CompleteJobCount   int           `json:"complete_job_count"`
	AvgQueueTime       time.Duration `json:"avg_queue_time"`
	AvgProcessingTime  time.Duration `json:"avg_processing_time"`
	TotalBatchDuration time.Duration `json:"total_batch_duration"`
	Throughput         float64       `json:"throughput_jobs_per_minute"`
	TotalSizeMB        float64       `json:"total_size_mb"`
}

// JobMetrics represents individual job metrics from Redis
type JobMetrics struct {
	BatchID            string    `json:"batch_id"`
	JobID              string    `json:"job_id"`
	QueueStartTime     time.Time `json:"queue_start_time"`
	QueueReceiveTime   time.Time `json:"queue_receive_time"`
	JobEndTime         time.Time `json:"job_end_time"`
	QueueDuration      float64   `json:"queue_duration"`
	ProcessingDuration float64   `json:"processing_duration"`
	TotalDuration      float64   `json:"total_duration"`
	IsComplete         bool      `json:"is_complete"`
	JobSizeMB          float64   `json:"job_size_mb"`
	QueueAheadLength   int       `json:"queue_ahead_length"`
}

// SystemSnapshot represents current system state for live monitoring
type SystemSnapshot struct {
	Timestamp      time.Time    `json:"timestamp"`
	ProcessorCount int          `json:"processor_count"`
	ActiveBatches  []string     `json:"active_batches"`
	JobMetrics     []JobMetrics `json:"job_metrics"`
}

// ProcessorOutputPattern defines how each processor generates output files
type ProcessorOutputPattern struct {
	ProcessorType    string   `json:"processor_type"`
	FilesPerInput    int      `json:"files_per_input"`    // STARLIGHT: 1, PPXF: 5
	OutputPathFormat string   `json:"output_path_format"` // Path structure pattern
	FilePatterns     []string `json:"file_patterns"`      // Expected file extensions
}

// ProcessorPatterns defines the output patterns for each supported processor
var ProcessorPatterns = map[string]ProcessorOutputPattern{
	"starlight": {
		ProcessorType:    "starlight",
		FilesPerInput:    1,
		OutputPathFormat: "starlight/output/%s/", // /starlight/output/NGC7025/
		FilePatterns:     []string{".txt", ".out"},
	},
	"ppxf": {
		ProcessorType:    "ppxf",
		FilesPerInput:    5,
		OutputPathFormat: "ppxf/output/%s/", // /ppxf/output/NGC7025/*/
		FilePatterns:     []string{".fits", ".txt", ".log", ".png", ".pdf"},
	},
}

// ExperimentRun represents a single experiment execution with processor-specific configuration
type ExperimentRun struct {
	DatasetName         string    `json:"dataset_name"`
	ProcessorType       string    `json:"processor_type"`
	UploadedFileCount   int       `json:"uploaded_file_count"`
	ExpectedOutputCount int       `json:"expected_output_count"` // Calculated: UploadedFileCount * FilesPerInput
	StartTime           time.Time `json:"start_time"`

	// S3 paths
	InputPath      string                 `json:"input_path"`       // "starlight/input/NGC7025/"
	OutputBasePath string                 `json:"output_base_path"` // "starlight/output/NGC7025/"
	OutputPattern  ProcessorOutputPattern `json:"output_pattern"`
}

// NewExperimentRun creates a new experiment run with processor-specific configuration
func NewExperimentRun(datasetName, processorType string, uploadedFileCount int) (*ExperimentRun, error) {
	pattern, exists := ProcessorPatterns[processorType]
	if !exists {
		return nil, fmt.Errorf("unsupported processor type: %s", processorType)
	}

	return &ExperimentRun{
		DatasetName:         datasetName,
		ProcessorType:       processorType,
		UploadedFileCount:   uploadedFileCount,
		ExpectedOutputCount: uploadedFileCount * pattern.FilesPerInput,
		StartTime:           time.Now(),
		InputPath:           fmt.Sprintf("%s/input/%s/", processorType, datasetName),
		OutputBasePath:      fmt.Sprintf(pattern.OutputPathFormat, datasetName),
		OutputPattern:       pattern,
	}, nil
}
