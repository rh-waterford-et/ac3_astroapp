package producer

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalFileSource handles local filesystem operations
type LocalFileSource struct {
	InputDir     string
	OutputDir    string
	ProcessedDir string
}

func NewLocalFileSource(inputDir, outputDir, processedDir string) *LocalFileSource {
	return &LocalFileSource{
		InputDir:     inputDir,
		OutputDir:    outputDir,
		ProcessedDir: processedDir,
	}
}

func (l *LocalFileSource) ListFiles() ([]string, error) {
	entries, err := os.ReadDir(l.InputDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (l *LocalFileSource) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.InputDir, filename))
}

func (l *LocalFileSource) DeleteFile(filename string) error {
	sourcePath := filepath.Join(l.InputDir, filename)
	destPath := filepath.Join(l.ProcessedDir, filename)
	//log.Print(sourcePath, destPath)
	err := moveFile(sourcePath, destPath)
	if err != nil {
		fmt.Printf("Error moving file %s: %v\n", filename, err)
	}
	return nil
}

func moveFile(source, destination string) error {
	return os.Rename(source, destination)
}
