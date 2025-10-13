package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// UC3Client handles communication with the UC3 backend API
type UC3Client struct {
	baseURL string
	client  *http.Client
}

// ProcessDatasetRequest matches the UC3 API expected payload
type ProcessDatasetRequest struct {
	Dataset       string `json:"dataset"`
	ProcessorType string `json:"processorType"`
}

// ProcessDatasetResponse matches the UC3 API response
type ProcessDatasetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NewUC3Client creates a new UC3 API client
func NewUC3Client(baseURL string) *UC3Client {
	// Skip SSL verification for internal APIs
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &UC3Client{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: tr,
		},
	}
}

// TriggerProcessing triggers dataset processing via the UC3 API
func (c *UC3Client) TriggerProcessing(dataset, processorType string) error {
	url := fmt.Sprintf("%s/api/datasets/process", c.baseURL)

	payload := ProcessDatasetRequest{
		Dataset:       dataset,
		ProcessorType: processorType,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request to UC3 API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("UC3 API returned error status: %d", resp.StatusCode)
	}

	var response ProcessDatasetResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Optional debug output (can be controlled via environment variable)
	if os.Getenv("UC3_DEBUG") != "" {
		fmt.Printf("  UC3 API Response: success=%t, message=%s\n", response.Success, response.Message)
	}

	// UC3 API returns 200 OK for success, and the message indicates success
	// The Success field seems to be incorrectly set to false even for successful triggers
	if response.Message == "" {
		return fmt.Errorf("UC3 API returned empty response")
	}

	// Check if the message indicates an actual error vs successful trigger
	if response.Message != "" && !response.Success {
		// If message contains "Processing started", treat as success despite success=false
		if !strings.Contains(response.Message, "Processing started") {
			return fmt.Errorf("UC3 API error: %s", response.Message)
		}
	}

	return nil
}

// TestConnection tests connectivity to the UC3 API
func (c *UC3Client) TestConnection() error {
	// Try a simple request to test connectivity
	// Since there's no health endpoint, we'll just test the base URL
	resp, err := c.client.Get(c.baseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to UC3 API: %w", err)
	}
	defer resp.Body.Close()

	// Any response indicates the server is reachable
	return nil
}
