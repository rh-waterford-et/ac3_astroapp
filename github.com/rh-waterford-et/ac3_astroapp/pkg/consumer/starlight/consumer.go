package starlight

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/consumer/common"
)

type StarlightConsumer struct {
	*common.BaseConsumer
}

func NewStarlightConsumer() *StarlightConsumer {
	base := common.NewBaseConsumer("starlight")
	return &StarlightConsumer{BaseConsumer: base}
}

func (c *StarlightConsumer) HandleSpecialFile(file common.FileData) bool {
	if strings.HasSuffix(file.Name, ".in") {
		c.updateToProcessList(file.Name, []byte(file.Content))
		log.Printf("│ ✓ Processed .in file: %s", file.Name)
		return true
	}
	return false
}

func (c *StarlightConsumer) updateToProcessList(inFileName string, fileContent []byte) {
	processList := os.Getenv("PROCESS_LIST")
	inFilePath := os.Getenv("IN_FILE_PATH")

	if err := touchFile(processList); err != nil {
		log.Printf("│ ✗ Error creating process list: %v", err)
		return
	}

	specialFilePath := filepath.Join(inFilePath, inFileName)
	if err := os.WriteFile(specialFilePath, fileContent, 0644); err != nil {
		log.Printf("│ ✗ Error writing .in file: %v", err)
		return
	}

	f, err := os.OpenFile(processList, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		log.Printf("│ ✗ Error opening process list: %v", err)
		return
	}
	defer f.Close()

	if _, err = f.WriteString(inFileName + "\n"); err != nil {
		log.Printf("│ ✗ Error updating process list: %v", err)
	}
}

func touchFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}
