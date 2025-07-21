package producer

import (
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

// S3FileSource handles S3 bucket operations
type S3FileSource struct {
	Bucket    s3bucket.S3BucketInterface
	AppName   string
	InputDir  string // S3 prefix
	OutputDir string // S3 prefix
}

func NewS3FileSource(bucket s3bucket.S3BucketInterface, appName, inputDir, outputDir string) *S3FileSource {
	return &S3FileSource{
		Bucket:    bucket,
		AppName:   appName,
		InputDir:  inputDir,
		OutputDir: outputDir,
	}
}

func (s *S3FileSource) ListFiles() ([]string, error) {
	return s.Bucket.GetNewAssets(s.InputDir)
}

func (s *S3FileSource) ReadFile(filename string) ([]byte, error) {
	s3Client := s.Bucket.GetS3Client()
	bucketName := s.Bucket.GetBucketName()

	// filename now includes batch directory (e.g., "NGC7025/spectrum_001.txt")
	// Construct the full S3 key: inputDir/batchDir/filename
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filepath.Join(s.InputDir, filename)),
	})
	if err != nil {
		return nil, fmt.Errorf("error reading file %s from S3: %v", filename, err)
	}
	defer result.Body.Close()

	//log.Printf("Successfully read file from S3: %s", filepath.Join(s.InputDir, filename))
	return io.ReadAll(result.Body)
}

func (s *S3FileSource) DeleteFile(filename string) error {
	s3Client := s.Bucket.GetS3Client()
	bucketName := s.Bucket.GetBucketName()

	parts := strings.Split(filename, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid filename format, expected batch/filename: %s", filename)
	}
	batchName := parts[0]
	actualFilename := strings.Join(parts[1:], "/")

	// Skip moving placeholder files - they should stay in input directory to maintain batch structure
	if strings.Contains(actualFilename, ".batch_placeholder") || strings.Contains(actualFilename, ".dataset_placeholder") {
		log.Printf("Skipping placeholder file, keeping in input directory: %s", filename)
		return nil
	}

	// Construct source and destination keys
	sourceKey := filepath.Join(s.InputDir, filename)

	// First check if source file exists
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			return fmt.Errorf("source file does not exist: s3://%s/%s", bucketName, sourceKey)
		}
		return fmt.Errorf("error checking source file: %v", err)
	}

	// Replace /input/ with /processed/ and maintain batch structure
	processedDir := strings.Replace(s.InputDir, "/input", "/processed", 1)
	destKey := filepath.Join(processedDir, batchName, actualFilename)

	// Copy file to processed directory
	_, err = s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		CopySource: aws.String(bucketName + "/" + sourceKey),
		Key:        aws.String(destKey),
	})
	if err != nil {
		return fmt.Errorf("copy failed for batch %s: %v", batchName, err)
	}

	// Wait for copy to complete
	err = s3Client.WaitUntilObjectExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(destKey),
	})
	if err != nil {
		return fmt.Errorf("copy verification failed for batch %s: %v", batchName, err)
	}

	// Delete original file
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		return fmt.Errorf("delete failed for batch %s: %v", batchName, err)
	}

	return nil
}

// ExtractBatchName extracts the batch name from a file path
// e.g., "NGC7025/spectrum_001.txt" -> "NGC7025"
func (s *S3FileSource) ExtractBatchName(filename string) (string, error) {
	parts := strings.Split(filename, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid filename format, expected batch/filename: %s", filename)
	}
	return parts[0], nil
}

// GetBatchesWithFiles returns a map of batch names to their file counts
func (s *S3FileSource) GetBatchesWithFiles() (map[string]int, error) {
	files, err := s.ListFiles()
	if err != nil {
		return nil, err
	}

	batchCounts := make(map[string]int)
	for _, file := range files {
		if batch, err := s.ExtractBatchName(file); err == nil {
			batchCounts[batch]++
		}
	}

	return batchCounts, nil
}
