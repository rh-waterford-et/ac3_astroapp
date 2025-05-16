package common

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
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
		os.Getenv("PROCESSED_DIR_STECKMAP"),
		os.Getenv("PROCESSED_DIR_STARLIGHT"),
		os.Getenv("PROCESSED_DIR_PPFX"),
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

func (u *Utils) EnsureBucketDirectoriesExist(bucket s3bucket.S3BucketInterface) error {
	requiredDirs := []string{
		os.Getenv("EXPLORED_STARLIGHT"),
		os.Getenv("EXPLORED_PPFX"),
		os.Getenv("EXPLORED_STECKMAP"),
		os.Getenv("PROCESSED_STARLIGHT"),
		os.Getenv("PROCESSED_PPFX"),
		os.Getenv("PROCESSED_STECKMAP"),
		os.Getenv("OUTPUT_STARLIGHT"),
		os.Getenv("OUTPUT_PPFX"),
		os.Getenv("OUTPUT_STECKMAP"),
	}

	for _, dir := range requiredDirs {
		if dir == "" {
			continue
		}

		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}

		// Check if the directory exists by listing objects with this prefix
		resp, err := bucket.GetS3Client().ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket.GetBucketName()),
			Prefix:  aws.String(dir),
			MaxKeys: aws.Int64(1),
		})
		if err != nil {
			return fmt.Errorf("failed to check directory %s in bucket: %w", dir, err)
		}

		if len(resp.Contents) == 0 && len(resp.CommonPrefixes) == 0 {
			_, err := bucket.GetS3Client().PutObject(&s3.PutObjectInput{
				Bucket: aws.String(bucket.GetBucketName()),
				Key:    aws.String(dir),
			})
			if err != nil {
				return fmt.Errorf("failed to create directory %s in bucket: %w", dir, err)
			}
			log.Printf("Created directory in bucket: %s", dir)
		} else {
			log.Printf("Directory already exists in bucket: %s", dir)
		}
	}
	return nil
}
