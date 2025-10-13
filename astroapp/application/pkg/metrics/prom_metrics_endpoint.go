package metrics

import (
	"context"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusJobMetrics holds all Prometheus metrics collectors
type PrometheusJobMetrics struct {
	queueDuration      *prometheus.GaugeVec
	processingDuration *prometheus.GaugeVec
	totalDuration      *prometheus.GaugeVec
	jobSize            *prometheus.GaugeVec
	queueAheadLength   *prometheus.GaugeVec
	completedJobs      *prometheus.CounterVec
	activeUsers        *prometheus.GaugeVec
	store              *MetricsStore
}

// NewPrometheusJobMetrics creates and registers Prometheus metrics
func NewPrometheusJobMetrics(store *MetricsStore, registry *prometheus.Registry) *PrometheusJobMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	pm := &PrometheusJobMetrics{
		store: store,
		queueDuration: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_job_queue_duration_seconds",
				Help: "Duration a job spent in the queue (seconds)",
			},
			[]string{"batch_id", "job_id"},
		),
		processingDuration: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_job_processing_duration_seconds",
				Help: "Duration of job processing (seconds)",
			},
			[]string{"batch_id", "job_id"},
		),
		totalDuration: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_job_total_duration_seconds",
				Help: "Total duration from queue start to job completion (seconds)",
			},
			[]string{"batch_id", "job_id"},
		),
		jobSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_job_size_mb",
				Help: "Size of the job in megabytes",
			},
			[]string{"batch_id", "job_id"},
		),
		queueAheadLength: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_job_queue_ahead_length",
				Help: "Number of jobs ahead in the queue",
			},
			[]string{"batch_id", "job_id"},
		),
		completedJobs: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "astroapp_completed_jobs_total",
				Help: "Total number of completed jobs",
			},
			[]string{"batch_id"},
		),
	}

	// Register metrics
	registry.MustRegister(pm.queueDuration)
	registry.MustRegister(pm.processingDuration)
	registry.MustRegister(pm.totalDuration)
	registry.MustRegister(pm.jobSize)
	registry.MustRegister(pm.queueAheadLength)
	registry.MustRegister(pm.completedJobs)

	return pm
}

// UpdateMetrics fetches all metrics from the store and updates Prometheus metrics
func (pm *PrometheusJobMetrics) UpdateMetrics(ctx context.Context) error {
	// Get all batch IDs
	batchIDs, err := pm.store.GetAllBatchIDs(ctx)
	if err != nil {
		log.Printf("Error getting batch IDs: %v", err)
		return err
	}

	// For each batch, get all jobs and update metrics
	for _, batchID := range batchIDs {
		jobs, err := pm.store.GetBatchJobes(ctx, batchID)
		log.Print(jobs)
		if err != nil {
			log.Printf("Error getting jobs for batch %s: %v", batchID, err)
			continue
		}

		for _, job := range jobs {
			// Update gauge metrics
			if job.QueueDuration > 0 {
				pm.queueDuration.WithLabelValues(job.BatchID, job.JobID).Set(job.QueueDuration)
			}
			if job.ProcessingDuration > 0 {
				pm.processingDuration.WithLabelValues(job.BatchID, job.JobID).Set(job.ProcessingDuration)
			}
			if job.TotalDuration > 0 {
				pm.totalDuration.WithLabelValues(job.BatchID, job.JobID).Set(job.TotalDuration)
			}
			if job.JobSizeMB > 0 {
				pm.jobSize.WithLabelValues(job.BatchID, job.JobID).Set(job.JobSizeMB)
			}
			if job.JobQueueAheadLength >= 0 {
				pm.queueAheadLength.WithLabelValues(job.BatchID, job.JobID).Set(float64(job.JobQueueAheadLength))
			}

		}
	}

	return nil
}

// StartMetricsServer starts an HTTP server to expose Prometheus metrics
func StartMetricsServer(addr string, store *MetricsStore) error {
	registry := prometheus.NewRegistry()
	pm := NewPrometheusJobMetrics(store, registry)
	log.Printf("--------------- Starting /metrics Server ---------------")
	// Update metrics initially
	ctx := context.Background()
	if err := pm.UpdateMetrics(ctx); err != nil {
		log.Printf("Warning: initial metrics update failed: %v", err)
	}

	// Create HTTP handler that updates metrics on each scrape
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Update metrics before serving them
		if err := pm.UpdateMetrics(r.Context()); err != nil {
			log.Printf("Error updating metrics: %v", err)
		}
		//log.Printf(registry.Gather())
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})

	http.Handle("/metrics", handler)
	//http.Handle("/metrics", promhttp.Handler())

	log.Printf("Starting Prometheus metrics server on %s", addr)
	return http.ListenAndServe(addr, nil)
}
