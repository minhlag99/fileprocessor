// Package models provides data structures for the file processing application
package models

import (
	"time"
)

// File represents a file in the system
type File struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Size        int64             `json:"size"`
	ContentType string            `json:"contentType"`
	UploadedAt  time.Time         `json:"uploadedAt"`
	StorageType string            `json:"storageType"` // "local", "s3", "gcs", etc.
	StorageID   string            `json:"storageId"`   // ID in the storage system
	Metadata    map[string]string `json:"metadata"`
}

// ProcessedFile represents a file that has been processed
type ProcessedFile struct {
	File         *File              `json:"file"`
	Summary      string             `json:"summary"`
	PreviewURL   string             `json:"previewUrl,omitempty"`
	ProcessedAt  time.Time          `json:"processedAt"`
	ProcessStats map[string]string  `json:"processStats,omitempty"`
}

// FileUploadRequest represents a request to upload a file
type FileUploadRequest struct {
	Filename    string            `json:"filename"`
	ContentType string            `json:"contentType"`
	StorageType string            `json:"storageType,omitempty"` // Default to local if empty
	ProcessFile bool              `json:"processFile"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StorageConfig represents configuration for a storage provider
type StorageConfig struct {
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Options map[string]string `json:"options"`
}

// APIResponse is a generic API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}