package common

import (
	"fmt"
	"log"
	"os"
)

type UtilsInterface interface {
	Exists(path string) (bool, error)
	FailOnError(msg string, err error)
	TouchFile(name string) error
}

type Utils struct{}

func (u *Utils) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("%w", err)
}

func (u *Utils) FailOnError(msg string, err error) {
	if err != nil {
		log.Panicf("%s: %w", msg, err)
	}
}

func (u *Utils) TouchFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	err = file.Close()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (u *Utils) EnsureDirectoriesExist() error {
	requiredDirs := []string{
		os.Getenv("EXPLORED_DIR_STARLIGHT"),
		os.Getenv("INPUT_DIR_STARLIGHT"),
		os.Getenv("EXPLORED_DIR_PPFX"),
		os.Getenv("OUTPUT_DIR_PPFX"),
		os.Getenv("EXPLORED_DIR_STECKMAP"),
		os.Getenv("OUTPUT_DIR_STECKMAP"),
		os.Getenv("IN_FILE_OUTPUT_PATH"),
	}

	for _, dir := range requiredDirs {
		if dir == "" {
			// Skip empty paths (env vars not set)
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		log.Printf("Verified directory: %s", dir)
	}
	return nil
}
