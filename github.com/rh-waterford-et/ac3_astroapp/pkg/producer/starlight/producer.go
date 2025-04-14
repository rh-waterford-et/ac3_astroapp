package starlight

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/producer/common"
)

type StarlightProducer struct {
	*common.BaseProducer
}

func NewStarlightProducer(batchSize int, inputDir, outputDir string, eventQueue chan common.Event) *StarlightProducer {
	base := common.NewBaseProducer(batchSize, inputDir, outputDir, eventQueue)
	return &StarlightProducer{BaseProducer: base}
}

func (p *StarlightProducer) SendBatch() {
	if p.BaseProducer == nil || len(p.Batch) == 0 {
		return
	}

	// Starlight-specific .in file handling
	inFileName, content := p.updateInFile()
	if inFileName != "" && content != "" {
		p.Batch = append(p.Batch, common.DataFile{Name: inFileName, Content: content})
	}

	// Call base SendBatch
	p.BaseProducer.SendBatch()

	// Remove .in file after sending
	p.removeInFileFromBatch()
}

func (p *StarlightProducer) updateInFile() (string, string) {
	templateInFilePath := os.Getenv("TEMPLATE_IN_FILE_PATH")
	inFileOutputPath := os.Getenv("IN_FILE_OUTPUT_PATH")
	newInFileName := fmt.Sprintf("grid_example_%d.in", rand.Intn(100))

	// Check if the template .in file exists
	if _, err := os.Stat(templateInFilePath); os.IsNotExist(err) {
		log.Println("Error: template .in file does not exist")
		return "", ""
	}

	f, err := os.Open(templateInFilePath)
	if err != nil {
		log.Println("Error opening template file:", err)
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	i := 0
	var newFile strings.Builder
	for scanner.Scan() {
		i++
		if i == 16 {
			// Replace the input file name in the .in file
			res := strings.Split(scanner.Text(), "  ")
			for j := 0; j < len(p.Batch); j++ {
				res[0] = p.Batch[j].Name

				// Get kinematic values for the current file
				kinematicValues, err := p.getKinematicValues(p.Batch[j].Name)
				if err != nil {
					log.Printf("Error getting kinematic values for file %s: %v", p.Batch[j].Name, err)
					continue
				}
				res[4] = kinematicValues // Update the 4th and 5th parameters with Velocity and Sigma
				res[5] = "output_" + p.Batch[j].Name
				overwriteString := strings.Join(res, "  ")
				newFile.WriteString(overwriteString + "\n")
			}
		} else {
			newFile.WriteString(scanner.Text() + "\n")
		}
	}

	// Write the updated .in file
	outputPath := filepath.Join(inFileOutputPath, newInFileName)
	err = os.WriteFile(outputPath, []byte(newFile.String()), 0644)
	if err != nil {
		log.Println("Error writing .in file:", err)
		return "", ""
	}

	// Read the content of the new .in file
	content, err := os.ReadFile(outputPath)
	if err != nil {
		log.Println("Error reading the newly created .in file:", err)
		return "", ""
	}

	return newInFileName, string(content)
}

func (p *StarlightProducer) removeInFileFromBatch() {
	filteredBatch := make([]common.DataFile, 0, len(p.Batch))

	for _, file := range p.Batch {
		if !strings.HasSuffix(file.Name, ".in") {
			filteredBatch = append(filteredBatch, file)
		} else {
			inFilePath := filepath.Join("/processing_data/starlight/runtime/infiles/", file.Name)
			err := os.Remove(inFilePath)
			if err != nil {
				log.Printf("Error removing .in file %s: %v\n", inFilePath, err)
			} else {
				log.Printf("Successfully removed .in file: %s\n", inFilePath)
			}
		}
	}

	p.Batch = filteredBatch
}

func (p *StarlightProducer) getKinematicValues(fileName string) (string, error) {
	kinematicFilePath := "./data/kinematic_information_file_NGC7025_LR-V.txt"
	file, err := os.Open(kinematicFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open kinematic file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == fileName {
			velocity := fields[1]
			sigma := fields[3]
			return fmt.Sprintf("%s %s", velocity, sigma), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading kinematic file: %v", err)
	}

	return "", fmt.Errorf("file %s not found in kinematic information", fileName)
}
