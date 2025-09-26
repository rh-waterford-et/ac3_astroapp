package collector

import (
	"fmt"
	"sync"
	"sync/atomic"
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

	expectedOutputCount := uploadedFileCount * pattern.FilesPerInput

	return &ExperimentRun{
		DatasetName:         datasetName,
		ProcessorType:       processorType,
		UploadedFileCount:   uploadedFileCount,
		ExpectedOutputCount: expectedOutputCount,
		StartTime:           time.Now(),
		InputPath:           fmt.Sprintf("/app/datasets/%s", datasetName),
		OutputBasePath:      fmt.Sprintf("%s/output/%s/", processorType, datasetName),
		OutputPattern:       pattern,
	}, nil
}

// DatasetStatus represents the current state of a dataset in a multi-dataset experiment
type DatasetStatus string

const (
	StatusPending     DatasetStatus = "PENDING"
	StatusUploading   DatasetStatus = "UPLOADING"
	StatusProcessing  DatasetStatus = "PROCESSING"
	StatusAggregating DatasetStatus = "AGGREGATING"
	StatusCompleted   DatasetStatus = "COMPLETED"
	StatusFailed      DatasetStatus = "FAILED"
)

// DatasetExecution tracks the execution state of a single dataset in a multi-dataset experiment
type DatasetExecution struct {
	Config         DatasetConfig
	ExperimentRun  *ExperimentRun
	Status         DatasetStatus
	StartTime      time.Time
	CompletionTime time.Time
	Error          error
	ProcessorCount int
	Mutex          sync.RWMutex
}

// DatasetConfig represents configuration for a single dataset
type DatasetConfig struct {
	Name          string
	ProcessorType string
}

// GetStatus returns the current status of the dataset execution (thread-safe)
func (de *DatasetExecution) GetStatus() DatasetStatus {
	de.Mutex.RLock()
	defer de.Mutex.RUnlock()
	return de.Status
}

// SetStatus updates the status of the dataset execution (thread-safe)
func (de *DatasetExecution) SetStatus(status DatasetStatus) {
	de.Mutex.Lock()
	defer de.Mutex.Unlock()
	de.Status = status
	if status == StatusCompleted || status == StatusFailed {
		de.CompletionTime = time.Now()
	}
}

// SetError sets an error and marks the dataset as failed (thread-safe)
func (de *DatasetExecution) SetError(err error) {
	de.Mutex.Lock()
	defer de.Mutex.Unlock()
	de.Error = err
	de.Status = StatusFailed
	de.CompletionTime = time.Now()
}

// GetError returns the current error (thread-safe)
func (de *DatasetExecution) GetError() error {
	de.Mutex.RLock()
	defer de.Mutex.RUnlock()
	return de.Error
}

// Duration returns the total execution time for the dataset
func (de *DatasetExecution) Duration() time.Duration {
	de.Mutex.RLock()
	defer de.Mutex.RUnlock()

	if de.CompletionTime.IsZero() {
		return time.Since(de.StartTime)
	}
	return de.CompletionTime.Sub(de.StartTime)
}

// MultiDatasetExperiment manages multiple concurrent dataset executions
type MultiDatasetExperiment struct {
	Datasets       map[string]*DatasetExecution
	StartQueue     chan *DatasetExecution
	ActiveCount    int32
	CompletedCount int32
	FailedCount    int32
	Mutex          sync.RWMutex
}

// NewMultiDatasetExperiment creates a new multi-dataset experiment manager
func NewMultiDatasetExperiment(datasetConfigs []DatasetConfig) *MultiDatasetExperiment {
	datasets := make(map[string]*DatasetExecution)

	for _, config := range datasetConfigs {
		datasets[config.Name] = &DatasetExecution{
			Config:    config,
			Status:    StatusPending,
			StartTime: time.Now(),
		}
	}

	return &MultiDatasetExperiment{
		Datasets:   datasets,
		StartQueue: make(chan *DatasetExecution, len(datasetConfigs)),
	}
}

// GetDataset returns a dataset execution by name (thread-safe)
func (mde *MultiDatasetExperiment) GetDataset(name string) (*DatasetExecution, bool) {
	mde.Mutex.RLock()
	defer mde.Mutex.RUnlock()
	dataset, exists := mde.Datasets[name]
	return dataset, exists
}

// GetAllDatasets returns all dataset executions (thread-safe copy)
func (mde *MultiDatasetExperiment) GetAllDatasets() map[string]*DatasetExecution {
	mde.Mutex.RLock()
	defer mde.Mutex.RUnlock()

	result := make(map[string]*DatasetExecution)
	for name, dataset := range mde.Datasets {
		result[name] = dataset
	}
	return result
}

// GetActiveCount returns the number of currently active datasets
func (mde *MultiDatasetExperiment) GetActiveCount() int32 {
	return atomic.LoadInt32(&mde.ActiveCount)
}

// GetCompletedCount returns the number of completed datasets
func (mde *MultiDatasetExperiment) GetCompletedCount() int32 {
	return atomic.LoadInt32(&mde.CompletedCount)
}

// GetFailedCount returns the number of failed datasets
func (mde *MultiDatasetExperiment) GetFailedCount() int32 {
	return atomic.LoadInt32(&mde.FailedCount)
}

// IncrementActiveCount atomically increments the active dataset count
func (mde *MultiDatasetExperiment) IncrementActiveCount() {
	atomic.AddInt32(&mde.ActiveCount, 1)
}

// DecrementActiveCount atomically decrements the active dataset count
func (mde *MultiDatasetExperiment) DecrementActiveCount() {
	atomic.AddInt32(&mde.ActiveCount, -1)
}

// IncrementCompletedCount atomically increments the completed dataset count
func (mde *MultiDatasetExperiment) IncrementCompletedCount() {
	atomic.AddInt32(&mde.CompletedCount, 1)
}

// IncrementFailedCount atomically increments the failed dataset count
func (mde *MultiDatasetExperiment) IncrementFailedCount() {
	atomic.AddInt32(&mde.FailedCount, 1)
}

// GetStatusSummary returns a summary of dataset statuses
func (mde *MultiDatasetExperiment) GetStatusSummary() map[DatasetStatus]int {
	mde.Mutex.RLock()
	defer mde.Mutex.RUnlock()

	summary := make(map[DatasetStatus]int)
	for _, dataset := range mde.Datasets {
		status := dataset.GetStatus()
		summary[status]++
	}
	return summary
}

// IsComplete returns true if all datasets have completed (successfully or failed)
func (mde *MultiDatasetExperiment) IsComplete() bool {
	mde.Mutex.RLock()
	defer mde.Mutex.RUnlock()

	for _, dataset := range mde.Datasets {
		status := dataset.GetStatus()
		if status != StatusCompleted && status != StatusFailed {
			return false
		}
	}
	return true
}

// GetTotalDatasetCount returns the total number of datasets in the experiment
func (mde *MultiDatasetExperiment) GetTotalDatasetCount() int {
	mde.Mutex.RLock()
	defer mde.Mutex.RUnlock()
	return len(mde.Datasets)
}
