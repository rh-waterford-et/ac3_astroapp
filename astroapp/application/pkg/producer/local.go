package producer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalFileSource handles local filesystem operations
type LocalFileSource struct {
	InputDir     string
	ProcessedDir string
}

func NewLocalFileSource(inputDir, processedDir string) *LocalFileSource {
	return &LocalFileSource{
		InputDir: inputDir,

		ProcessedDir: processedDir,
	}
}

func (l *LocalFileSource) ListFiles() ([]string, error) {
	entries, err := os.ReadDir(l.InputDir)
	if err != nil {
		return nil, err
	}

	// Check if this is a pPXF output directory
	isPPXFOutput := strings.Contains(l.InputDir, "/ppxf/data/output")

	if isPPXFOutput {
		// For pPXF output, group files by their base input file and only return complete sets
		return l.listPPXFCompleteFiles()
	}

	// For non-pPXF files, use the original logic
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			filename := entry.Name()

			// Skip system/configuration files that shouldn't be processed as data files
			if l.isSystemFile(filename) {
				continue
			}

			// Only include files that are "stable" (older than 10 seconds) to prevent race conditions
			if l.isFileStable(filename) {
				files = append(files, filename)
			}
		}
	}
	return files, nil
}

// listPPXFCompleteFiles returns only complete sets of pPXF output files
func (l *LocalFileSource) listPPXFCompleteFiles() ([]string, error) {
	entries, err := os.ReadDir(l.InputDir)
	if err != nil {
		return nil, err
	}

	// Group files by their base input file name
	baseFiles := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			filename := entry.Name()

			// Skip system files
			if l.isSystemFile(filename) {
				continue
			}

			// Extract base filename from pPXF output file
			// e.g., "NGC7025_LR-V_final_cube_voronoi_cell_10_kinematics_and_stellar_pops_info.txt"
			// becomes "NGC7025_LR-V_final_cube_voronoi_cell_10"
			if strings.Contains(filename, "_kinematics_and_stellar_pops_info.txt") ||
				strings.Contains(filename, "_pPXF_fitting.pdf") ||
				strings.Contains(filename, "_residuals.fits") ||
				strings.Contains(filename, "_bestfit.fits") ||
				strings.Contains(filename, "_galaxy.fits") {

				var baseName string
				if strings.Contains(filename, "_kinematics_and_stellar_pops_info.txt") {
					baseName = strings.TrimSuffix(filename, "_kinematics_and_stellar_pops_info.txt")
				} else if strings.Contains(filename, "_pPXF_fitting.pdf") {
					baseName = strings.TrimSuffix(filename, "_pPXF_fitting.pdf")
				} else if strings.Contains(filename, "_residuals.fits") {
					baseName = strings.TrimSuffix(filename, "_residuals.fits")
				} else if strings.Contains(filename, "_bestfit.fits") {
					baseName = strings.TrimSuffix(filename, "_bestfit.fits")
				} else if strings.Contains(filename, "_galaxy.fits") {
					baseName = strings.TrimSuffix(filename, "_galaxy.fits")
				}

				baseFiles[baseName] = true
			}
		}
	}

	// Now check which base files have complete sets
	var completeFiles []string
	for baseName := range baseFiles {
		if l.isPPXFSetComplete(baseName + ".fits") { // Add .fits extension for the check
			// Add all 5 files for this complete set
			expectedFiles := []string{
				baseName + "_kinematics_and_stellar_pops_info.txt",
				baseName + "_pPXF_fitting.pdf",
				baseName + "_residuals.fits",
				baseName + "_bestfit.fits",
				baseName + "_galaxy.fits",
			}
			completeFiles = append(completeFiles, expectedFiles...)
		}
	}

	return completeFiles, nil
}

func (l *LocalFileSource) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.InputDir, filename))
}
func (l *LocalFileSource) GetBaseInputDir() string {
	return filepath.Base(l.InputDir)
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
		"mask.txt",           // pPXF mask file
		"ppxf_config.json",   // pPXF user configuration file
		".job_placeholder", // Old build remnant
		".gitkeep",           // Git keep files
		".DS_Store",          // macOS system files
		"Thumbs.db",          // Windows thumbnail cache
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

// isFileStable checks if a file has been stable (unmodified) for at least 15 seconds
// This prevents race conditions where the watcher picks up files before they're fully written
func (l *LocalFileSource) isFileStable(filename string) bool {
	filePath := filepath.Join(l.InputDir, filename)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	// File must be older than 15 seconds to be considered "stable"
	return time.Since(fileInfo.ModTime()) > 10*time.Second
}

// isPPXFSetComplete checks if all 5 expected pPXF output files exist for a given input file
func (l *LocalFileSource) isPPXFSetComplete(baseFilename string) bool {
	// Extract the base name without extension (e.g., "NGC7025_LR-V_final_cube_voronoi_cell_10")
	baseName := strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))

	expectedFiles := []string{
		baseName + "_kinematics_and_stellar_pops_info.txt",
		baseName + "_pPXF_fitting.pdf",
		baseName + "_residuals.fits",
		baseName + "_bestfit.fits",
		baseName + "_galaxy.fits",
	}

	for _, expectedFile := range expectedFiles {
		filePath := filepath.Join(l.InputDir, expectedFile)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return false
		}
		// Also check if this file is stable
		if !l.isFileStable(expectedFile) {
			return false
		}
	}

	return true
}
