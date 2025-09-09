package receiver

import (
	"encoding/json"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
)

func (r *Receiver) logBatchStart(batchN, appName, batchID, jobID string) {
	log.Printf("\n■■■ JOB START [%s] ■■■", batchN)
	log.Printf("│ App:        %s", appName)
	log.Printf("│ Batch ID:   %s", batchID)
	log.Printf("│ Job ID:   %s", jobID)
}

func (r *Receiver) requeueWithLog(d amqp.Delivery, jobID string) {
	err := d.Nack(false, true)
	if err != nil {
		log.Printf("│ ERROR nack: %v", err)
	}
	log.Printf("■■■ BATCH ERROR [%s] - Message requeued ■■■", jobID)
}

// extractBatchNameFromFilename extracts the batch name from output filenames
// STARLIGHT: "output_NGC7025_LR-V_final_cube_voronoi_cell_0.txt" -> "NGC7025"
// pPXF: "NGC7025_LR-V_final_cube_voronoi_cell_0_kinematics_and_stellar_pops_info.txt" -> "NGC7025"
func (r *Receiver) extractBatchNameFromFilename(filename string) string {
	// Remove file extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	//log.Printf("│ DEBUG: Extracting batch name from filename: %s (without extension: %s)", filename, name)

	// Check if it's a STARLIGHT output file pattern
	if strings.HasPrefix(name, "output_") && strings.Contains(name, "_LR-V") {
		// Extract the part between "output_" and "_LR-V"
		parts := strings.Split(name, "_LR-V")
		if len(parts) > 0 {
			prefixPart := parts[0]
			if strings.HasPrefix(prefixPart, "output_") {
				batchName := strings.TrimPrefix(prefixPart, "output_")
				//log.Printf("│ DEBUG: STARLIGHT batch name extracted: %s", batchName)
				return batchName
			}
		}
	}

	// Check if it's a pPXF output file pattern
	// pPXF files start with batch name: "NGC7025_LR-V_final_cube_voronoi_cell_0_..."
	if strings.Contains(name, "_LR-V_final_cube_voronoi_cell_") {
		// Extract the part before "_LR-V"
		parts := strings.Split(name, "_LR-V")
		if len(parts) > 0 && parts[0] != "" {
			batchName := parts[0]
			log.Printf("│ DEBUG: pPXF batch name extracted: %s", batchName)
			return batchName
		}
	}

	// More generic pattern for pPXF files - look for anything before "_LR-V" or "_LR-R"
	if strings.Contains(name, "_LR-V") || strings.Contains(name, "_LR-R") {
		var separator string
		if strings.Contains(name, "_LR-V") {
			separator = "_LR-V"
		} else {
			separator = "_LR-R"
		}

		parts := strings.Split(name, separator)
		if len(parts) > 0 && parts[0] != "" {
			batchName := parts[0]
			log.Printf("│ DEBUG: Generic pPXF batch name extracted: %s", batchName)
			return batchName
		}
	}

	// For other file patterns, try to extract from different patterns
	// This could be extended for STECKMAP if needed

	log.Printf("│ DEBUG: No batch name could be extracted from filename: %s", filename)
	return "" // Return empty if no batch name can be extracted
}

// extractCellNumberFromFilename extracts cell number from PPXF output files
// Example: "NGC7025_LR-V_final_cube_voronoi_cell_0_bestfit.fits" -> "0"
// Example: "NGC7025_LR-V_final_cube_voronoi_cell_12_galaxy.fits" -> "12"
func (r *Receiver) extractCellNumberFromFilename(filename string) string {
	// Regex pattern to match "cell_" followed by digits
	cellPattern := regexp.MustCompile(`cell_(\d+)`)
	matches := cellPattern.FindStringSubmatch(filename)

	if len(matches) >= 2 {
		cellNumber := matches[1]
		log.Printf("│ DEBUG: Extracted cell number '%s' from filename: %s", cellNumber, filename)
		return cellNumber
	}

	log.Printf("│ DEBUG: No cell number found in filename: %s", filename)
	return ""
}

// calculateJobSizeMB calculates the total size of all files in MB
func (r *Receiver) calculateJobSizeMB(messageBody []byte, filenames []string) float64 {
	var msgBody api.MessageBody
	err := json.Unmarshal(messageBody, &msgBody)
	if err != nil {
		log.Printf("│ ERROR: Failed to parse message body for size calculation: %v", err)
		return 0
	}

	totalSize, totalfiles := 0, 0
	for _, file := range msgBody.Files {
		if strings.Contains(file.Name, ".in") {
			continue
		}
		totalSize += len(file.Content)
		totalfiles++
	}

	// Convert bytes to MB
	sizeMB := float64(totalSize) / (1024 * 1024)
	log.Printf("│ DEBUG: Calculated job size: %.2f MB (%d bytes across %d files)",
		sizeMB, totalSize, totalfiles)

	return sizeMB
}

// calculateBinaryJobSizeMB calculates the total size of binary files in MB
func (r *Receiver) calculateBinaryJobSizeMB(messageBody []byte, filenames []string) float64 {
	var binaryMsgBody api.BinaryMessageBody
	err := json.Unmarshal(messageBody, &binaryMsgBody)
	if err != nil {
		log.Printf("│ ERROR: Failed to parse binary message body for size calculation: %v", err)
		return 0
	}

	totalSize := 0
	for _, file := range binaryMsgBody.Files {
		totalSize += len(file.Content)
	}

	// Convert bytes to MB
	sizeMB := float64(totalSize) / (1024 * 1024)
	log.Printf("│ DEBUG: Calculated binary job size: %.2f MB (%d bytes across %d files)",
		sizeMB, totalSize, len(binaryMsgBody.Files))

	return sizeMB
}
