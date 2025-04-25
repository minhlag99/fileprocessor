// Package storage provides interfaces and implementations for different storage providers
package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GoogleCloudStorage implements StorageProvider interface for Google Cloud Storage
type GoogleCloudStorage struct {
	client     *storage.Client
	bucketName string
	prefix     string
}

// NewGoogleCloudStorage creates a new Google Cloud Storage provider
func NewGoogleCloudStorage() *GoogleCloudStorage {
	return &GoogleCloudStorage{}
}

// Initialize sets up the Google Cloud Storage with configuration
func (g *GoogleCloudStorage) Initialize(config map[string]string) error {
	var opts []option.ClientOption

	// Check if credential file is provided
	if credFile, ok := config["credentialFile"]; ok && credFile != "" {
		opts = append(opts, option.WithCredentialsFile(credFile))
	}

	// Create storage client
	client, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		return fmt.Errorf("failed to create Google Cloud Storage client: %w", err)
	}
	g.client = client

	// Get bucket name (required)
	bucketName, ok := config["bucket"]
	if !ok || bucketName == "" {
		return fmt.Errorf("bucket is required for Google Cloud Storage")
	}
	g.bucketName = bucketName

	// Optional prefix (folder) within the bucket
	if prefix, ok := config["prefix"]; ok {
		g.prefix = prefix
	}

	return nil
}

// Store saves a file to Google Cloud Storage
func (g *GoogleCloudStorage) Store(ctx context.Context, name string, content io.Reader, size int64, metadata map[string]string) (string, error) {
	// Generate a unique object name
	timestamp := time.Now().UnixNano()
	objectName := fmt.Sprintf("%s%d-%s", g.prefix, timestamp, name)

	// Get bucket and object handles
	bucket := g.client.Bucket(g.bucketName)
	obj := bucket.Object(objectName)
	writer := obj.NewWriter(ctx)

	// Set metadata
	writer.Metadata = metadata

	// Write content to GCS
	if _, err := io.Copy(writer, content); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write file content to GCS: %w", err)
	}

	// Close the writer to finalize the upload
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize file upload to GCS: %w", err)
	}

	return objectName, nil
}

// Retrieve gets a file from Google Cloud Storage
func (g *GoogleCloudStorage) Retrieve(ctx context.Context, id string) (io.ReadCloser, map[string]string, error) {
	// Get bucket and object handles
	bucket := g.client.Bucket(g.bucketName)
	obj := bucket.Object(id)

	// Get object attributes to retrieve metadata
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object attributes from GCS: %w", err)
	}

	// Open reader
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file from GCS: %w", err)
	}

	return reader, attrs.Metadata, nil
}

// Delete removes a file from Google Cloud Storage
func (g *GoogleCloudStorage) Delete(ctx context.Context, id string) error {
	// Get bucket and object handles
	bucket := g.client.Bucket(g.bucketName)
	obj := bucket.Object(id)

	// Delete the object
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete file from GCS: %w", err)
	}

	return nil
}

// List returns a list of files in Google Cloud Storage
func (g *GoogleCloudStorage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	// Combine prefix with the storage prefix
	fullPrefix := g.prefix + prefix

	// Get bucket handle
	bucket := g.client.Bucket(g.bucketName)

	// List objects with the specified prefix
	it := bucket.Objects(ctx, &storage.Query{Prefix: fullPrefix})

	var files []FileInfo
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list files from GCS: %w", err)
		}

		// Create FileInfo object
		file := FileInfo{
			ID:          attrs.Name,
			Name:        filepath.Base(attrs.Name),
			Size:        attrs.Size,
			ContentType: attrs.ContentType,
			ModifiedAt:  attrs.Updated.Unix(),
			Metadata:    attrs.Metadata,
		}

		files = append(files, file)
	}

	return files, nil
}

// GetSignedURL returns a pre-signed URL for temporary access to a file in Google Cloud Storage
func (g *GoogleCloudStorage) GetSignedURL(ctx context.Context, id string, expiryMinutes int, operation string) (string, error) {
	// Set expiration time
	expires := time.Now().Add(time.Duration(expiryMinutes) * time.Minute)

	// Generate signed URL
	opts := &storage.SignedURLOptions{
		Expires: expires,
	}

	switch operation {
	case "read":
		opts.Method = "GET"
	case "write":
		opts.Method = "PUT"
	default:
		opts.Method = "GET"
	}

	// Generate signed URL using the bucket's SignedURL method
	url, err := storage.SignedURL(g.bucketName, id, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}