package api

import (
	"log"
	"sync"
	"time"
)

// ProgressTracker manages pipeline progress for datasets
type ProgressTracker struct {
	mu       sync.RWMutex
	datasets map[string]*DatasetProgress
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		datasets: make(map[string]*DatasetProgress),
	}
}

// StartDataset initializes progress tracking for a dataset
func (pt *ProgressTracker) StartDataset(datasetID, datasetName string, filesTotal int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.datasets[datasetID] = &DatasetProgress{
		DatasetID:        datasetID,
		DatasetName:      datasetName,
		Stage:            StageReady,
		Progress:         0.0,
		FilesTotal:       filesTotal,
		FilesProcessed:   0,
		BatchesTotal:     0,
		BatchesProcessed: 0,
		StartTime:        time.Now(),
		LastUpdated:      time.Now(),
	}

	log.Printf("Progress: Started tracking dataset %s (%s) with %d files", datasetID, datasetName, filesTotal)
}

// UpdateProgress updates the progress for a dataset
func (pt *ProgressTracker) UpdateProgress(datasetID string, stage PipelineStage, progress float64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	dataset, exists := pt.datasets[datasetID]
	if !exists {
		log.Printf("Progress: Dataset %s not found for update", datasetID)
		return
	}

	dataset.Stage = stage
	dataset.Progress = progress
	dataset.LastUpdated = time.Now()

	log.Printf("Progress: Updated dataset %s to stage %s (%.1f%%)", datasetID, stage, progress)
}

// UpdateBatchProgress updates batch-specific progress
func (pt *ProgressTracker) UpdateBatchProgress(datasetID string, batchSize int, isComplete bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	dataset, exists := pt.datasets[datasetID]
	if !exists {
		log.Printf("Progress: Dataset %s not found for batch update", datasetID)
		return
	}

	if isComplete {
		dataset.BatchesProcessed++
		dataset.FilesProcessed += batchSize
		
		// Calculate progress based on files processed
		if dataset.FilesTotal > 0 {
			fileProgress := float64(dataset.FilesProcessed) / float64(dataset.FilesTotal)
			
			// Map to pipeline stages
			switch {
			case fileProgress >= 1.0:
				dataset.Stage = StageComplete
				dataset.Progress = 100.0
			case fileProgress >= 0.7:
				dataset.Stage = StageAnalysis
				dataset.Progress = 70.0 + (fileProgress-0.7)*30.0/0.3 // 70-100%
			case fileProgress >= 0.2:
				dataset.Stage = StageProcessing
				dataset.Progress = 20.0 + (fileProgress-0.2)*50.0/0.5 // 20-70%
			default:
				dataset.Stage = StageQueued
				dataset.Progress = fileProgress * 20.0 // 0-20%
			}
		}
	} else {
		dataset.BatchesTotal++
	}

	dataset.LastUpdated = time.Now()
	log.Printf("Progress: Batch update for dataset %s - %d/%d files processed", 
		datasetID, dataset.FilesProcessed, dataset.FilesTotal)
}

// SetError sets an error state for a dataset
func (pt *ProgressTracker) SetError(datasetID, errorMessage string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	dataset, exists := pt.datasets[datasetID]
	if !exists {
		log.Printf("Progress: Dataset %s not found for error update", datasetID)
		return
	}

	dataset.Stage = StageError
	dataset.ErrorMessage = errorMessage
	dataset.LastUpdated = time.Now()

	log.Printf("Progress: Error for dataset %s: %s", datasetID, errorMessage)
}

// GetProgress retrieves progress for a specific dataset
func (pt *ProgressTracker) GetProgress(datasetID string) (*DatasetProgress, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	dataset, exists := pt.datasets[datasetID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	progress := *dataset
	return &progress, true
}

// GetAllProgress retrieves progress for all datasets
func (pt *ProgressTracker) GetAllProgress() map[string]*DatasetProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make(map[string]*DatasetProgress)
	for id, dataset := range pt.datasets {
		// Return copies to avoid race conditions
		progress := *dataset
		result[id] = &progress
	}

	return result
}

// CleanupCompleted removes completed datasets older than specified duration
func (pt *ProgressTracker) CleanupCompleted(maxAge time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	
	for id, dataset := range pt.datasets {
		if dataset.Stage == StageComplete && dataset.LastUpdated.Before(cutoff) {
			delete(pt.datasets, id)
			log.Printf("Progress: Cleaned up completed dataset %s", id)
		}
	}
} 