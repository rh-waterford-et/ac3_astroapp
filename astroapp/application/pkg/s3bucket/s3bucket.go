package s3bucket

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3BucketInterface interface {
	//InitializeKnownAssets(appName string)
	GetNewAssets(appName string) ([]string, error)
	GetS3Objects(appName string) ([]string, error)
	GetS3Client() *s3.S3
	GetBucketName() string
	UploadFileToBucket(folderPath string, fileName string, content []byte) error
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

/*
	 func (sb *S3Bucket) InitializeKnownAssets(appName string) {
		assets, err := sb.GetS3Objects(appName)
		if err != nil {
			log.Printf("Error getting initial assets: %v", err)
			return
		}
		log.Printf("Initial assets in '%s': %v", appName, assets)
	}
*/
func (sb *S3Bucket) GetNewAssets(appName string) ([]string, error) {
	currentAssets, err := sb.GetS3Objects(appName)
	if err != nil {
		return nil, err
	}

	var newAssets []string
	for _, asset := range currentAssets {
		if !strings.HasSuffix(asset, "/") { // Skip directory markers
			newAssets = append(newAssets, asset)
		}
	}
	return newAssets, nil
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
