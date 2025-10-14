package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
)

type PPXFInterface interface {
	ProcessFile(file api.DataFile) error
	GetOutputFiles(inputFileName string) ([]string, error)
	AddToProcessList(fileName string) error
}

type PPXF struct {
	Utils common.UtilsInterface
}

func NewPPXF(utils common.UtilsInterface) *PPXF {
	return &PPXF{
		Utils: utils,
	}
}

// ProcessFile handles individual .fits file processing for pPXF
func (p *PPXF) ProcessFile(file api.DataFile) error {
	inputDir := os.Getenv("INPUT_DIR_PPXF")
	if inputDir == "" {
		inputDir = "/processing_data/ppxf/data/input"
	}

	// Create the full path for the input file
	inputPath := filepath.Join(inputDir, file.Name)

	// Ensure input directory exists
	err := os.MkdirAll(inputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create pPXF input directory: %v", err)
	}

	// Write the file to the input directory
	err = os.WriteFile(inputPath, []byte(file.Content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write pPXF input file %s: %v", file.Name, err)
	}

	log.Printf("pPXF: Written input file: %s", inputPath)

	// Add to process list for the pPXF container to pick up
	err = p.AddToProcessList(file.Name)
	if err != nil {
		return fmt.Errorf("processlist not empty, deferring: %v", err)
	}

	return nil
}

// GetOutputFiles returns expected output files for a given input file
func (p *PPXF) GetOutputFiles(inputFileName string) ([]string, error) {
	outputDir := os.Getenv("OUTPUT_DIR_PPXF")
	if outputDir == "" {
		outputDir = "/processing_data/ppxf/data/output"
	}

	// Remove .fits extension to get base name
	baseName := strings.TrimSuffix(inputFileName, ".fits")

	// pPXF generates multiple output files per input file
	expectedFiles := []string{
		fmt.Sprintf("%s_kinematics_and_stellar_pops_info.txt", baseName),
		fmt.Sprintf("%s_pPXF_fitting.pdf", baseName),
		fmt.Sprintf("%s_residuals.fits", baseName),
		fmt.Sprintf("%s_bestfit.fits", baseName),
		fmt.Sprintf("%s_galaxy.fits", baseName),
	}

	var outputFiles []string
	for _, expectedFile := range expectedFiles {
		outputPath := filepath.Join(outputDir, expectedFile)
		if _, err := os.Stat(outputPath); err == nil {
			outputFiles = append(outputFiles, outputPath)
		}
	}

	if len(outputFiles) == 0 {
		return nil, fmt.Errorf("no output files found for input file: %s", inputFileName)
	}

	return outputFiles, nil
}

// AddToProcessList adds a file to the pPXF processing list
// Returns an error if the processlist is not empty (ensuring one-at-a-time processing)
func (p *PPXF) AddToProcessList(fileName string) error {
	// Get process list path - will already have POD_NAME expanded by Kubernetes
	processListPath := os.Getenv("PROCESS_LIST_PPXF")
	if processListPath == "" {
		// Fallback: construct pod-specific path manually
		podName := os.Getenv("POD_NAME")
		if podName == "" {
			podName = "default"
		}
		processListPath = fmt.Sprintf("/processing_data/ppxf/runtime/processlist-%s.txt", podName)
	}

	log.Printf("pPXF: Using processlist: %s", processListPath)

	// Ensure the runtime directory exists
	runtimeDir := filepath.Dir(processListPath)
	err := os.MkdirAll(runtimeDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating pPXF runtime directory: %v", err)
	}

	// Check if processlist already has files - enforce one-at-a-time processing
	if !p.isProcessListEmpty(processListPath) {
		return fmt.Errorf("processlist not empty - deferring file until current job completes")
	}

	// Check if file already exists in processlist to prevent duplicates
	if p.isFileInProcessList(fileName, processListPath) {
		log.Printf("pPXF: DUPLICATE ATTEMPT - File already in process list, skipping: %s", fileName)
		return nil // Not an error, just skip
	}

	// Append the filename to the process list
	file, err := os.OpenFile(processListPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening pPXF process list file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(fileName + "\n")
	if err != nil {
		return fmt.Errorf("error adding file to pPXF process list: %v", err)
	}

	log.Printf("pPXF: ✓ SUCCESSFULLY ADDED to process list: %s", fileName)
	return nil
}

// isProcessListEmpty checks if the processlist is empty (ensuring one-at-a-time processing)
func (p *PPXF) isProcessListEmpty(processListPath string) bool {
	// If file doesn't exist, it's empty
	if _, err := os.Stat(processListPath); os.IsNotExist(err) {
		return true
	}

	// Read the processlist file
	content, err := os.ReadFile(processListPath)
	if err != nil {
		log.Printf("Error reading pPXF process list: %v", err)
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
func (p *PPXF) isFileInProcessList(fileName string, processListPath string) bool {
	// If file doesn't exist, file is not in list
	if _, err := os.Stat(processListPath); os.IsNotExist(err) {
		return false
	}

	// Read the processlist file
	content, err := os.ReadFile(processListPath)
	if err != nil {
		log.Printf("Error reading pPXF process list for duplicate check: %v", err)
		return false
	}

	// Check if filename exists in the list
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == fileName {
			return true
		}
	}
	return false
}
