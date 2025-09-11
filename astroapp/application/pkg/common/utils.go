package common

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
)

type UtilsInterface interface {
	Exists(path string) (bool, error)
	FailOnError(msg string, err error)
	TouchFile(name string) error
	GenerateUUID() string
}

type Utils struct{}

func (u *Utils) GenerateUUID() string {
	return uuid.New().String()
}
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
		log.Panicf("%s: %v", msg, err)
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
		os.Getenv("EXPLORED_DIR_PPXF"),
		os.Getenv("INPUT_DIR_PPXF"),
		os.Getenv("EXPLORED_DIR_STECKMAP"),
		os.Getenv("OUTPUT_DIR_STECKMAP"),
		os.Getenv("IN_FILE_OUTPUT_PATH"),
		os.Getenv("PROCESSED_STECKMAP"),
		os.Getenv("PROCESSED_STARLIGHT"),
		os.Getenv("PROCESSED_PPXF"),
		os.Getenv("BATCH_INFO_DIR"),
		os.Getenv("PROCESS_LIST_STARLIGHT_PATH"),
		os.Getenv("PROCESS_LIST_PPXF_PATH"),
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

	processList := []string{os.Getenv("PROCESS_LIST_STARLIGHT"), os.Getenv("PROCESS_LIST_PPXF")}
	for _, processList := range processList {
		// Create process list file if it doesn't exist
		if _, err := os.Stat(processList); os.IsNotExist(err) {
			if err := u.TouchFile(processList); err != nil {
				log.Printf("│ ✗ Error creating process list: %v", err)
			} else {
				log.Printf("│ ✓ Creating process list: %s", processList)
			}
		}
	}

	return nil
}

func (u *Utils) EnsureBucketDirectoriesExist(bucket s3bucket.S3BucketInterface) error {
	requiredDirs := []string{
		os.Getenv("EXPLORED_STARLIGHT"),
		os.Getenv("EXPLORED_PPXF"),
		os.Getenv("EXPLORED_STECKMAP"),
		os.Getenv("PROCESSED_STARLIGHT"),
		os.Getenv("PROCESSED_PPXF"),
		os.Getenv("PROCESSED_STECKMAP"),
		os.Getenv("OUTPUT_STARLIGHT"),
		os.Getenv("OUTPUT_PPXF"),
		os.Getenv("OUTPUT_STECKMAP"),
		os.Getenv("METRICS"),
	}

	for _, dir := range requiredDirs {
		if dir == "" {
			continue
		}

		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}

		_, err := bucket.GetS3Client().HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(bucket.GetBucketName()),
			Key:    aws.String(dir),
		})

		if err == nil {
			log.Printf("Directory already exists in bucket: %s", dir)
			continue
		}

		if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NotFound" {
			return fmt.Errorf("failed to check directory %s in bucket: %w", dir, err)
		}

		_, err = bucket.GetS3Client().PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucket.GetBucketName()),
			Key:    aws.String(dir),
		})
		if err != nil {
			return fmt.Errorf("failed to create directory %s in bucket: %w", dir, err)
		}
		log.Printf("Created directory in bucket: %s", dir)
	}

	checkInFile := os.Getenv("IN_FILE_OUTPUT_PATH")
	if err := os.MkdirAll(checkInFile, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", checkInFile, err)
	}
	log.Printf("Verified directory: %s", checkInFile)

	return nil
}
