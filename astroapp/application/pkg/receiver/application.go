package receiver

import (
	"log"
	"os"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
)

func (r *Receiver) handleApplicationProcessing(side, appName, jobID string, successCount int, batchSize int32, inFileName, inFileContent string, spectrumFiles []api.DataFile) {
	starlight := app.NewStarlight([]api.DataFile{}, r.Utils)

	if successCount == int(batchSize) && appName == "STARLIGHT" && side == "processor" {
		if len(spectrumFiles) > 0 && inFileName == "" {
			r.handleStarlightGeneratedInFile(starlight, spectrumFiles, jobID)
		} else if inFileName != "" {
			r.handleStarlightReceivedInFile(starlight, inFileName, inFileContent, jobID)
		}
	}

	if successCount == int(batchSize) && appName == "PPXF" && side == "processor" {
		log.Printf("│ ✓ All pPXF files processed and added to process list")
		r.updateProgress(appName, jobID, api.StageAnalysis, 70.0)
	}
}

func (r *Receiver) handleStarlightGeneratedInFile(starlight *app.Starlight, spectrumFiles []api.DataFile, jobID string) {
	newInFileName, newInContent := starlight.UpdateInFile(spectrumFiles)
	if newInFileName != "" && newInContent != "" {
		starlight.UpdateToProcessList(newInFileName, []byte(newInContent))
		log.Printf("│ ✓ Generated and added .in file to processlist: %s with %d spectrum files from current batch",
			newInFileName, len(spectrumFiles))
		r.updateProgress("STARLIGHT", jobID, api.StageAnalysis, 70.0)
	} else {
		log.Printf("│ ⚠ Failed to generate .in file content")
	}
}

func (r *Receiver) handleStarlightReceivedInFile(starlight *app.Starlight, inFileName, inFileContent, jobID string) {
	starlight.UpdateToProcessList(inFileName, []byte(inFileContent))
	log.Printf("│ ✓ Added .in file from input side to processlist: %s", inFileName)
	r.updateProgress("STARLIGHT", jobID, api.StageAnalysis, 70.0)
}

func (r *Receiver) getOutputPath(side, appName string) string {
	var outputPath string
	if side == "producer" {
		switch appName {
		case "STARLIGHT":
			outputPath = os.Getenv("OUTPUT_BUCKET_STARLIGHT")
		case "PPXF":
			outputPath = os.Getenv("OUTPUT_BUCKET_PPXF")
		case "STECKMAP":
			outputPath = os.Getenv("OUTPUT_BUCKET_STECKMAP")
		default:
			log.Printf("│ ERROR: Unknown app: %s", appName)
			return ""
		}
	} else {
		switch appName {
		case "STARLIGHT":
			outputPath = os.Getenv("INPUT_DIR_STARLIGHT")
		case "PPXF":
			outputPath = os.Getenv("EXPLORED_DIR_PPXF")
		case "STECKMAP":
			outputPath = os.Getenv("EXPLORED_DIR_STECKMAP")
		default:
			log.Printf("│ ERROR: Unknown app: %s", appName)
			return ""
		}
	}
	return outputPath
}
