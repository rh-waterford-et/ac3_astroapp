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
	fullKey := filepath.Join(s.InputDir, filename)
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, fmt.Errorf("error reading file %s from S3: %v", filename, err)
	}
	defer result.Body.Close()

	//log.Printf("Successfully read file from S3: %s", filepath.Join(s.InputDir, filename))
	data, readErr := io.ReadAll(result.Body)
	return data, readErr
}

func (s *S3FileSource) DeleteFile(filename string) error {
	s3Client := s.Bucket.GetS3Client()
	bucketName := s.Bucket.GetBucketName()

	// Construct source and destination keys
	sourceKey := filepath.Join(s.InputDir, filename)

	// Replace /input/ with /processed/ in the path
	processedDir := strings.Replace(s.InputDir, "/input", "/processed", 1)
	destKey := strings.Replace(sourceKey, s.InputDir, processedDir, 1)


	// Copy file to processed directory
	copySource := bucketName + "/" + sourceKey
	_, err := s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		CopySource: aws.String(copySource),
		Key:        aws.String(destKey),
	})
	if err != nil {
		// Check if the error is because the source file no longer exists
		if aerr, ok := err.(awserr.Error); ok && (aerr.Code() == "NoSuchKey" || aerr.Code() == "NotFound") {
			log.Printf("File %s no longer exists during copy, may have been processed concurrently", filename)
			return nil // Don't treat this as an error
		}
		return fmt.Errorf("copy failed: %v", err)
	}

	// Wait for copy to complete with timeout
	err = s3Client.WaitUntilObjectExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(destKey),
	})
	if err != nil {
		return fmt.Errorf("copy verification failed: %v", err)
	}

	// Delete original file
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		// If deletion fails but copy succeeded, log as warning rather than error
		return nil // Don't fail the operation if copy succeeded
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
	batchName := parts[0]

	return batchName, nil
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

		} else {
			log.Printf("DEBUG: Failed to extract batch name from file %s: %v", file, err)
		}
	}

	return batchCounts, nil
}
