package producer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalFileSource handles local filesystem operations
type LocalFileSource struct {
	InputDir     string
	OutputDir    string
	ProcessedDir string
}

func NewLocalFileSource(inputDir, outputDir, processedDir string) *LocalFileSource {
	return &LocalFileSource{
		InputDir:     inputDir,
		OutputDir:    outputDir,
		ProcessedDir: processedDir,
	}
}

func (l *LocalFileSource) ListFiles() ([]string, error) {
	entries, err := os.ReadDir(l.InputDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			filename := entry.Name()

			// Skip system/configuration files that shouldn't be processed as data files
			if l.isSystemFile(filename) {
				continue
			}

			files = append(files, filename)
		}
	}
	return files, nil
}

func (l *LocalFileSource) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.InputDir, filename))
}

// isPPXFFile checks if this is a pPXF input directory
func (l *LocalFileSource) isPPXFFile() bool {
	return strings.Contains(l.InputDir, "/ppxf/data/input")
}

// isFileInProcessList checks if a file is still in the pPXF processlist
func (l *LocalFileSource) isFileInProcessList(filename string) bool {
	processListPath := os.Getenv("PPXF_PROCESS_LIST")
	if processListPath == "" {
		processListPath = "/processing_data/ppxf/runtime/processlist.txt"
	}

	// Check if processlist file exists
	if _, err := os.Stat(processListPath); os.IsNotExist(err) {
		return false
	}

	// Read processlist file
	file, err := os.Open(processListPath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Check if filename is in the processlist
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == filename {
			return true
		}
	}

	return false
}

func (l *LocalFileSource) DeleteFile(filename string) error {
	// Special handling for pPXF files
	if l.isPPXFFile() && strings.HasSuffix(filename, ".fits") {
		// For pPXF files, only move to processed if they're NOT in the processlist anymore
		// (meaning pPXF has finished processing them)
		if l.isFileInProcessList(filename) {
			fmt.Printf("pPXF file %s still in processlist, skipping move to processed\n", filename)
			return nil
		}
		fmt.Printf("pPXF file %s no longer in processlist, moving to processed\n", filename)
	}

	sourcePath := filepath.Join(l.InputDir, filename)
	destPath := filepath.Join(l.ProcessedDir, filename)
	//log.Print(sourcePath, destPath)
	err := moveFile(sourcePath, destPath)
	if err != nil {
		fmt.Printf("Error moving file %s: %v\n", filename, err)
	}
	return nil
}

func moveFile(source, destination string) error {
	return os.Rename(source, destination)
}

// isSystemFile checks if a file is a system/configuration file that shouldn't be processed
func (l *LocalFileSource) isSystemFile(filename string) bool {
	systemFiles := []string{
		"mask.txt",  // pPXF mask file
		".gitkeep",  // Git keep files
		".DS_Store", // macOS system files
		"Thumbs.db", // Windows thumbnail cache
	}

	for _, sysFile := range systemFiles {
		if filename == sysFile {
			return true
		}
	}

	// Skip hidden files (starting with .)
	if strings.HasPrefix(filename, ".") {
		return true
	}

	return false
}
