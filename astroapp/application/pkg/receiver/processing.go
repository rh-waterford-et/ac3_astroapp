package receiver

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/app"
)

func (r *Receiver) processFiles(msgBody api.MessageBody, side, appName, outputPath string) (int, string, string, []api.DataFile) {
	successCount := 0
	var inFileName string
	var inFileContent string
	var spectrumFiles []api.DataFile

	// starlight := app.NewStarlight([]api.DataFile{}, r.Utils)
	ppxf := app.NewPPXF(r.Utils)

	for _, file := range msgBody.Files {
		if strings.HasPrefix(filepath.Base(file.Name), ".") {
			log.Printf("│ ⚠ Skipping hidden file: %s", file.Name)
			successCount++
			continue
		}

		if strings.HasSuffix(file.Name, ".in") {
			inFileName = file.Name
			inFileContent = string(file.Content)
			log.Printf("│ ✓ Found .in file: %s", file.Name)
			successCount++
			continue
		}

		if !strings.HasSuffix(file.Name, ".in") && !strings.HasPrefix(filepath.Base(file.Name), ".") {
			spectrumFiles = append(spectrumFiles, file)
		}

		if side == "producer" {
			successCount += r.handleProducerFile(file, outputPath)
		} else {
			successCount += r.handleProcessorFile(file, outputPath, appName, ppxf)
		}
	}

	return successCount, inFileName, inFileContent, spectrumFiles
}

func (r *Receiver) handleProducerFile(file api.DataFile, outputPath string) int {
	batchName := r.extractBatchNameFromFilename(file.Name)
	var uploadPath string

	if batchName != "" {
		uploadPath = filepath.Join(outputPath, batchName, file.Name)
	} else {
		uploadPath = filepath.Join(outputPath, file.Name)
	}

	folderPath := filepath.Dir(uploadPath)
	fileName := filepath.Base(uploadPath)

	if folderPath == "." {
		folderPath = ""
	}

	//log.Printf("│ DEBUG: S3 upload - folderPath: '%s', fileName: '%s'", folderPath, fileName)

	err := r.Bucket.UploadFileToBucket(folderPath, fileName, []byte(file.Content))
	if err != nil {
		log.Printf("│ ✗ Error uploading file %s to bucket: %v", uploadPath, err)
		return 0
	}

	log.Printf("│ ✓ Uploaded file to bucket: %s", uploadPath)
	return 1
}

func (r *Receiver) handleProcessorFile(file api.DataFile, outputPath, appName string, ppxf *app.PPXF) int {
	filename := filepath.Base(file.Name)
	filePath := filepath.Join(outputPath, filename)

	err := os.WriteFile(filePath, []byte(file.Content), 0644)
	if err != nil {
		log.Printf("│ ✗ Error writing file %s: %v", filePath, err)
		return 0
	}

	log.Printf("│ ✓ Wrote file: %s to %s", file.Name, filePath)

	if appName == "PPXF" && strings.HasSuffix(file.Name, ".fits") {
		ppxf.AddToProcessList(filename)
		log.Printf("│ ✓ Added pPXF file to process list: %s", filename)
	}

	return 1
}
