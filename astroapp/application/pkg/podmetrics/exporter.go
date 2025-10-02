package podmetrics

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PodMetricsExporter collects and exposes pod metrics
type PodMetricsExporter struct {
	clientset   *kubernetes.Clientset
	namespace   string
	metricsPort string

	// Prometheus metrics
	podCountTotal      *prometheus.GaugeVec
	podStatusCount     *prometheus.GaugeVec
	deploymentReplicas *prometheus.GaugeVec
	podAgeSeconds      *prometheus.GaugeVec
	podRestartCount    *prometheus.CounterVec
}

// NewPodMetricsExporter creates a new pod metrics exporter
func NewPodMetricsExporter() (*PodMetricsExporter, error) {
	// Create Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	exporter := &PodMetricsExporter{
		clientset:   clientset,
		namespace:   getEnvOrDefault("NAMESPACE", "uc3-applications"),
		metricsPort: getEnvOrDefault("METRICS_PORT", "8080"),
	}

	// Initialize Prometheus metrics
	exporter.initMetrics()

	return exporter, nil
}

func (pme *PodMetricsExporter) initMetrics() {
	// Pod count metric
	pme.podCountTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pod_count_total",
			Help: "Total number of pods",
		},
		[]string{"namespace", "status"},
	)

	// Pod status count metric
	pme.podStatusCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pod_status_count",
			Help: "Number of pods by status",
		},
		[]string{"namespace", "status"},
	)

	// Deployment replicas metric
	pme.deploymentReplicas = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "deployment_replicas",
			Help: "Number of replicas for deployments",
		},
		[]string{"namespace", "deployment"},
	)

	// Pod age metric
	pme.podAgeSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pod_age_seconds",
			Help: "Age of pods in seconds",
		},
		[]string{"namespace", "pod", "deployment"},
	)

	// Pod restart count metric
	pme.podRestartCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pod_restart_count_total",
			Help: "Total number of pod restarts",
		},
		[]string{"namespace", "pod", "container"},
	)

	// Register metrics
	prometheus.MustRegister(pme.podCountTotal)
	prometheus.MustRegister(pme.podStatusCount)
	prometheus.MustRegister(pme.deploymentReplicas)
	prometheus.MustRegister(pme.podAgeSeconds)
	prometheus.MustRegister(pme.podRestartCount)
}

// CollectMetrics collects metrics from Kubernetes API
func (pme *PodMetricsExporter) CollectMetrics(ctx context.Context) error {
	// Get pods
	pods, err := pme.clientset.CoreV1().Pods(pme.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Reset metrics
	pme.podCountTotal.Reset()
	pme.podStatusCount.Reset()
	pme.podAgeSeconds.Reset()

	// Count pods by status
	statusCount := make(map[string]int)
	for _, pod := range pods.Items {
		status := string(pod.Status.Phase)
		statusCount[status]++

		// Set pod age
		age := time.Since(pod.CreationTimestamp.Time).Seconds()
		deployment := pme.getDeploymentName(pod.Labels)
		pme.podAgeSeconds.WithLabelValues(pme.namespace, pod.Name, deployment).Set(age)

		// Count container restarts
		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 0 {
				pme.podRestartCount.WithLabelValues(
					pme.namespace,
					pod.Name,
					container.Name,
				).Add(float64(container.RestartCount))
			}
		}
	}

	// Set status counts
	for status, count := range statusCount {
		pme.podStatusCount.WithLabelValues(pme.namespace, status).Set(float64(count))
	}

	// Set total pod count
	totalPods := len(pods.Items)
	pme.podCountTotal.WithLabelValues(pme.namespace, "total").Set(float64(totalPods))

	// Get deployments
	deployments, err := pme.clientset.AppsV1().Deployments(pme.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	// Set deployment replicas
	for _, deployment := range deployments.Items {
		pme.deploymentReplicas.WithLabelValues(
			pme.namespace,
			deployment.Name,
		).Set(float64(*deployment.Spec.Replicas))
	}

	return nil
}

// getDeploymentName extracts deployment name from pod labels
func (pme *PodMetricsExporter) getDeploymentName(labels map[string]string) string {
	if deployment, exists := labels["app"]; exists {
		return deployment
	}
	return "unknown"
}

// Start starts the metrics exporter server
func (pme *PodMetricsExporter) Start(ctx context.Context) error {
	// Start metrics collection in background
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pme.CollectMetrics(ctx); err != nil {
					log.Printf("Error collecting metrics: %v", err)
				}
			}
		}
	}()

	// Start HTTP server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting pod metrics exporter on port %s", pme.metricsPort)
	return http.ListenAndServe(":"+pme.metricsPort, nil)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
