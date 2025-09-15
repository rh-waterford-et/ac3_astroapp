package collector

import "time"

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
