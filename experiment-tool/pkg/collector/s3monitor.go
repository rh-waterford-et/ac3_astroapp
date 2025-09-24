package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

// S3Monitor handles S3-based completion detection for different processors
type S3Monitor struct {
	s3Client s3bucket.S3BucketInterface
}

// S3MonitorInterface defines the interface for S3-based completion detection
type S3MonitorInterface interface {
	WaitForCompletion(ctx context.Context, experiment *ExperimentRun) error
	CountOutputFiles(ctx context.Context, experiment *ExperimentRun) (int, error)
	GetS3Client() s3bucket.S3BucketInterface
}

// NewS3Monitor creates a new S3Monitor using UC3's S3 client
func NewS3Monitor() (*S3Monitor, error) {
	s3Client := s3bucket.NewS3Bucket()
	if s3Client == nil {
		return nil, fmt.Errorf("failed to create S3 client")
	}

	return &S3Monitor{
		s3Client: s3Client,
	}, nil
}

// GetS3Client returns the underlying S3 client for advanced operations
func (s *S3Monitor) GetS3Client() s3bucket.S3BucketInterface {
	return s.s3Client
}

// WaitForCompletion waits for processing to complete by monitoring S3 output files
func (s *S3Monitor) WaitForCompletion(ctx context.Context, experiment *ExperimentRun) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Printf("Waiting for %s completion: expecting %d output files (%d input × %d files per input)",
		experiment.ProcessorType,
		experiment.ExpectedOutputCount,
		experiment.UploadedFileCount,
		experiment.OutputPattern.FilesPerInput)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			outputCount, err := s.CountOutputFiles(ctx, experiment)
			if err != nil {
				log.Printf("Error counting output files: %v", err)
				continue
			}

			log.Printf("Dataset %s (%s): %d/%d files processed",
				experiment.DatasetName,
				experiment.ProcessorType,
				outputCount,
				experiment.ExpectedOutputCount)

			if outputCount >= experiment.ExpectedOutputCount {
				log.Printf("Dataset %s completed! (%d %s output files generated)",
					experiment.DatasetName, outputCount, experiment.ProcessorType)
				return nil
			}
		}
	}
}

// CountOutputFiles counts output files based on processor type
func (s *S3Monitor) CountOutputFiles(ctx context.Context, experiment *ExperimentRun) (int, error) {
	switch experiment.ProcessorType {
	case "starlight":
		return s.countStarlightOutputs(ctx, experiment)
	case "ppxf":
		return s.countPPXFOutputs(ctx, experiment)
	default:
		return 0, fmt.Errorf("unsupported processor type: %s", experiment.ProcessorType)
	}
}

// countStarlightOutputs counts files directly in starlight/output/NGC7025/
func (s *S3Monitor) countStarlightOutputs(ctx context.Context, experiment *ExperimentRun) (int, error) {
	// Remove trailing slash for S3 prefix
	prefix := strings.TrimSuffix(experiment.OutputBasePath, "/")

	objects, err := s.s3Client.GetS3Objects(prefix)
	if err != nil {
		return 0, fmt.Errorf("failed to list S3 objects for prefix %s: %w", prefix, err)
	}

	count := 0
	for _, relativeKey := range objects {
		// Skip directory markers (keys ending with /)
		if !strings.HasSuffix(relativeKey, "/") {
			// Check if file matches expected patterns
			if s.matchesFilePatterns(relativeKey, experiment.OutputPattern.FilePatterns) {
				count++
			}
		}
	}

	return count, nil
}

// countPPXFOutputs counts files in all cell subdirectories under ppxf/output/NGC7025/
func (s *S3Monitor) countPPXFOutputs(ctx context.Context, experiment *ExperimentRun) (int, error) {
	// Remove trailing slash for S3 prefix
	prefix := strings.TrimSuffix(experiment.OutputBasePath, "/")

	objects, err := s.s3Client.GetS3Objects(prefix)
	if err != nil {
		return 0, fmt.Errorf("failed to list S3 objects for prefix %s: %w", prefix, err)
	}

	count := 0
	for _, relativeKey := range objects {
		// Skip directory markers (keys ending with /)
		if !strings.HasSuffix(relativeKey, "/") {
			// Check if file is in a cell subdirectory (e.g., "0/file.fits")
			if s.isPPXFCellFile(relativeKey, "") { // Pass empty basePath since key is already relative
				// Check if file matches expected patterns
				if s.matchesFilePatterns(relativeKey, experiment.OutputPattern.FilePatterns) {
					count++
				}
			}
		}
	}

	return count, nil
}

// matchesFilePatterns checks if a filename matches any of the expected patterns
func (s *S3Monitor) matchesFilePatterns(filename string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // If no patterns specified, accept all files
	}

	for _, pattern := range patterns {
		if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// isPPXFCellFile checks if a file is in a PPXF cell subdirectory
func (s *S3Monitor) isPPXFCellFile(relativeKey, basePath string) bool {
	// Since the key is already relative, we don't need to trim basePath
	// Check if path has at least one directory level (cell number)
	// Expected format: "0/filename.fits" or "12/filename.txt"
	parts := strings.Split(relativeKey, "/")
	return len(parts) >= 2 // At least cell_dir/filename
}
