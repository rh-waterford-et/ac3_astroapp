package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
)

type VoronoiInterface interface {
	AddToProcessList(datacubeName string) error
	AddToProcessListWithDataset(datacubeName, datasetName string) error
}

type Voronoi struct {
	Utils common.UtilsInterface
}

func NewVoronoi(utils common.UtilsInterface) *Voronoi {
	return &Voronoi{
		Utils: utils,
	}
}

// AddToProcessList adds datacube to processlist ONLY if config also exists
// Returns an error if the processlist is not empty (ensuring one-at-a-time processing)
func (v *Voronoi) AddToProcessList(datacubeName string) error {
	// Get process list path - will already have POD_NAME expanded by Kubernetes
	processListPath := os.Getenv("PROCESS_LIST_VORONOI")
	if processListPath == "" {
		// Fallback: construct pod-specific path manually
		podName := os.Getenv("POD_NAME")
		if podName == "" {
			podName = "default"
		}
		processListPath = fmt.Sprintf("/processing_data/voronoi/runtime/processlist-%s.txt", podName)
	}

	log.Printf("Voronoi: Using processlist: %s", processListPath)

	// Ensure the runtime directory exists
	runtimeDir := filepath.Dir(processListPath)
	err := os.MkdirAll(runtimeDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating Voronoi runtime directory: %v", err)
	}

	// Check if processlist already has files - enforce one-at-a-time processing
	if !v.isProcessListEmpty(processListPath) {
		return fmt.Errorf("processlist not empty - deferring file until current job completes")
	}

	// Check if datacube file already exists in processlist to prevent duplicates
	if v.isFileInProcessList(datacubeName, processListPath) {
		log.Printf("Voronoi: DUPLICATE ATTEMPT - File already in process list, skipping: %s", datacubeName)
		return nil // Not an error, just skip
	}

	// CRITICAL: Check if BOTH datacube AND config exist before adding to processlist
	inputDir := os.Getenv("INPUT_DIR_VORONOI")
	if inputDir == "" {
		inputDir = "/processing_data/voronoi/data/input"
	}

	datacubePath := filepath.Join(inputDir, datacubeName)
	configPath := filepath.Join(inputDir, "voronoi_config.json")

	// Check datacube exists
	if _, err := os.Stat(datacubePath); os.IsNotExist(err) {
		log.Printf("Voronoi: Datacube not found yet: %s", datacubePath)
		return fmt.Errorf("datacube not found: %s", datacubePath)
	}

	// Check config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Voronoi: Config not found yet: %s (waiting for both files)", configPath)
		return fmt.Errorf("config not found: %s", configPath)
	}

	log.Printf("Voronoi: ✓ Found both datacube and config - ready to process")

	// Both files exist! Append the datacube filename to the process list
	file, err := os.OpenFile(processListPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening Voronoi process list file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(datacubeName + "\n")
	if err != nil {
		return fmt.Errorf("error adding file to Voronoi process list: %v", err)
	}

	log.Printf("Voronoi: ✓ SUCCESSFULLY ADDED to process list: %s", datacubeName)
	return nil
}

// AddToProcessListWithDataset adds datacube to processlist with dataset-specific config check
// Config file is stored as voronoi_config_{dataset}.json to support per-dataset configs
func (v *Voronoi) AddToProcessListWithDataset(datacubeName, datasetName string) error {
	// Get process list path - will already have POD_NAME expanded by Kubernetes
	processListPath := os.Getenv("PROCESS_LIST_VORONOI")
	if processListPath == "" {
		// Fallback: construct pod-specific path manually
		podName := os.Getenv("POD_NAME")
		if podName == "" {
			podName = "default"
		}
		processListPath = fmt.Sprintf("/processing_data/voronoi/runtime/processlist-%s.txt", podName)
	}

	log.Printf("Voronoi: Using processlist: %s", processListPath)

	// Ensure the runtime directory exists
	runtimeDir := filepath.Dir(processListPath)
	err := os.MkdirAll(runtimeDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating Voronoi runtime directory: %v", err)
	}

	// Check if processlist already has files - enforce one-at-a-time processing
	if !v.isProcessListEmpty(processListPath) {
		return fmt.Errorf("processlist not empty - deferring file until current job completes")
	}

	// Check if datacube file already exists in processlist to prevent duplicates
	if v.isFileInProcessList(datacubeName, processListPath) {
		log.Printf("Voronoi: DUPLICATE ATTEMPT - File already in process list, skipping: %s", datacubeName)
		return nil // Not an error, just skip
	}

	// CRITICAL: Check if BOTH datacube AND dataset-specific config exist before adding to processlist
	inputDir := os.Getenv("INPUT_DIR_VORONOI")
	if inputDir == "" {
		inputDir = "/processing_data/voronoi/data/input"
	}

	datacubePath := filepath.Join(inputDir, datacubeName)

	// Use dataset-specific config file name
	var configPath string
	if datasetName != "" {
		configPath = filepath.Join(inputDir, fmt.Sprintf("voronoi_config_%s.json", datasetName))
	} else {
		// Fallback to generic name if dataset not provided
		configPath = filepath.Join(inputDir, "voronoi_config.json")
	}

	// Check datacube exists
	if _, err := os.Stat(datacubePath); os.IsNotExist(err) {
		log.Printf("Voronoi: Datacube not found yet: %s", datacubePath)
		return fmt.Errorf("datacube not found: %s", datacubePath)
	}

	// Check dataset-specific config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Voronoi: Config not found yet: %s (waiting for both files, dataset: %s)", configPath, datasetName)
		return fmt.Errorf("config not found: %s", configPath)
	}

	log.Printf("Voronoi: ✓ Found both datacube and dataset-specific config (dataset: %s) - ready to process", datasetName)

	// Store dataset:filename in processlist so script knows which config to use
	processListEntry := datacubeName
	if datasetName != "" {
		processListEntry = fmt.Sprintf("%s:%s", datasetName, datacubeName)
	}

	// Both files exist! Append the entry to the process list
	file, err := os.OpenFile(processListPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening Voronoi process list file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(processListEntry + "\n")
	if err != nil {
		return fmt.Errorf("error adding file to Voronoi process list: %v", err)
	}

	log.Printf("Voronoi: ✓ SUCCESSFULLY ADDED to process list: %s (dataset: %s)", datacubeName, datasetName)
	return nil
}

// isProcessListEmpty checks if the processlist is empty (ensuring one-at-a-time processing)
func (v *Voronoi) isProcessListEmpty(processListPath string) bool {
	// If file doesn't exist, it's empty
	if _, err := os.Stat(processListPath); os.IsNotExist(err) {
		return true
	}

	// Read the processlist file
	content, err := os.ReadFile(processListPath)
	if err != nil {
		log.Printf("Error reading Voronoi process list: %v", err)
		return false // Assume not empty on error to be safe
	}

	// Check if there are any non-empty, non-comment lines
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return false // Found a file in the list
		}
	}
	return true // No files in list
}

// isFileInProcessList checks if a file is already in the processlist
func (v *Voronoi) isFileInProcessList(datacubeName string, processListPath string) bool {
	// If file doesn't exist, file is not in list
	if _, err := os.Stat(processListPath); os.IsNotExist(err) {
		return false
	}

	// Read the processlist file
	content, err := os.ReadFile(processListPath)
	if err != nil {
		log.Printf("Error reading Voronoi process list for duplicate check: %v", err)
		return false
	}

	// Check if filename exists in the list
	// Handle both formats: "dataset:filename" and just "filename"
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Check if line matches exactly, or if it's in "dataset:filename" format
		if trimmed == datacubeName {
			return true
		}
		// Check if line is "dataset:filename" format and filename matches
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) == datacubeName {
				return true
			}
		}
	}
	return false
}
