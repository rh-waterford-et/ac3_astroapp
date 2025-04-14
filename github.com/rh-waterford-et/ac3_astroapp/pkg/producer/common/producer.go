package common

import (
	"log"
	"os"
	"path/filepath"
)

type DataFile struct {
	Name    string
	Content string
}

type Event struct {
	Files []DataFile
}

type ProducerInterface interface {
	AddFile(file DataFile)
	SendBatch()
	ReadFiles()
	MoveProcessedFiles()
}

type BaseProducer struct {
	BatchSize  int
	Batch      []DataFile
	EventQueue chan Event
	InputDir   string
	OutputDir  string
}

func NewBaseProducer(batchSize int, inputDir, outputDir string, eventQueue chan Event) *BaseProducer {
	return &BaseProducer{
		BatchSize:  batchSize,
		Batch:      make([]DataFile, 0, batchSize),
		EventQueue: eventQueue,
		InputDir:   inputDir,
		OutputDir:  outputDir,
	}
}

func (p *BaseProducer) AddFile(file DataFile) {
	p.Batch = append(p.Batch, file)
	if len(p.Batch) >= p.BatchSize {
		p.SendBatch()
	}
}

func (p *BaseProducer) SendBatch() {
	if len(p.Batch) > 0 {
		event := Event{Files: p.Batch}
		p.EventQueue <- event
		p.MoveProcessedFiles()
		p.Batch = make([]DataFile, 0, p.BatchSize)
	}
}

func (p *BaseProducer) MoveProcessedFiles() {
	for _, file := range p.Batch {
		sourcePath := filepath.Join(p.InputDir, file.Name)
		destPath := filepath.Join(p.OutputDir, file.Name)
		err := os.Rename(sourcePath, destPath)
		if err != nil {
			log.Printf("Error moving file %s: %v\n", file.Name, err)
		}
	}
}

func (p *BaseProducer) ReadFiles() {
	files, err := os.ReadDir(p.InputDir)
	if err != nil {
		log.Printf("Failed reading input directory: %v", err)
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(p.InputDir, file.Name()))
			if err != nil {
				log.Printf("Error reading file %s: %v\n", file.Name(), err)
				continue
			}
			p.AddFile(DataFile{Name: file.Name(), Content: string(content)})
		}
	}
}
