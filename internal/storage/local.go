// Package storage provides interfaces and implementations for different storage providers
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage implements StorageProvider interface for local filesystem storage
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local storage provider
func NewLocalStorage() *LocalStorage {
	return &LocalStorage{}
}

// Initialize sets up the local storage with configuration
func (l *LocalStorage) Initialize(config map[string]string) error {
	if path, ok := config["basePath"]; ok && path != "" {
		l.basePath = path
	} else {
		l.basePath = "./storage" // Default storage path
	}

	// Create base directory if it doesn't exist
	if _, err := os.Stat(l.basePath); os.IsNotExist(err) {
		if err := os.MkdirAll(l.basePath, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}
	return nil
}

// Store saves a file to local storage
func (l *LocalStorage) Store(ctx context.Context, name string, content io.Reader, size int64, metadata map[string]string) (string, error) {
	// Generate unique ID based on timestamp and name
	id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), strings.Replace(name, " ", "_", -1))

	// Create file path
	filePath := filepath.Join(l.basePath, id)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write content to file
	if _, err := io.Copy(file, content); err != nil {
		return "", fmt.Errorf("failed to write file content: %w", err)
	}

	// Store metadata in a separate file if needed
	if len(metadata) > 0 {
		metaFile, err := os.Create(filePath + ".meta")
		if err == nil {
			defer metaFile.Close()
			for k, v := range metadata {
				metaFile.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			}
		}
	}

	return id, nil
}

// Retrieve gets a file from local storage
func (l *LocalStorage) Retrieve(ctx context.Context, id string) (io.ReadCloser, map[string]string, error) {
	filePath := filepath.Join(l.basePath, id)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Read metadata if exists
	metadata := make(map[string]string)
	metaPath := filePath + ".meta"
	if metaData, err := os.ReadFile(metaPath); err == nil {
		lines := strings.Split(string(metaData), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				metadata[parts[0]] = parts[1]
			}
		}
	}

	return file, metadata, nil
}

// Delete removes a file from local storage
func (l *LocalStorage) Delete(ctx context.Context, id string) error {
	filePath := filepath.Join(l.basePath, id)

	// Delete file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file if exists
	metaPath := filePath + ".meta"
	if _, err := os.Stat(metaPath); err == nil {
		os.Remove(metaPath)
	}

	return nil
}

// List returns a list of files in local storage
func (l *LocalStorage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(l.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and metadata files
		if info.IsDir() || strings.HasSuffix(info.Name(), ".meta") {
			return nil
		}

		// Skip files not matching prefix
		if prefix != "" && !strings.HasPrefix(info.Name(), prefix) {
			return nil
		}

		relPath, _ := filepath.Rel(l.basePath, path)

		// Read metadata if exists
		metadata := make(map[string]string)
		metaPath := path + ".meta"
		if metaData, err := os.ReadFile(metaPath); err == nil {
			lines := strings.Split(string(metaData), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					metadata[parts[0]] = parts[1]
				}
			}
		}
		// Extract the original filename from metadata if available
		fileName := info.Name()
		if originalName, ok := metadata["filename"]; ok && originalName != "" {
			fileName = originalName
		}

		files = append(files, FileInfo{
			ID:          relPath,
			Name:        fileName,
			Size:        info.Size(),
			ContentType: metadata["contentType"],
			ModifiedAt:  info.ModTime().Unix(),
			Metadata:    metadata,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// GetBasePath returns the base path of this storage provider
func (l *LocalStorage) GetBasePath() string {
	return l.basePath
}

// GetSignedURL returns a file:// URL for local files
func (l *LocalStorage) GetSignedURL(ctx context.Context, id string, expiryMinutes int, operation string) (string, error) {
	filePath := filepath.Join(l.basePath, id)

	// For local storage, we just return a file:// URL
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	// Convert to file:// URL format
	return "file://" + strings.Replace(absPath, "\\", "/", -1), nil
}
