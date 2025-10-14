package receiver

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
)

func (r *Receiver) handleApplicationProcessing(side, appName, jobID string, successCount int, batchSize int32, inFileName, inFileContent string, spectrumFiles []api.DataFile) {
	starlight := app.NewStarlight([]api.DataFile{}, r.Utils)

	if successCount == int(batchSize) && appName == "STARLIGHT" && side == "processor" {
		starlight.UpdateToProcessList(inFileName, []byte(inFileContent))
		r.ProcessingMessage = false
		log.Printf("│ ✓ Added .in file from input side to processlist: %s", inFileName)
		r.updateProgress("STARLIGHT", jobID, api.StageAnalysis, 70.0)

	}
	if successCount == int(batchSize) && appName == "PPXF" && side == "processor" {
		log.Printf("│ ✓ All pPXF files processed and added to process list")
		r.updateProgress(appName, jobID, api.StageAnalysis, 70.0)
	}
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
			outputPath = os.Getenv("INPUT_DIR_PPXF")
		case "STECKMAP":
			outputPath = os.Getenv("EXPLORED_DIR_STECKMAP")
		default:
			log.Printf("│ ERROR: Unknown app: %s", appName)
			return ""
		}
	}
	return outputPath
}

// updateProgress sends a progress update to the API server
func (r *Receiver) updateProgress(appName, jobID string, stage api.PipelineStage, progress float64) {
	request := api.ProgressUpdateRequest{
		DatasetID:   jobID,
		DatasetName: appName,
		Stage:       stage,
		Progress:    progress,
	}

	// Send progress update to local API server
	go func() {
		jsonData, err := json.Marshal(request)
		if err != nil {
			log.Printf("Error marshaling progress update: %v", err)
			return
		}

		apiURL := "http://localhost:8080/api/progress/update"
		if serverURL := os.Getenv("API_SERVER_URL"); serverURL != "" {
			apiURL = serverURL + "/api/progress/update"
		}

		resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error sending progress update: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Progress update returned non-200 status: %d", resp.StatusCode)
		}
	}()
}
