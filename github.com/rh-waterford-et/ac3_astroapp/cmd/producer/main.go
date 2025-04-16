package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/producer"
)

func main() {

	if err := ensureDirectoriesExist(); err != nil {
		log.Fatalf("Directory initialization failed: %v", err)
	}
	// Run all three applications concurrently
	for {
		producer.RunApp("PPFX")
		producer.RunApp("Starlight")
		producer.RunApp("Steckmap")
		log.Println("Checking for new files...")
		time.Sleep(10 * time.Second)
	}

}

func ensureDirectoriesExist() error {
	requiredDirs := []string{
		os.Getenv("INPUT_DIR_Starlight"),
		os.Getenv("OUTPUT_DIR_Starlight"),
		os.Getenv("INPUT_DIR_PPFX"),
		os.Getenv("OUTPUT_DIR_PPFX"),
		os.Getenv("INPUT_DIR_Steckmap"),
		os.Getenv("OUTPUT_DIR_Steckmap"),
		os.Getenv("IN_FILE_OUTPUT_PATH"),
	}

	for _, dir := range requiredDirs {
		if dir == "" {
			continue // Skip empty paths (env vars not set)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
		log.Printf("Verified directory: %s", dir)
	}
	return nil
}
