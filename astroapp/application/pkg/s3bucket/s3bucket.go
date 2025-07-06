package s3bucket

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type ObjectMetadata struct {
	Size         int64
	LastModified time.Time
}

type S3BucketInterface interface {
	//InitializeKnownAssets(appName string)
	GetNewAssets(appName string) ([]string, error)
	GetS3Objects(appName string) ([]string, error)
	GetBatchDirectories(appName string) ([]string, error)
	GetS3Client() *s3.S3
	GetBucketName() string
	UploadFileToBucket(folderPath string, fileName string, content []byte) error
	GetObjectMetadata(objectKey string) (*ObjectMetadata, error)
}

type S3Watcher struct {
	Bucket      S3BucketInterface
	BucketName  string
	KnownAssets map[string]bool
}

type S3Bucket struct {
	S3Client   *s3.S3
	BucketName string
}

func NewS3Watcher() *S3Watcher {
	bucket := NewS3Bucket()
	return &S3Watcher{
		Bucket:      bucket,
		BucketName:  os.Getenv("S3_BUCKET_NAME"),
		KnownAssets: make(map[string]bool),
	}
}

func NewS3Bucket() *S3Bucket {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			""),
		Endpoint:         aws.String(os.Getenv("S3_ENDPOINT")),
		Region:           aws.String(os.Getenv("S3_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		log.Fatalf("Failed to create S3 session: %v", err)
	}

	BucketName := os.Getenv("S3_BUCKET_NAME")
	if BucketName == "" {
		log.Fatal("S3_BUCKET_NAME environment variable not set")
	}

	return &S3Bucket{
		S3Client:   s3.New(sess),
		BucketName: BucketName,
	}
}

func (sb *S3Bucket) GetS3Client() *s3.S3 {
	return sb.S3Client
}

func (sb *S3Bucket) GetBucketName() string {
	return sb.BucketName
}

func (sb *S3Bucket) GetNewAssets(appName string) ([]string, error) {
	// First, get all batch directories under the app input directory
	batchDirs, err := sb.GetBatchDirectories(appName)
	if err != nil {
		return nil, err
	}

	var allFiles []string
	for _, batchDir := range batchDirs {
		// Get files from each batch directory
		batchFiles, err := sb.GetS3Objects(appName + "/" + batchDir)
		if err != nil {
			log.Printf("Error getting files from batch %s: %v", batchDir, err)
			continue
		}

		// Add batch directory prefix to each file
		for _, file := range batchFiles {
			if !strings.HasSuffix(file, "/") { // Skip directory markers
				allFiles = append(allFiles, batchDir+"/"+file)
			}
		}
	}

	log.Printf("Found %d total files across %d batch directories for %s", len(allFiles), len(batchDirs), appName)
	return allFiles, nil
}

// GetBatchDirectories returns all batch directory names under the app input directory
func (sb *S3Bucket) GetBatchDirectories(appName string) ([]string, error) {
	prefix := appName
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	log.Printf("DEBUG: Searching for batch directories with prefix: %s in bucket: %s", prefix, sb.BucketName)

	resp, err := sb.S3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(sb.BucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // This groups results by directory
	})
	if err != nil {
		log.Printf("DEBUG: Error calling ListObjectsV2: %v", err)
		return nil, fmt.Errorf("error listing batch directories: %v", err)
	}

	log.Printf("DEBUG: ListObjectsV2 returned %d common prefixes and %d contents", len(resp.CommonPrefixes), len(resp.Contents))

	// Log all common prefixes found
	for i, commonPrefix := range resp.CommonPrefixes {
		log.Printf("DEBUG: CommonPrefix[%d]: %s", i, *commonPrefix.Prefix)
	}

	// Log all contents found
	for i, content := range resp.Contents {
		log.Printf("DEBUG: Content[%d]: %s (size: %d)", i, *content.Key, *content.Size)
	}

	var batchDirs []string
	for _, commonPrefix := range resp.CommonPrefixes {
		// Extract batch directory name from the prefix
		fullPrefix := *commonPrefix.Prefix
		batchDir := strings.TrimPrefix(fullPrefix, prefix)
		batchDir = strings.TrimSuffix(batchDir, "/")
		if batchDir != "" {
			batchDirs = append(batchDirs, batchDir)
		}
	}

	log.Printf("Found %d batch directories for %s: %v", len(batchDirs), appName, batchDirs)
	return batchDirs, nil
}

func (sb *S3Bucket) GetS3Objects(appName string) ([]string, error) {
	if appName != "" && !strings.HasSuffix(appName, "/") {
		appName = appName + "/"
	}

	resp, err := sb.S3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(sb.BucketName),
		Prefix: aws.String(appName),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing objects: %v", err)
	}

	var keys []string
	for _, item := range resp.Contents {
		key := strings.TrimPrefix(*item.Key, appName)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (sb *S3Bucket) UploadFileToBucket(folderPath string, fileName string, content []byte) error {
	var fullKey string
	if folderPath != "" {
		fullKey = strings.TrimSuffix(folderPath, "/") + "/" + fileName
	} else {
		fullKey = fileName
	}

	contentReader := bytes.NewReader(content)

	_, err := sb.S3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(sb.BucketName),
		Key:    aws.String(fullKey),
		Body:   contentReader,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %v", err)
	}

	log.Printf("Successfully uploaded content to s3://%s/%s", sb.BucketName, fullKey)
	return nil
}

func (sb *S3Bucket) GetObjectMetadata(objectKey string) (*ObjectMetadata, error) {
	// Use HeadObject to get metadata without downloading the file
	result, err := sb.S3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(sb.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %v", err)
	}

	metadata := &ObjectMetadata{
		Size:         *result.ContentLength,
		LastModified: *result.LastModified,
	}

	return metadata, nil
}
