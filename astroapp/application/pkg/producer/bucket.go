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
	JobName   string // Optional job filter
}

// SingleFileSource handles single file operations
type SingleFileSource struct {
	Bucket       s3bucket.S3BucketInterface
	AppName      string
	InputDir     string // S3 prefix for input
	ProcessedDir string // S3 prefix for processed files
	OutputDir    string // S3 prefix for output
	JobName      string // Job name
	FileName     string // Specific file name
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
	if s.JobName != "" {
		// Scan specific job directory only
		return s.ListFilesForJob(s.JobName)
	}
	// Scan all directories (fallback)
	return s.Bucket.GetNewAssets(s.InputDir)
}

// ListFilesForJob scans files in a specific job directory
func (s *S3FileSource) ListFilesForJob(jobName string) ([]string, error) {
	jobPath := s.InputDir + "/" + jobName
	files, err := s.Bucket.GetS3Objects(jobPath)
	if err != nil {
		return nil, err
	}

	// Add job directory prefix to each file
	var jobFiles []string
	for _, file := range files {
		if !strings.HasSuffix(file, "/") { // Skip directory markers
			jobFiles = append(jobFiles, jobName+"/"+file)
		}
	}
	log.Printf("Found %d files in batch %s", len(jobFiles), jobName)
	return jobFiles, nil
}

// ListFiles implementation for SingleFileSource - returns single file if it exists
func (s *SingleFileSource) ListFiles() ([]string, error) {
	// First check the input directory
	inputJobPath := fmt.Sprintf("%s/%s", s.InputDir, s.JobName)
	objects, err := s.Bucket.GetS3Objects(inputJobPath)
	if err == nil {
		for _, obj := range objects {
			if strings.HasSuffix(obj, s.FileName) {
				log.Printf("Found file in input directory: %s/%s", inputJobPath, s.FileName)
				return []string{fmt.Sprintf("%s/%s", s.JobName, s.FileName)}, nil
			}
		}
	}

	// If not found in input, check the processed directory
	processedJobPath := fmt.Sprintf("%s/%s", s.ProcessedDir, s.JobName)
	objects, err = s.Bucket.GetS3Objects(processedJobPath)
	if err == nil {
		for _, obj := range objects {
			if strings.HasSuffix(obj, s.FileName) {
				log.Printf("Found file in processed directory: %s/%s", processedJobPath, s.FileName)
				return []string{fmt.Sprintf("%s/%s", s.JobName, s.FileName)}, nil
			}
		}
	}

	// File not found in either location
	log.Printf("File not found in either input or processed directories. Searched for: %s", s.FileName)
	return []string{}, nil
}

// ReadFile implementation for SingleFileSource
func (s *SingleFileSource) ReadFile(filename string) ([]byte, error) {
	// For single file source, check both input and processed directories
	var inputKey, processedKey string
	if strings.Contains(filename, "/") {
		// filename already includes job (e.g., "job/file.fits")
		inputKey = fmt.Sprintf("%s/%s", s.InputDir, filename)
		processedKey = fmt.Sprintf("%s/%s", s.ProcessedDir, filename)
	} else {
		// just filename, add job prefix
		inputKey = fmt.Sprintf("%s/%s/%s", s.InputDir, s.JobName, filename)
		processedKey = fmt.Sprintf("%s/%s/%s", s.ProcessedDir, s.JobName, filename)
	}

	// Try input directory first
	data, err := s.Bucket.DownloadFile(inputKey)
	if err == nil {
		log.Printf("Read file from input directory: %s", inputKey)
		return data, nil
	}

	// Try processed directory
	data, err = s.Bucket.DownloadFile(processedKey)
	if err == nil {
		log.Printf("Read file from processed directory: %s", processedKey)
		return data, nil
	}

	return nil, fmt.Errorf("file not found in either input (%s) or processed (%s) directories", inputKey, processedKey)
}

// DeleteFile implementation for SingleFileSource
func (s *SingleFileSource) DeleteFile(filename string) error {
	// For single file source, check both input and processed directories
	var inputKey, processedKey string
	if strings.Contains(filename, "/") {
		// filename already includes job (e.g., "job/file.fits")
		inputKey = fmt.Sprintf("%s/%s", s.InputDir, filename)
		processedKey = fmt.Sprintf("%s/%s", s.ProcessedDir, filename)
	} else {
		// just filename, add job prefix
		inputKey = fmt.Sprintf("%s/%s/%s", s.InputDir, s.JobName, filename)
		processedKey = fmt.Sprintf("%s/%s/%s", s.ProcessedDir, s.JobName, filename)
	}

	// Try to delete from input directory first
	err := s.Bucket.DeleteFile(inputKey)
	if err == nil {
		log.Printf("Deleted file from input directory: %s", inputKey)
		return nil
	}

	// Try to delete from processed directory
	err = s.Bucket.DeleteFile(processedKey)
	if err == nil {
		log.Printf("Deleted file from processed directory: %s", processedKey)
		return nil
	}

	return fmt.Errorf("file not found in either input (%s) or processed (%s) directories for deletion", inputKey, processedKey)
}

func (s *S3FileSource) ReadFile(filename string) ([]byte, error) {
	s3Client := s.Bucket.GetS3Client()
	bucketName := s.Bucket.GetBucketName()

	// filename now includes job directory (e.g., "NGC7025/spectrum_001.txt")
	// Construct the full S3 key: inputDir/jobDir/filename
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

	// Construct source and destination keys
	sourceKey := filepath.Join(s.InputDir, filename)
	if strings.HasPrefix(filename, s.InputDir) {
		// If filename already includes the input directory, use it as is
		sourceKey = filename
	}

	// First check if source file exists
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			// File doesn't exist - it may have already been processed/moved
			log.Printf("File %s already processed or moved, skipping deletion", filename)
			return nil // Don't treat this as an error
		}
		return fmt.Errorf("error checking source file: %v", err)
	}

	// Replace /input/ with /processed/ in the path
	processedDir := strings.Replace(s.InputDir, "/input", "/processed", 1)
	destKey := strings.Replace(sourceKey, s.InputDir, processedDir, 1)

	// Create destination directory structure if needed
	_, err = s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filepath.Dir(destKey) + "/"),
	})
	if err != nil {
		// Create directory marker if it doesn't exist
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(filepath.Dir(destKey) + "/"),
			Body:   strings.NewReader(""),
		})
		if err != nil {
			return fmt.Errorf("failed to create destination directory: %v", err)
		}
	}

	// Copy file to processed directory
	_, err = s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		CopySource: aws.String(bucketName + "/" + sourceKey),
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
		log.Printf("Warning: Failed to delete original file %s after successful copy: %v", filename, err)
		return nil // Don't fail the operation if copy succeeded
	}

	return nil
}

// ExtractJobName extracts the job name from a file path
// e.g., "NGC7025/spectrum_001.txt" -> "NGC7025"
func (s *S3FileSource) ExtractJobName(filename string) (string, error) {
	parts := strings.Split(filename, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid filename format, expected job/filename: %s", filename)
	}
	return parts[0], nil
}

// GetJobesWithFiles returns a map of job names to their file counts
func (s *S3FileSource) GetJobesWithFiles() (map[string]int, error) {
	files, err := s.ListFiles()
	if err != nil {
		return nil, err
	}

	jobCounts := make(map[string]int)
	for _, file := range files {
		if job, err := s.ExtractJobName(file); err == nil {
			jobCounts[job]++
		}
	}

	return jobCounts, nil
}
