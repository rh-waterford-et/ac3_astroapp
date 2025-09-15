package collector

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// RabbitMQCollector monitors RabbitMQ queue metrics via the prometheus exporter
type RabbitMQCollector struct {
	exporterURL string
	httpClient  *http.Client
	queueName   string
}

// NewRabbitMQCollector creates a new RabbitMQ metrics collector
func NewRabbitMQCollector(exporterURL, queueName string) *RabbitMQCollector {
	return &RabbitMQCollector{
		exporterURL: exporterURL,
		queueName:   queueName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetQueueDepth retrieves the current queue depth for the configured queue
func (r *RabbitMQCollector) GetQueueDepth() (int, error) {
	resp, err := r.httpClient.Get(r.exporterURL + "/metrics")
	if err != nil {
		return 0, fmt.Errorf("failed to fetch RabbitMQ metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read metrics response: %w", err)
	}

	return r.parseQueueDepth(string(body))
}

// parseQueueDepth extracts queue depth from Prometheus metrics format
func (r *RabbitMQCollector) parseQueueDepth(metricsText string) (int, error) {
	// Look for: rabbitmq_queue_messages{queue="producer_to_processor_queue"} 42
	pattern := fmt.Sprintf(`rabbitmq_queue_messages\{[^}]*queue="%s"[^}]*\}\s+(\d+)`, regexp.QuoteMeta(r.queueName))
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(metricsText)
	if len(matches) < 2 {
		// Try alternative patterns
		alternativePatterns := []string{
			fmt.Sprintf(`rabbitmq_queue_messages_total\{[^}]*queue="%s"[^}]*\}\s+(\d+)`, regexp.QuoteMeta(r.queueName)),
			fmt.Sprintf(`rabbitmq_queue_messages\{queue="%s"\}\s+(\d+)`, regexp.QuoteMeta(r.queueName)),
		}

		for _, altPattern := range alternativePatterns {
			altRe := regexp.MustCompile(altPattern)
			matches = altRe.FindStringSubmatch(metricsText)
			if len(matches) >= 2 {
				break
			}
		}

		if len(matches) < 2 {
			return 0, fmt.Errorf("queue depth metric not found for queue '%s'", r.queueName)
		}
	}

	depth, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("failed to parse queue depth value '%s': %w", matches[1], err)
	}

	return depth, nil
}

// TestConnection verifies connectivity to the RabbitMQ exporter
func (r *RabbitMQCollector) TestConnection() error {
	resp, err := r.httpClient.Get(r.exporterURL + "/metrics")
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ exporter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RabbitMQ exporter returned status %d", resp.StatusCode)
	}

	return nil
}

// GetAvailableQueues returns all queue names found in the metrics
func (r *RabbitMQCollector) GetAvailableQueues() ([]string, error) {
	resp, err := r.httpClient.Get(r.exporterURL + "/metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RabbitMQ metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}

	return r.parseQueueNames(string(body))
}

// parseQueueNames extracts all queue names from metrics
func (r *RabbitMQCollector) parseQueueNames(metricsText string) ([]string, error) {
	re := regexp.MustCompile(`rabbitmq_queue_messages[^{]*\{[^}]*queue="([^"]+)"[^}]*\}`)
	matches := re.FindAllStringSubmatch(metricsText, -1)

	queueSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 2 {
			queueSet[match[1]] = true
		}
	}

	var queues []string
	for queue := range queueSet {
		queues = append(queues, queue)
	}

	if len(queues) == 0 {
		return nil, fmt.Errorf("no queues found in metrics")
	}

	return queues, nil
}
