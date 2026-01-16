package aws

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

type S3Bucket struct {
	BucketName string
	Region     string
	S3Client   *s3.Client
}

func UploadToS3(file multipart.File, key string, contentType string) (string, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("!!!!!..........Error loading .env file.........!!!!!!")
	}

	s3Bucket := S3Bucket{
		BucketName: os.Getenv("AWS_S3_BUCKET"),
		Region:     os.Getenv("AWS_REGION"),
	}

	if s3Bucket.BucketName == "" || s3Bucket.Region == "" {
		fmt.Println("Usage: make sure aws s3 bucket and region are provided")
		os.Exit(1)
	}

	fmt.Println("BucketName::::", s3Bucket.BucketName)

	ctx := context.Background()

	//Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s3Bucket.Region))
	if err != nil {
		panic(fmt.Sprintf("Failed to load aws config: %v", err))
	}

	s3Bucket.S3Client = s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(s3Bucket.S3Client)

	result, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket.BucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to s3 %v", err)
	}

	return result.Location, nil
}
