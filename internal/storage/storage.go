// Package storage provides interfaces and implementations for different storage providers
package storage

import (
	"context"
	"io"
)

// StorageProvider defines the interface for all storage implementations
// This includes local storage and cloud providers like Amazon S3 and Google Cloud Storage
type StorageProvider interface {
	// Initialize sets up the storage provider with configuration
	Initialize(config map[string]string) error
	
	// Store saves a file to the storage provider
	// Returns the unique identifier of the stored file
	Store(ctx context.Context, name string, content io.Reader, size int64, metadata map[string]string) (string, error)
	
	// Retrieve gets a file from the storage provider
	Retrieve(ctx context.Context, id string) (io.ReadCloser, map[string]string, error)
	
	// Delete removes a file from the storage provider
	Delete(ctx context.Context, id string) error
	
	// List returns a list of files in the storage provider that match the given prefix
	List(ctx context.Context, prefix string) ([]FileInfo, error)
	
	// GetSignedURL returns a pre-signed URL for temporary access to a file
	// Useful for generating temporary download/upload links
	GetSignedURL(ctx context.Context, id string, expiryMinutes int, operation string) (string, error)
}

// FileInfo represents metadata about a file in storage
type FileInfo struct {
	ID          string
	Name        string
	Size        int64
	ContentType string
	ModifiedAt  int64
	Metadata    map[string]string
}

// Config represents common configuration for storage providers
type Config struct {
	// Provider type (e.g., "local", "s3", "gcs")
	Provider string `json:"provider"`
	
	// Base path/bucket for storage
	BasePath string `json:"base_path"`
	
	// Additional provider-specific configurations
	Options map[string]string `json:"options"`
}