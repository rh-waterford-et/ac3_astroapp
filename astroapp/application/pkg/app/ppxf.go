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
	AddToProcessList(fileName string)
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
	p.AddToProcessList(file.Name)

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
func (p *PPXF) AddToProcessList(fileName string) {
	processListPath := os.Getenv("PPXF_PROCESS_LIST")
	if processListPath == "" {
		processListPath = "/processing_data/ppxf/runtime/processlist.txt"
	}

	// Ensure the runtime directory exists
	runtimeDir := filepath.Dir(processListPath)
	err := os.MkdirAll(runtimeDir, 0755)
	if err != nil {
		log.Printf("Error creating pPXF runtime directory: %v", err)
		return
	}

	// Check if file already exists in processlist to prevent duplicates
	if p.isFileInProcessList(fileName, processListPath) {
		log.Printf("pPXF: DUPLICATE ATTEMPT - File already in process list, skipping: %s", fileName)
		return
	}

	// Append the filename to the process list
	file, err := os.OpenFile(processListPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error opening pPXF process list file: %v", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(fileName + "\n")
	if err != nil {
		log.Printf("Error adding file to pPXF process list: %v", err)
	} else {
		log.Printf("pPXF: ✓ SUCCESSFULLY ADDED to process list: %s", fileName)
	}
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
