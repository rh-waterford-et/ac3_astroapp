package app

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/api"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
)

type StarlightInterface interface {
	UpdateInFile(batch []api.DataFile) (string, string)
	GetKinematicValues(fileName string) (string, error)
	RemoveInFileFromBatch(batch []api.DataFile) []api.DataFile
	UpdateToProcessList(inFileName string, fileContent []byte)
}

type Starlight struct {
	Utils common.UtilsInterface
}

func NewStarlight(batch []api.DataFile, utils common.UtilsInterface) *Starlight {
	return &Starlight{
		Utils: utils,
	}
}

func (s *Starlight) UpdateInFile(batch []api.DataFile) (string, string) {
	templateInFilePath := os.Getenv("TEMPLATE_IN_FILE_PATH")
	inFileOutputPath := os.Getenv("IN_FILE_OUTPUT_PATH")
	// #nosec G404
	newInFileName := fmt.Sprintf("grid_example_%d.in", rand.Intn(100))

	// Check if the template .in file exists
	if exists, _ := s.Utils.Exists(templateInFilePath); !exists {
		println("Error: file does not exist")
		return "", ""
	}

	f, err := os.Open(templateInFilePath)
	if err != nil {
		println("Error opening file")
		panic(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	i := 0
	var newFile string
	println("start scaner")
	for scanner.Scan() {
		i++
		println("%v", i)
		if i == 16 {
			// Replace the input file name in the .in file
			res := strings.Split(scanner.Text(), "  ")
			for j := 0; j < len(batch); j++ {
				res[0] = batch[j].Name
				// Get kinematic values for the current file
				kinematicValues, err := s.GetKinematicValues(batch[j].Name)
				if err != nil {
					log.Printf("Error getting kinematic values for file %s: %v", batch[j].Name, err)
					continue
				}
				res[4] = "CAL " + kinematicValues // Update the 4th and 5th parameters with Velocity and Sigma
				res[5] = "output_" + batch[j].Name
				overwrite_string := strings.Join(res, "  ")
				newFile = newFile + overwrite_string + "\n"
			}
		} else {
			newFile = newFile + scanner.Text() + "\n"
		}
	}

	// Write the updated .in file to the output directory
	// #nosec G306
	err = os.WriteFile(inFileOutputPath+newInFileName, []byte(newFile), 0644)
	if err != nil {
		println("Error writing .in file: ", err.Error())
		return "", ""
	}

	// Read the content of the new .in file
	content, err := os.ReadFile(inFileOutputPath + newInFileName)
	if err != nil {
		println("Error reading the newly created .in file:", err.Error())
		return "", ""
	}

	return newInFileName, string(content)
}

func (s *Starlight) GetKinematicValues(fileName string) (string, error) {
	kinematicFilePath := "/docker/starlight/config_files_starlight/kinematic_information_file_NGC7025_LR-V.txt"
	file, err := os.Open(kinematicFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open kinematic file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if fields[0] == fileName {
			velocity := fields[1]
			sigma := fields[3]

			return fmt.Sprintf("%s %s", velocity, sigma), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading kinematic file: %w", err)
	}

	return "", fmt.Errorf("file %s not found in kinematic information", fileName)
}

func (s *Starlight) RemoveInFileFromBatch(batch []api.DataFile) []api.DataFile {

	filteredBatch := make([]api.DataFile, 0, len(batch))

	for _, file := range batch {
		if !strings.HasSuffix(file.Name, ".in") {
			filteredBatch = append(filteredBatch, file)
		} else {
			inFilePath := filepath.Join(os.Getenv("IN_FILE_OUTPUT_PATH"), file.Name)
			if err := os.Remove(inFilePath); err != nil {
				log.Printf("Error removing .in file %s: %v\n", inFilePath, err)
			} else {
				log.Printf("Successfully removed .in file: %s\n", inFilePath)
			}
		}
	}
	return filteredBatch
}

func (s *Starlight) UpdateToProcessList(inFileName string, fileContent []byte) {
	PROCESS_LIST := os.Getenv("PROCESS_LIST")
	InFilePath := os.Getenv("IN_FILE_OUTPUT_PATH")

	if err := s.Utils.TouchFile(PROCESS_LIST); err != nil {
		log.Printf("│ ✗ Error creating process list: %w", err)
		return
	}

	specialFilePath := filepath.Join(InFilePath, inFileName)

	// #nosec G306
	err := os.WriteFile(specialFilePath, fileContent, 0644)
	if err != nil {
		log.Printf("│ ✗ Error writing .in file: %w", err)
		return
	}

	f, err := os.OpenFile(PROCESS_LIST, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		log.Printf("│ ✗ Error opening process list: %v", err)
		return
	}
	defer f.Close()

	if _, err = f.WriteString(inFileName + "\n"); err != nil {
		log.Printf("│ ✗ Error updating process list: %v", err)
	}
}
