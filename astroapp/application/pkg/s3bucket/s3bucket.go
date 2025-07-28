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
	CreateDirectory(directoryPath string) error
	GetObjectMetadata(objectKey string) (*ObjectMetadata, error)
	DeleteDirectory(directoryPath string) error
	DeleteFile(fileKey string) error
	DownloadFile(objectKey string) ([]byte, error)
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

	//log.Printf("DEBUG: Searching for batch directories with prefix: %s in bucket: %s", prefix, sb.BucketName)

	resp, err := sb.S3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(sb.BucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // This groups results by directory
	})
	if err != nil {
		log.Printf("DEBUG: Error calling ListObjectsV2: %v", err)
		return nil, fmt.Errorf("error listing batch directories: %v", err)
	}

	// Use a map to collect unique batch directories
	batchDirMap := make(map[string]bool)

	// First, process CommonPrefixes (the traditional way)
	for _, commonPrefix := range resp.CommonPrefixes {
		// Extract batch directory name from the prefix
		fullPrefix := *commonPrefix.Prefix
		batchDir := strings.TrimPrefix(fullPrefix, prefix)
		batchDir = strings.TrimSuffix(batchDir, "/")
		if batchDir != "" {
			batchDirMap[batchDir] = true
		}
	}

	// Second, process file paths to extract directories (fallback method)
	for _, content := range resp.Contents {
		key := *content.Key
		// Skip the root directory marker
		if key == prefix {
			continue
		}

		// Extract directory name from file path
		relativePath := strings.TrimPrefix(key, prefix)
		if strings.Contains(relativePath, "/") {
			// This is a file in a subdirectory
			batchDir := strings.Split(relativePath, "/")[0]
			if batchDir != "" {
				batchDirMap[batchDir] = true
			}
		}
	}

	// Convert map to slice
	var batchDirs []string
	for batchDir := range batchDirMap {
		batchDirs = append(batchDirs, batchDir)
	}

	//log.Printf("Found %d batch directories for %s: %v", len(batchDirs), appName, batchDirs)
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

	//log.Printf("Successfully uploaded content to s3://%s/%s", sb.BucketName, fullKey)
	return nil
}

func (sb *S3Bucket) CreateDirectory(directoryPath string) error {
	// Create directory marker by uploading empty object with trailing slash
	directoryKey := strings.TrimSuffix(directoryPath, "/") + "/"

	_, err := sb.S3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(sb.BucketName),
		Key:    aws.String(directoryKey),
		Body:   bytes.NewReader([]byte{}),
	})
	if err != nil {
		return fmt.Errorf("failed to create directory %s in S3: %v", directoryPath, err)
	}

	log.Printf("Successfully created directory s3://%s/%s", sb.BucketName, directoryKey)
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

func (sb *S3Bucket) DeleteDirectory(directoryPath string) error {
	// Ensure the directory path ends with a slash
	if !strings.HasSuffix(directoryPath, "/") {
		directoryPath = directoryPath + "/"
	}

	// List all objects in the directory
	objects, err := sb.S3Client.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(sb.BucketName),
		Prefix: aws.String(directoryPath),
	})

	if err != nil {
		return fmt.Errorf("failed to list objects in directory %s: %v", directoryPath, err)
	}

	// If no objects found, directory is already empty or doesn't exist
	if len(objects.Contents) == 0 {
		return nil
	}

	// Prepare batch delete request
	var deleteObjects []*s3.ObjectIdentifier
	for _, object := range objects.Contents {
		deleteObjects = append(deleteObjects, &s3.ObjectIdentifier{
			Key: object.Key,
		})
	}

	// Delete objects in batches (S3 allows up to 1000 objects per batch)
	batchSize := 1000
	for i := 0; i < len(deleteObjects); i += batchSize {
		end := i + batchSize
		if end > len(deleteObjects) {
			end = len(deleteObjects)
		}

		batchObjects := deleteObjects[i:end]

		_, err := sb.S3Client.DeleteObjects(&s3.DeleteObjectsInput{
			Bucket: aws.String(sb.BucketName),
			Delete: &s3.Delete{
				Objects: batchObjects,
			},
		})

		if err != nil {
			return fmt.Errorf("failed to delete objects in directory %s: %v", directoryPath, err)
		}
	}

	return nil
}

func (sb *S3Bucket) DeleteFile(fileKey string) error {
	// Delete the specific file from S3
	_, err := sb.S3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(sb.BucketName),
		Key:    aws.String(fileKey),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file %s: %v", fileKey, err)
	}

	return nil
}

func (sb *S3Bucket) DownloadFile(objectKey string) ([]byte, error) {
	log.Printf("S3 DownloadFile: Attempting to download key '%s' from bucket '%s'", objectKey, sb.BucketName)

	result, err := sb.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(sb.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file %s: %v", objectKey, err)
	}
	defer result.Body.Close()

	content := bytes.Buffer{}
	_, err = content.ReadFrom(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %v", err)
	}

	return content.Bytes(), nil
}
