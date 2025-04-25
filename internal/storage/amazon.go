// Package storage provides interfaces and implementations for different storage providers
package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// AmazonS3Storage implements StorageProvider interface for Amazon S3
type AmazonS3Storage struct {
	bucket   string
	prefix   string
	s3Client *s3.S3
	uploader *s3manager.Uploader
}

// NewAmazonS3Storage creates a new Amazon S3 storage provider
func NewAmazonS3Storage() *AmazonS3Storage {
	return &AmazonS3Storage{}
}

// Initialize sets up the Amazon S3 storage with configuration
func (a *AmazonS3Storage) Initialize(config map[string]string) error {
	// Required configuration
	region, ok := config["region"]
	if !ok || region == "" {
		return fmt.Errorf("region is required for Amazon S3 storage")
	}

	bucket, ok := config["bucket"]
	if !ok || bucket == "" {
		return fmt.Errorf("bucket is required for Amazon S3 storage")
	}
	a.bucket = bucket

	// Optional prefix (folder) within the bucket
	if prefix, ok := config["prefix"]; ok {
		a.prefix = prefix
	}

	// Create AWS session
	var sess *session.Session
	var err error

	// Check if credentials are provided or using env/instance profile
	accessKey, hasAccessKey := config["accessKey"]
	secretKey, hasSecretKey := config["secretKey"]

	if hasAccessKey && hasSecretKey {
		// Use provided credentials
		sess, err = session.NewSession(&aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
		})
	} else {
		// Use environment variables or instance profile
		sess, err = session.NewSession(&aws.Config{
			Region: aws.String(region),
		})
	}

	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create S3 client and uploader
	a.s3Client = s3.New(sess)
	a.uploader = s3manager.NewUploader(sess)

	return nil
}

// Store saves a file to Amazon S3
func (a *AmazonS3Storage) Store(ctx context.Context, name string, content io.Reader, size int64, metadata map[string]string) (string, error) {
	// Generate a unique key for the file
	timestamp := time.Now().UnixNano()
	key := fmt.Sprintf("%s%d-%s", a.prefix, timestamp, name)

	// Convert metadata to S3 format
	s3Metadata := make(map[string]*string)
	for k, v := range metadata {
		s3Metadata[k] = aws.String(v)
	}

	// Upload the file
	_, err := a.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:   aws.String(a.bucket),
		Key:      aws.String(key),
		Body:     content,
		Metadata: s3Metadata,
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return key, nil
}

// Retrieve gets a file from Amazon S3
func (a *AmazonS3Storage) Retrieve(ctx context.Context, id string) (io.ReadCloser, map[string]string, error) {
	// Get object from S3
	output, err := a.s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(id),
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve file from S3: %w", err)
	}

	// Convert S3 metadata to map[string]string
	metadata := make(map[string]string)
	for k, v := range output.Metadata {
		if v != nil {
			metadata[k] = *v
		}
	}

	return output.Body, metadata, nil
}

// Delete removes a file from Amazon S3
func (a *AmazonS3Storage) Delete(ctx context.Context, id string) error {
	_, err := a.s3Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(id),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}

	return nil
}

// List returns a list of files in Amazon S3
func (a *AmazonS3Storage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	// Combine prefix with the storage prefix
	fullPrefix := a.prefix + prefix

	// List objects in the bucket with the specified prefix
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(a.bucket),
		Prefix: aws.String(fullPrefix),
	}

	var files []FileInfo
	err := a.s3Client.ListObjectsV2PagesWithContext(ctx, input, func(output *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range output.Contents {
			// Get object metadata
			objOutput, err := a.s3Client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(a.bucket),
				Key:    obj.Key,
			})

			if err != nil {
				continue
			}

			// Convert S3 metadata to map[string]string
			metadata := make(map[string]string)
			for k, v := range objOutput.Metadata {
				if v != nil {
					metadata[k] = *v
				}
			}

			// Create FileInfo object
			file := FileInfo{
				ID:   *obj.Key,
				Name: filepath.Base(*obj.Key),
				Size: *obj.Size,
				ContentType: func() string {
					if objOutput.ContentType != nil {
						return *objOutput.ContentType
					}
					return ""
				}(),
				ModifiedAt: obj.LastModified.Unix(),
				Metadata:   metadata,
			}

			files = append(files, file)
		}

		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files from S3: %w", err)
	}

	return files, nil
}

// GetSignedURL returns a pre-signed URL for temporary access to a file in Amazon S3
func (a *AmazonS3Storage) GetSignedURL(ctx context.Context, id string, expiryMinutes int, operation string) (string, error) {
	req, _ := a.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(id),
	})

	// Pre-sign the request with the specified expiry time
	duration := time.Duration(expiryMinutes) * time.Minute
	url, err := req.Presign(duration)

	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed URL: %w", err)
	}

	return url, nil
}