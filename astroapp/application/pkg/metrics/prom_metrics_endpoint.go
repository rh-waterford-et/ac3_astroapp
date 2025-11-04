package metrics

import (
	"context"
	"fmt"
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
	completedJobs      *prometheus.GaugeVec
	totalCompletedJobs prometheus.Gauge
	podCount           *prometheus.GaugeVec
	runningPodCount    prometheus.Gauge
	store              *MetricsStore
	k8sClient          *K8sClient
}

// NewPrometheusJobMetrics creates and registers Prometheus metrics
func NewPrometheusJobMetrics(store *MetricsStore, registry *prometheus.Registry) *PrometheusJobMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	// Initialize Kubernetes client (optional, will log warning if it fails)
	k8sClient, err := NewK8sClient()
	if err != nil {
		log.Printf("Warning: Failed to initialize Kubernetes client for pod metrics: %v", err)
		log.Printf("Pod count metrics will not be available")
	} else {
		log.Printf("Successfully initialized Kubernetes client for pod metrics")
	}

	pm := &PrometheusJobMetrics{
		store:     store,
		k8sClient: k8sClient,
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
		completedJobs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_completed_jobs_per_batch",
				Help: "Number of completed jobs per batch",
			},
			[]string{"batch_id"},
		),
		totalCompletedJobs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "astroapp_completed_jobs_total",
				Help: "Total number of completed jobs across all batches",
			},
		),
		podCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "astroapp_pod_count",
				Help: "Number of pods by application label",
			},
			[]string{"app"},
		),
		runningPodCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "astroapp_running_pods_total",
				Help: "Total number of running pods in the namespace",
			},
		),
	}

	// Register metrics
	registry.MustRegister(pm.queueDuration)
	registry.MustRegister(pm.processingDuration)
	registry.MustRegister(pm.totalDuration)
	registry.MustRegister(pm.jobSize)
	registry.MustRegister(pm.queueAheadLength)
	registry.MustRegister(pm.completedJobs)
	registry.MustRegister(pm.totalCompletedJobs)

	// Register pod metrics only if k8s client is available
	if pm.k8sClient != nil {
		registry.MustRegister(pm.podCount)
		registry.MustRegister(pm.runningPodCount)
	}

	return pm
}

// UpdateMetrics fetches all metrics from the store and updates Prometheus metrics
func (pm *PrometheusJobMetrics) UpdateMetrics(ctx context.Context) error {
	// Update pod counts from Kubernetes (independent of Redis)
	if pm.k8sClient != nil {
		if err := pm.updatePodMetrics(ctx); err != nil {
			log.Printf("Warning: Failed to update pod metrics: %v", err)
			// Don't return error, continue with other metrics
		}
	}

	// Get all completed batch IDs (only from archived/completed metrics table)
	batchIDs, err := pm.store.GetAllCompletedBatchIDs(ctx)
	if err != nil {
		log.Printf("Error getting completed batch IDs: %v", err)
		return err
	}

	log.Printf("DEBUG: Found %d completed batch IDs: %v", len(batchIDs), batchIDs)

	totalCompletedCount := 0

	// For each batch, get completed jobs and update metrics
	for _, batchID := range batchIDs {
		jobs, err := pm.store.GetCompletedBatchJobes(ctx, batchID)
		if err != nil {
			log.Printf("Error getting completed jobs for batch %s: %v", batchID, err)
			continue
		}

		log.Printf("Updating Prometheus metrics for batch %s: %d completed jobs", batchID, len(jobs))

		// Update all individual job metrics
		for _, job := range jobs {
			// Update gauge metrics with actual values from completed jobs
			pm.queueDuration.WithLabelValues(job.BatchID, job.JobID).Set(job.QueueDuration)
			pm.processingDuration.WithLabelValues(job.BatchID, job.JobID).Set(job.ProcessingDuration)
			pm.totalDuration.WithLabelValues(job.BatchID, job.JobID).Set(job.TotalDuration)
			pm.jobSize.WithLabelValues(job.BatchID, job.JobID).Set(job.JobSizeMB)
			pm.queueAheadLength.WithLabelValues(job.BatchID, job.JobID).Set(float64(job.JobQueueAheadLength))
		}

		// Update completed jobs count for this batch
		// Since we're only reading from completed table, all jobs are complete
		completedCount := len(jobs)
		totalCompletedCount += completedCount
		log.Printf("Setting astroapp_completed_jobs_per_batch{batch_id=\"%s\"} = %d", batchID, completedCount)
		pm.completedJobs.WithLabelValues(batchID).Set(float64(completedCount))
	}

	// Update total completed jobs across all batches
	log.Printf("Setting astroapp_completed_jobs_total = %d", totalCompletedCount)
	pm.totalCompletedJobs.Set(float64(totalCompletedCount))

	return nil
}

// updatePodMetrics queries Kubernetes API and updates pod count metrics
func (pm *PrometheusJobMetrics) updatePodMetrics(ctx context.Context) error {
	// Get running pod count
	runningCount, err := pm.k8sClient.GetRunningPodCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running pod count: %w", err)
	}
	pm.runningPodCount.Set(float64(runningCount))

	// Get pod counts by app label
	podCounts, err := pm.k8sClient.GetPodCountsByApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pod counts by app: %w", err)
	}

	//log.Printf("Pod counts by app:")
	for app, count := range podCounts {
		pm.podCount.WithLabelValues(app).Set(float64(count))
		//log.Printf("  - %s: %d pods", app, count)
	}

	//log.Printf("Updated pod metrics: %d running pods total, %d apps tracked", runningCount, len(podCounts))
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
