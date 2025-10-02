package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics holds all Prometheus metrics for the application
type PrometheusMetrics struct {
	// HTTP request metrics
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPRequestsInFlight  *prometheus.GaugeVec
	
	// Job processing metrics
	JobsProcessedTotal    *prometheus.CounterVec
	JobProcessingDuration *prometheus.HistogramVec
	JobsInQueue           *prometheus.GaugeVec
	
	// File upload metrics
	FilesUploadedTotal    *prometheus.CounterVec
	FileUploadSize        *prometheus.HistogramVec
	
	// Dataset metrics
	DatasetsCreatedTotal  *prometheus.CounterVec
	DatasetsProcessedTotal *prometheus.CounterVec
	
	// Redis metrics
	RedisOperationsTotal  *prometheus.CounterVec
	RedisOperationDuration *prometheus.HistogramVec
	
	// S3 metrics
	S3OperationsTotal     *prometheus.CounterVec
	S3OperationDuration   *prometheus.HistogramVec
	S3DataTransferred      *prometheus.CounterVec
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		// HTTP request metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		
		HTTPRequestsInFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
			[]string{"method", "endpoint"},
		),
		
		// Job processing metrics
		JobsProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jobs_processed_total",
				Help: "Total number of jobs processed",
			},
			[]string{"job_type", "status"},
		),
		
		JobProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "job_processing_duration_seconds",
				Help:    "Job processing duration in seconds",
				Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
			},
			[]string{"job_type"},
		),
		
		JobsInQueue: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "jobs_in_queue",
				Help: "Number of jobs currently in queue",
			},
			[]string{"queue_name"},
		),
		
		// File upload metrics
		FilesUploadedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "files_uploaded_total",
				Help: "Total number of files uploaded",
			},
			[]string{"dataset_id", "status"},
		),
		
		FileUploadSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "file_upload_size_bytes",
				Help:    "Size of uploaded files in bytes",
				Buckets: prometheus.ExponentialBuckets(1024, 2, 20), // 1KB to 1GB
			},
			[]string{"dataset_id"},
		),
		
		// Dataset metrics
		DatasetsCreatedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "datasets_created_total",
				Help: "Total number of datasets created",
			},
			[]string{"status"},
		),
		
		DatasetsProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "datasets_processed_total",
				Help: "Total number of datasets processed",
			},
			[]string{"status"},
		),
		
		// Redis metrics
		RedisOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_operations_total",
				Help: "Total number of Redis operations",
			},
			[]string{"operation", "status"},
		),
		
		RedisOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "redis_operation_duration_seconds",
				Help:    "Redis operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		
		// S3 metrics
		S3OperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "s3_operations_total",
				Help: "Total number of S3 operations",
			},
			[]string{"operation", "status"},
		),
		
		S3OperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "s3_operation_duration_seconds",
				Help:    "S3 operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		
		S3DataTransferred: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "s3_data_transferred_bytes_total",
				Help: "Total amount of data transferred to/from S3 in bytes",
			},
			[]string{"operation", "direction"},
		),
	}
}

// RecordHTTPRequest records HTTP request metrics
func (pm *PrometheusMetrics) RecordHTTPRequest(method, endpoint, statusCode string, duration float64) {
	pm.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	pm.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
}

// RecordJobProcessed records job processing metrics
func (pm *PrometheusMetrics) RecordJobProcessed(jobType, status string, duration float64) {
	pm.JobsProcessedTotal.WithLabelValues(jobType, status).Inc()
	pm.JobProcessingDuration.WithLabelValues(jobType).Observe(duration)
}

// RecordFileUpload records file upload metrics
func (pm *PrometheusMetrics) RecordFileUpload(datasetID, status string, size int64) {
	pm.FilesUploadedTotal.WithLabelValues(datasetID, status).Inc()
	pm.FileUploadSize.WithLabelValues(datasetID).Observe(float64(size))
}

// RecordDatasetCreated records dataset creation metrics
func (pm *PrometheusMetrics) RecordDatasetCreated(status string) {
	pm.DatasetsCreatedTotal.WithLabelValues(status).Inc()
}

// RecordDatasetProcessed records dataset processing metrics
func (pm *PrometheusMetrics) RecordDatasetProcessed(status string) {
	pm.DatasetsProcessedTotal.WithLabelValues(status).Inc()
}

// RecordRedisOperation records Redis operation metrics
func (pm *PrometheusMetrics) RecordRedisOperation(operation, status string, duration float64) {
	pm.RedisOperationsTotal.WithLabelValues(operation, status).Inc()
	pm.RedisOperationDuration.WithLabelValues(operation).Observe(duration)
}

// RecordS3Operation records S3 operation metrics
func (pm *PrometheusMetrics) RecordS3Operation(operation, status string, duration float64, dataSize int64, direction string) {
	pm.S3OperationsTotal.WithLabelValues(operation, status).Inc()
	pm.S3OperationDuration.WithLabelValues(operation).Observe(duration)
	pm.S3DataTransferred.WithLabelValues(operation, direction).Add(float64(dataSize))
}

// UpdateJobsInQueue updates the number of jobs in queue
func (pm *PrometheusMetrics) UpdateJobsInQueue(queueName string, count int) {
	pm.JobsInQueue.WithLabelValues(queueName).Set(float64(count))
}
