package dataset

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

// DatasetManager handles local dataset operations and S3 uploads
type DatasetManager struct {
	s3Client      *s3bucket.S3Bucket
	experimentID  string
	processorType string
}

// LocalDataset represents a local dataset directory
type LocalDataset struct {
	Path        string
	Files       []string
	TotalSize   int64
	DatasetName string
}

// NewDatasetManager creates a new dataset manager with UC3 S3 integration
func NewDatasetManager(experimentID, processorType string) (*DatasetManager, error) {
	// Validate processor type
	validTypes := []string{"starlight", "ppxf", "steckmap"}
	isValid := false
	for _, validType := range validTypes {
		if processorType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return nil, fmt.Errorf("invalid processor type '%s', must be one of: %s",
			processorType, strings.Join(validTypes, ", "))
	}

	return &DatasetManager{
		s3Client:      nil, // Lazy initialization
		experimentID:  experimentID,
		processorType: processorType,
	}, nil
}

// getS3Client creates S3 client on demand
func (dm *DatasetManager) getS3Client() *s3bucket.S3Bucket {
	if dm.s3Client == nil {
		dm.s3Client = s3bucket.NewS3Bucket()
	}
	return dm.s3Client
}

// ScanLocalDataset scans a local directory for dataset files
func (dm *DatasetManager) ScanLocalDataset(localPath string) (*LocalDataset, error) {
	// Validate path exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("dataset path does not exist: %s", localPath)
	}

	dataset := &LocalDataset{
		Path:  localPath,
		Files: make([]string, 0),
	}

	// Walk directory and find supported files
	err := filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check for processor-specific file extensions
		ext := strings.ToLower(filepath.Ext(d.Name()))

		// Filter by processor type
		var isSupported bool
		switch dm.processorType {
		case "starlight":
			isSupported = ext == ".txt"
		case "ppxf":
			isSupported = ext == ".fits"
		case "steckmap":
			// Accept other file types for steckmap
			supportedExts := []string{".csv", ".log", ".in", ".txt", ".fits"}
			for _, supportedExt := range supportedExts {
				if ext == supportedExt {
					isSupported = true
					break
				}
			}
		}

		if isSupported {
			// Get relative path from dataset root
			relPath, err := filepath.Rel(localPath, path)
			if err != nil {
				return err
			}

			dataset.Files = append(dataset.Files, relPath)

			// Get file size
			if info, err := d.Info(); err == nil {
				dataset.TotalSize += info.Size()
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning dataset directory: %w", err)
	}

	// Extract dataset name from first file if available
	if len(dataset.Files) > 0 {
		firstFile := filepath.Base(dataset.Files[0])
		dataset.DatasetName = dm.extractDatasetName(firstFile)
	}

	return dataset, nil
}

// extractDatasetName extracts the dataset identifier from a filename
func (dm *DatasetManager) extractDatasetName(filename string) string {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Common patterns for astronomical datasets
	// Pattern 1: NGC7025_... -> NGC7025
	if strings.HasPrefix(nameWithoutExt, "NGC") || strings.HasPrefix(nameWithoutExt, "IC") {
		parts := strings.Split(nameWithoutExt, "_")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	// Pattern 2: manga-8250-1902-... -> manga-8250-1902
	if strings.HasPrefix(nameWithoutExt, "manga-") {
		parts := strings.Split(nameWithoutExt, "-")
		if len(parts) >= 3 {
			return strings.Join(parts[0:3], "-")
		}
	}

	// Pattern 3: Generic fallback - take first part before underscore/dash
	if strings.Contains(nameWithoutExt, "_") {
		parts := strings.Split(nameWithoutExt, "_")
		return parts[0]
	}
	if strings.Contains(nameWithoutExt, "-") {
		parts := strings.Split(nameWithoutExt, "-")
		return parts[0]
	}

	// Fallback: use the whole filename without extension
	return nameWithoutExt
}

// GetS3Prefix returns the standard UC3 S3 path with dataset organization
func (dm *DatasetManager) GetS3Prefix() string {
	return fmt.Sprintf("%s/input/%s", dm.processorType, dm.experimentID)
}

// GetS3PrefixForDataset returns the S3 path using the actual dataset name
func (dm *DatasetManager) GetS3PrefixForDataset(datasetName string) string {
	return fmt.Sprintf("%s/input/%s", dm.processorType, datasetName)
}

// UploadDataset uploads a local dataset to S3 using UC3 standard structure
func (dm *DatasetManager) UploadDataset(localDataset *LocalDataset) error {
	// Use the actual dataset name for S3 path, not the experiment ID
	s3Prefix := dm.GetS3PrefixForDataset(localDataset.DatasetName)

	fmt.Printf("Uploading %d files to S3 path: %s\n", len(localDataset.Files), s3Prefix)

	for i, relativeFile := range localDataset.Files {
		// Read local file
		localFilePath := filepath.Join(localDataset.Path, relativeFile)
		content, err := os.ReadFile(localFilePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", localFilePath, err)
		}

		// Upload to S3 using UC3 standard structure
		fileName := filepath.Base(relativeFile)
		err = dm.getS3Client().UploadFileToBucket(s3Prefix, fileName, content)
		if err != nil {
			return fmt.Errorf("failed to upload file %s: %w", fileName, err)
		}

		fmt.Printf("  [%d/%d] Uploaded: %s\n", i+1, len(localDataset.Files), fileName)
	}

	fmt.Printf("✓ Successfully uploaded %d files to s3://%s/%s\n",
		len(localDataset.Files), dm.getS3Client().GetBucketName(), s3Prefix)

	return nil
}

// TestS3Connection tests the S3 connection using UC3 credentials
func (dm *DatasetManager) TestS3Connection() error {
	// Test S3 connection by getting bucket name
	bucketName := dm.getS3Client().GetBucketName()
	if bucketName == "" {
		return fmt.Errorf("S3 bucket name not configured")
	}

	fmt.Printf("✓ S3 connection successful\n")
	fmt.Printf("  Bucket: %s\n", bucketName)
	fmt.Printf("  Upload path template: %s/input/<dataset-name>\n", dm.processorType)

	return nil
}
