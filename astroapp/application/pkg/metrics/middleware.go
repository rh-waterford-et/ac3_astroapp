package metrics

import (
	"net/http"
	"strconv"
	"time"


)

// PrometheusMiddleware wraps HTTP handlers with Prometheus metrics
type PrometheusMiddleware struct {
	metrics *PrometheusMetrics
}

// NewPrometheusMiddleware creates a new Prometheus middleware
func NewPrometheusMiddleware(metrics *PrometheusMetrics) *PrometheusMiddleware {
	return &PrometheusMiddleware{
		metrics: metrics,
	}
}

// WrapHandler wraps an HTTP handler with Prometheus metrics
func (pm *PrometheusMiddleware) WrapHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Call the original handler
		handler(wrapped, r)
		
		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(wrapped.statusCode)
		
		pm.metrics.RecordHTTPRequest(r.Method, r.URL.Path, statusCode, duration)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// InstrumentHandler instruments an HTTP handler with Prometheus metrics
func InstrumentHandler(handler http.HandlerFunc, metrics *PrometheusMetrics) http.HandlerFunc {
	middleware := NewPrometheusMiddleware(metrics)
	return middleware.WrapHandler(handler)
}
