package producer

import (
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
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

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filepath.Join(s.InputDir, filename)),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(result.Body)
}

func (s *S3FileSource) DeleteFile(filename string) error {
    s3Client := s.Bucket.GetS3Client()
    bucketName := s.Bucket.GetBucketName()

    sourceKey := filepath.Join(s.InputDir, filename)
    destKey := strings.Replace(sourceKey, "/input/", "/processed/", 1)

    _, err := s3Client.CopyObject(&s3.CopyObjectInput{
        Bucket:     aws.String(bucketName),
        CopySource: aws.String(bucketName + "/" + sourceKey),
        Key:        aws.String(destKey),
    })
    if err != nil {
        return fmt.Errorf("copy failed: %v", err)
    }

    err = s3Client.WaitUntilObjectExists(&s3.HeadObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(destKey),
    })
    if err != nil {
        return fmt.Errorf("copy verification failed: %v", err)
    }

    _, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(sourceKey),
    })
    if err != nil {
        return fmt.Errorf("delete failed: %v", err)
    }

    log.Printf("Moved s3://%s/%s -> s3://%s/%s", bucketName, sourceKey, bucketName, destKey)
    return nil
}