package receiver

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// clearProcessList removes all entries from the process list file on startup
// This only runs once per deployment using a lock file mechanism
func (r *Receiver) ClearProcessList(processListPath string) {
	if processListPath == "" {
		log.Printf("│ ⚠ PROCESS_LIST environment variable not set, skipping cleanup")
		return
	}

	// Use a lock file to ensure cleanup only happens once per deployment
	lockFilePath := processListPath+"_PATH" + "/.cleanup_done"

	// Check if cleanup has already been done
	if _, err := os.Stat(lockFilePath); err == nil {
		log.Printf("│ ✓ Cleanup already performed (lock file exists), skipping")
		return
	}

	// Clear the process list file
	err := os.WriteFile(processListPath, []byte(""), 0644)
	if err != nil {
		log.Printf("│ ⚠ Failed to clear process list file %s: %v", processListPath, err)
		return
	}

	log.Printf("│ ✓ Cleared process list file: %s", processListPath)

	// Clean up old .in files from runtime directory
	inFileOutputPath := os.Getenv("IN_FILE_OUTPUT_PATH")
	if inFileOutputPath == "" {
		log.Printf("│ ⚠ IN_FILE_OUTPUT_PATH environment variable not set, skipping .in file cleanup")
		return
	}

	// Get runtime directory (parent of infiles directory)
	runtimeDir := filepath.Dir(inFileOutputPath)

	// Find all old .in files in the runtime directory
	pattern := filepath.Join(runtimeDir, "infilesgrid_example_*.in")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("│ ⚠ Failed to find old .in files: %v", err)
		return
	}

	if len(matches) > 0 {
		log.Printf("│ 🧹 Found %d old .in files to clean up", len(matches))
		for _, file := range matches {
			if err := os.Remove(file); err != nil {
				log.Printf("│ ⚠ Failed to remove old .in file %s: %v", file, err)
			} else {
				log.Printf("│ ✓ Removed old .in file: %s", filepath.Base(file))
			}
		}
	} else {
		log.Printf("│ ✓ No old .in files found to clean up")
	}

	// Create lock file to indicate cleanup is complete
	err = os.WriteFile(lockFilePath, []byte("cleanup completed"), 0644)
	if err != nil {
		log.Printf("│ ⚠ Failed to create cleanup lock file: %v", err)
	} else {
		log.Printf("│ ✓ Created cleanup lock file: %s", lockFilePath)
	}
}
func (r *Receiver) CreateBatchInfoFileStarlight(appName, batchID, jobID, filenamesHeader string) error {

	filePath := filepath.Join(os.Getenv("BATCH_INFO_DIR"), jobID+".txt")

	filenames := strings.Split(filenamesHeader, ",")
	var cleanFilenames []string
	for _, f := range filenames {

		cleanName := filepath.Base(f)

		if strings.HasSuffix(cleanName, ".in") {
			continue
		}

		cleanFilenames = append(cleanFilenames, "output_"+cleanName)
	}

	content := fmt.Sprintf("%s/data/output/\n%s\n%s",
		strings.ToLower(appName),
		batchID,
		strings.Join(cleanFilenames, ", "))

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write batch info file: %v", err)
	}

	log.Printf("│ ✓ Created batch info file: %s", filePath)
	return nil
}
func (r *Receiver) CreateBatchInfoFilePPXF(appName, batchID, jobID, filenamesHeader string) error {
	filePath := filepath.Join(os.Getenv("BATCH_INFO_DIR"), jobID+".in")

	expectedSuffixes := []string{
		"_kinematics_and_stellar_pops_info.txt",
		"_pPXF_fitting.pdf",
		"_residuals.fits",
		"_bestfit.fits",
		"_galaxy.fits",
	}

	pathLine := fmt.Sprintf("%s/output/run_%s", strings.ToLower(appName), batchID)
	batchLine := batchID

	filenames := make([]string, 0, len(expectedSuffixes))
	for _, suffix := range expectedSuffixes {
		filenames = append(filenames, filenamesHeader+suffix)
	}
	filesLine := strings.Join(filenames, ",")

	content := strings.Join([]string{pathLine, batchLine, filesLine}, "\n")

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write batch info file: %v", err)
	}

	log.Printf("│ ✓ Created batch info file: %s", filePath)
	return nil
}
func (r *Receiver) CheckProcessLists() (bool, error) {

	processLists := []string{os.Getenv("PROCESS_LIST_STARLIGHT"), os.Getenv("PROCESS_LIST_PPXF")}

	allEmpty := true
	for _, processList := range processLists {

		entries, err := r.GetProcessListEntries(processList)
		if err != nil {
			return false, err
		}

		if len(entries) > 0 {
			//log.Printf("Process list %s has %d entries", processList, len(entries))
			allEmpty = false
		}
	}
	return allEmpty, nil

}
func (r *Receiver) GetProcessListEntries(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var entries []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			entries = append(entries, trimmed)
		}
	}

	return entries, nil
}
