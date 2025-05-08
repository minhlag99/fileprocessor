// Package handlers provides HTTP handlers for file operations
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/fileprocessor/internal/config"
	"github.com/example/fileprocessor/internal/models"
	"github.com/example/fileprocessor/internal/processors"
	"github.com/example/fileprocessor/internal/storage"
	"github.com/gorilla/mux"
)

// FileHandler handles file operations
type FileHandler struct {
	defaultStorage storage.Provider
}

// NewFileHandlerWithProvider creates a new file handler with a specified storage provider
func NewFileHandlerWithProvider(defaultStorage storage.Provider) *FileHandler {
	return &FileHandler{
		defaultStorage: defaultStorage,
	}
}

// NewFileHandler creates a new file handler with the default local storage provider
func NewFileHandler() (*FileHandler, error) {
	// Create a default local storage provider
	storageConfig := map[string]string{
		"basePath": config.AppConfig.Storage.Local.BasePath,
	}

	defaultStorage, err := storage.CreateProvider("local", storageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create default storage provider: %w", err)
	}

	return &FileHandler{
		defaultStorage: defaultStorage,
	}, nil
}

// testCloudProviderAvailability attempts to initialize cloud providers with empty configs
// to check if they're available, and marks them as unavailable if they're not
func (h *FileHandler) testCloudProviderAvailability() {
	// Test Google Cloud Storage availability
	_, err := storage.CreateProvider("google", map[string]string{})
	if err != nil {
		log.Printf("Google Cloud Storage will be unavailable: %v", err)
		// Error already marked the provider as unavailable in the factory
	}

	// Test Amazon S3 availability
	_, err = storage.CreateProvider("s3", map[string]string{})
	if err != nil {
		log.Printf("Amazon S3 will be unavailable: %v", err)
		// Error already marked the provider as unavailable in the factory
	}
}

// GetStorageProviderStatus returns the status of all storage providers
func (h *FileHandler) GetStorageProviderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get status for each provider type
	localAvailable, localReason := storage.IsProviderAvailable("local")
	s3Available, s3Reason := storage.IsProviderAvailable("s3")
	googleAvailable, googleReason := storage.IsProviderAvailable("google")

	status := map[string]interface{}{
		"local": map[string]interface{}{
			"available": localAvailable,
			"reason":    localReason,
		},
		"s3": map[string]interface{}{
			"available": s3Available,
			"reason":    s3Reason,
		},
		"google": map[string]interface{}{
			"available": googleAvailable,
			"reason":    googleReason,
		},
	}

	response := models.APIResponse{
		Success: true,
		Data:    status,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// UploadFile handles file upload requests
func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 32MB files by default)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		sendJSONError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get file from the request
	file, header, err := r.FormFile("file")
	if err != nil {
		sendJSONError(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get storage parameters
	storageType := r.FormValue("storageType")
	if storageType == "" {
		storageType = "local"
	}

	// Check if the requested storage provider is available
	if storageType != "local" {
		available, reason := storage.IsProviderAvailable(storageType)
		if !available {
			sendJSONError(w, fmt.Sprintf("Storage provider '%s' is unavailable: %s", storageType, reason), http.StatusBadRequest)
			return
		}
	}

	// Get processing option
	processFile := r.FormValue("processFile") == "true"

	// Setup metadata
	metadata := make(map[string]string)
	metadata["filename"] = header.Filename
	metadata["contentType"] = header.Header.Get("Content-Type")
	if metadata["contentType"] == "" {
		metadata["contentType"] = "application/octet-stream"
	}

	// Get appropriate storage provider
	var provider storage.Provider
	if storageType == "local" {
		provider = h.defaultStorage
	} else {
		// Extract provider configuration from request
		config := extractStorageConfig(r, storageType)
		var err error
		provider, err = storage.CreateProvider(storageType, config)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to create storage provider: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Store the file
	id, err := provider.Store(r.Context(), header.Filename, file, header.Size, metadata)
	if err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to store file: %v", err), http.StatusInternalServerError)
		return
	}

	// Create file model
	fileModel := &models.File{
		ID:          id,
		Name:        header.Filename,
		Size:        header.Size,
		ContentType: metadata["contentType"],
		UploadedAt:  time.Now(),
		StorageType: storageType,
		StorageID:   id,
		Metadata:    metadata,
	}

	// Process file if requested
	var processedFile *models.ProcessedFile
	if processFile {
		// Create a task ID for tracking
		taskID := fmt.Sprintf("process-%s-%d", id, time.Now().UnixNano())

		// Create a task function
		processFn := func() (*processors.ProcessResult, error) {
			// Get file content
			reader, _, err := provider.Retrieve(r.Context(), fileModel.StorageID)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve file: %w", err)
			}
			defer reader.Close()

			// Get processor for this file type
			processor := processors.GetProcessor(fileModel.ContentType, filepath.Ext(fileModel.Name))
			if processor == nil {
				return nil, fmt.Errorf("no processor available for file type: %s", fileModel.ContentType)
			}

			// Process the file with progress reporting
			options := processors.ProcessOptions{
				GeneratePreview: true,
				ExtractMetadata: true,
				MaxPreviewSize:  1024 * 10, // 10KB
			}			// Send processing started notification via WebSocket
			DefaultWebSocketHub.Broadcast("processing_started", map[string]interface{}{
				"taskId": taskID,
				"file":   fileModel,
			})
			
			// Create a context with cancellation for the progress reporter
			progressCtx, cancelProgress := context.WithCancel(context.Background())
			defer cancelProgress()
					// Report progress updates with ability to stop when processing completes
			go func() {
				progress := 0
				ticker := time.NewTicker(500 * time.Millisecond)
				defer ticker.Stop()

				for progress < 100 { // Changed to 100 to ensure completion
					select {
					case <-ticker.C:
						progress += 5 // Smaller increments for smoother progress
						log.Printf("Sending progress update for task %s: %d%%", taskID, progress)
						DefaultWebSocketHub.SendTaskUpdate(taskID, "processing_progress", map[string]interface{}{
							"progress": progress,
							"file":     fileModel,
						})
					case <-progressCtx.Done():
						// Processing completed or failed, ensure we send 100% complete
						if progress < 100 {
							log.Printf("Sending final progress update for task %s: 100%%", taskID)
							DefaultWebSocketHub.SendTaskUpdate(taskID, "processing_progress", map[string]interface{}{
								"progress": 100,
								"file":     fileModel,
							})
						}
						return
					case <-r.Context().Done():
						// Request was cancelled
						return
					}
				}
			}()

			// Do the actual processing
			result, err := processor.Process(r.Context(), reader, fileModel.Name, options)			// Cancel progress reporting goroutine immediately before sending completion
			cancelProgress()

			// Send completion notification via WebSocket
			if err != nil {
				log.Printf("Processing failed for file %s: %v", fileModel.Name, err)
				DefaultWebSocketHub.SendTaskUpdate(taskID, "processing_failed", map[string]interface{}{
					"error": err.Error(),
					"file":  fileModel,
				})
			} else {
				log.Printf("Processing completed for file %s", fileModel.Name)
				DefaultWebSocketHub.SendTaskUpdate(taskID, "processing_completed", map[string]interface{}{
					"file":    fileModel,
					"summary": result.Summary,
				})
			}

			return result, err
		}

		// Create and submit the task
		task := processors.NewTask(taskID, processFn)
		processors.Submit(task)

		// Wait briefly for quick tasks to complete
		select {
		case result := <-task.Result:
			// Task completed successfully
			processedFile = createProcessedFile(fileModel, result)
		case err := <-task.Error:
			// Task failed
			log.Printf("Warning: Failed to process file %s: %v\n", fileModel.Name, err)
		case <-time.After(200 * time.Millisecond):
			// Task is still running, continue without waiting
			// The client will receive updates via WebSocket
			response := models.APIResponse{
				Success: true,
				Message: "File uploaded successfully. Processing in background.",
				Data: map[string]interface{}{
					"file":   fileModel,
					"taskId": taskID,
					"status": "processing",
				},
			}
			sendJSONResponse(w, response, http.StatusOK)
			return
		}
	}

	// Send response
	response := models.APIResponse{
		Success: true,
		Message: "File uploaded successfully",
	}

	if processedFile != nil {
		response.Data = processedFile
	} else {
		response.Data = fileModel
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// DownloadFile handles file download requests
func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from the request
	fileID := r.URL.Query().Get("id")
	storageType := r.URL.Query().Get("storageType")
	if fileID == "" {
		sendJSONError(w, "File ID is required", http.StatusBadRequest)
		return
	}

	if storageType == "" {
		storageType = "local"
	}

	// Check if the requested storage provider is available
	if storageType != "local" {
		available, reason := storage.IsProviderAvailable(storageType)
		if !available {
			sendJSONError(w, fmt.Sprintf("Storage provider '%s' is unavailable: %s", storageType, reason), http.StatusBadRequest)
			return
		}
	}

	// Get appropriate storage provider
	var provider storage.Provider
	if storageType == "local" {
		provider = h.defaultStorage
	} else {
		// Extract provider configuration from request
		config := extractStorageConfig(r, storageType)
		var err error
		provider, err = storage.CreateProvider(storageType, config)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to create storage provider: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Get the file from storage
	reader, metadata, err := provider.Retrieve(r.Context(), fileID)
	if err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to retrieve file: %v", err), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Set headers for the response
	filename := metadata["filename"]
	if filename == "" {
		filename = filepath.Base(fileID)
	}
	contentType := metadata["contentType"]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Stream the file to the client
	if _, err := io.Copy(w, reader); err != nil {
		// Can't send an error response here as we've already written to the response
		fmt.Printf("Error streaming file: %v\n", err)
	}
}

// ListFiles handles requests to list files
func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from the request
	storageType := r.URL.Query().Get("storageType")
	prefix := r.URL.Query().Get("prefix")

	if storageType == "" {
		storageType = "local"
	}

	// Check if the requested storage provider is available
	if storageType != "local" {
		available, reason := storage.IsProviderAvailable(storageType)
		if !available {
			sendJSONError(w, fmt.Sprintf("Storage provider '%s' is unavailable: %s", storageType, reason), http.StatusBadRequest)
			return
		}
	}

	// Get appropriate storage provider
	var provider storage.Provider
	if storageType == "local" {
		provider = h.defaultStorage
	} else {
		// Extract provider configuration from request
		config := extractStorageConfig(r, storageType)
		var err error
		provider, err = storage.CreateProvider(storageType, config)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to create storage provider: %v", err), http.StatusBadRequest)
			return
		}
	}

	// List files
	files, err := provider.List(r.Context(), prefix)
	if err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to list files: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to file models
	var fileModels []*models.File
	for _, file := range files {
		fileModel := &models.File{
			ID:          file.ID,
			Name:        file.Name,
			Size:        file.Size,
			ContentType: file.ContentType,
			StorageType: storageType,
			StorageID:   file.ID,
			Metadata:    file.Metadata,
		}

		if timeStr, ok := file.Metadata["uploadedAt"]; ok {
			if uploadTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
				fileModel.UploadedAt = uploadTime
			}
		}

		if fileModel.UploadedAt.IsZero() {
			fileModel.UploadedAt = time.Unix(file.ModifiedAt, 0)
		}

		fileModels = append(fileModels, fileModel)
	}

	// Send response
	response := models.APIResponse{
		Success: true,
		Data:    fileModels,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// GetSignedURL handles requests to get pre-signed URLs for files
func (h *FileHandler) GetSignedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from the request
	fileID := r.URL.Query().Get("id")
	storageType := r.URL.Query().Get("storageType")
	operation := r.URL.Query().Get("operation") // "read" or "write"
	expiryStr := r.URL.Query().Get("expiry")    // in minutes

	if fileID == "" {
		sendJSONError(w, "File ID is required", http.StatusBadRequest)
		return
	}

	if storageType == "" {
		storageType = "local"
	}

	// Check if the requested storage provider is available
	if storageType != "local" {
		available, reason := storage.IsProviderAvailable(storageType)
		if !available {
			sendJSONError(w, fmt.Sprintf("Storage provider '%s' is unavailable: %s", storageType, reason), http.StatusBadRequest)
			return
		}
	}

	if operation == "" {
		operation = "read"
	}

	expiry := 60 // default to 60 minutes
	if expiryStr != "" {
		if parsedExpiry, err := fmt.Sscanf(expiryStr, "%d", &expiry); err != nil || parsedExpiry != 1 {
			expiry = 60
		}
	}

	// Get appropriate storage provider
	var provider storage.Provider
	if storageType == "local" {
		provider = h.defaultStorage
	} else {
		// Extract provider configuration from request
		config := extractStorageConfig(r, storageType)
		var err error
		provider, err = storage.CreateProvider(storageType, config)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to create storage provider: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Get signed URL
	url, err := provider.GetSignedURL(r.Context(), fileID, expiry, operation)
	if err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to get signed URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	response := models.APIResponse{
		Success: true,
		Data: map[string]string{
			"url":       url,
			"expiresIn": fmt.Sprintf("%d minutes", expiry),
		},
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// DeleteFile handles requests to delete files
func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from the request
	fileID := r.URL.Query().Get("id")
	storageType := r.URL.Query().Get("storageType")

	if fileID == "" {
		sendJSONError(w, "File ID is required", http.StatusBadRequest)
		return
	}

	if storageType == "" {
		storageType = "local"
	}

	// Check if the requested storage provider is available
	if storageType != "local" {
		available, reason := storage.IsProviderAvailable(storageType)
		if !available {
			sendJSONError(w, fmt.Sprintf("Storage provider '%s' is unavailable: %s", storageType, reason), http.StatusBadRequest)
			return
		}
	}

	// Get appropriate storage provider
	var provider storage.Provider
	if storageType == "local" {
		provider = h.defaultStorage
	} else {
		// Extract provider configuration from request
		config := extractStorageConfig(r, storageType)
		var err error
		provider, err = storage.CreateProvider(storageType, config)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to create storage provider: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Delete the file
	if err := provider.Delete(r.Context(), fileID); err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to delete file: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	response := models.APIResponse{
		Success: true,
		Message: "File deleted successfully",
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// MediaPreviewHandler serves media preview for files
func (h *FileHandler) MediaPreviewHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the file ID from the URL parameters
	vars := mux.Vars(r)
	fileID := vars["id"]

	if fileID == "" {
		sendJSONError(w, "File ID is required", http.StatusBadRequest)
		return
	}

	// Get storage parameters
	storageType := r.URL.Query().Get("storageType")
	if storageType == "" {
		storageType = "local"
	}

	// Check if the requested storage provider is available
	if storageType != "local" {
		available, reason := storage.IsProviderAvailable(storageType)
		if !available {
			sendJSONError(w, fmt.Sprintf("Storage provider '%s' is unavailable: %s", storageType, reason), http.StatusBadRequest)
			return
		}
	}

	// Get appropriate storage provider
	var provider storage.Provider
	if storageType == "local" {
		provider = h.defaultStorage
	} else {
		// Extract provider configuration from request
		config := extractStorageConfig(r, storageType)
		var err error
		provider, err = storage.CreateProvider(storageType, config)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to create storage provider: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Try to get the preview version first (by convention, preview files have _preview suffix)
	previewID := fileID + "_preview"
	reader, metadata, err := provider.Retrieve(r.Context(), previewID)

	// If preview doesn't exist, fall back to the original file
	if err != nil {
		log.Printf("Preview not found for %s, falling back to original file: %v", fileID, err)
		reader, metadata, err = provider.Retrieve(r.Context(), fileID)
		if err != nil {
			sendJSONError(w, fmt.Sprintf("Failed to retrieve file: %v", err), http.StatusInternalServerError)
			return
		}
	}
	defer reader.Close()

	// Set headers for the response
	filename := metadata["filename"]
	if filename == "" {
		filename = filepath.Base(fileID)
	}

	contentType := metadata["contentType"]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)

	// For images and videos, use inline disposition so browser displays them
	if strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, "video/") {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", filename))
	} else {
		// For other files, use attachment disposition
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	}

	// Stream the file to the client
	if _, err := io.Copy(w, reader); err != nil {
		// Can't send an error response here as we've already written to the response
		log.Printf("Error streaming file preview: %v\n", err)
	}
}

// Helper functions

// createProcessedFile creates a processed file object from a processing result
func createProcessedFile(file *models.File, result *processors.ProcessResult) *models.ProcessedFile {
	// Generate preview URL if preview was generated
	var previewURL string
	if result.Preview != nil && len(result.Preview) > 0 {
		// In a real app, you'd save the preview and generate a URL for it
		// For now, we'll just report that a preview is available
		previewURL = fmt.Sprintf("/api/preview/%s", file.ID)
	}

	// Create processed file
	processedFile := &models.ProcessedFile{
		File:         file,
		Summary:      result.Summary,
		PreviewURL:   previewURL,
		ProcessedAt:  time.Now(),
		ProcessStats: make(map[string]string),
	}

	// Copy relevant metadata from result to process stats
	for k, v := range result.Metadata {
		processedFile.ProcessStats[k] = v
	}

	return processedFile
}

// processUploadedFile processes a file using the appropriate processor
func processUploadedFile(ctx context.Context, file *models.File, provider storage.Provider) (*models.ProcessedFile, error) {
	// Get file content
	reader, _, err := provider.Retrieve(ctx, file.StorageID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file: %w", err)
	}
	defer reader.Close()

	// Get processor for this file type
	processor := processors.GetProcessor(file.ContentType, filepath.Ext(file.Name))
	if processor == nil {
		return nil, fmt.Errorf("no processor available for file type: %s", file.ContentType)
	}

	// Process the file
	options := processors.ProcessOptions{
		GeneratePreview: true,
		ExtractMetadata: true,
		MaxPreviewSize:  1024 * 10, // 10KB
	}

	result, err := processor.Process(ctx, reader, file.Name, options)
	if err != nil {
		return nil, fmt.Errorf("failed to process file: %w", err)
	}

	// Generate preview URL if preview was generated
	var previewURL string
	if result.Preview != nil && len(result.Preview) > 0 {
		// In a real app, you'd save the preview and generate a URL for it
		// For now, we'll just report that a preview is available
		previewURL = fmt.Sprintf("/api/preview/%s", file.ID)
	}

	// Create processed file
	processedFile := &models.ProcessedFile{
		File:         file,
		Summary:      result.Summary,
		PreviewURL:   previewURL,
		ProcessedAt:  time.Now(),
		ProcessStats: make(map[string]string),
	}

	// Copy relevant metadata from result to process stats
	for k, v := range result.Metadata {
		processedFile.ProcessStats[k] = v
	}

	return processedFile, nil
}

// extractStorageConfig extracts storage configuration from the request
func extractStorageConfig(r *http.Request, storageType string) map[string]string {
	config := make(map[string]string)

	// Get configuration from query parameters or form values
	switch storageType {
	case "s3", "amazon", "aws":
		config["region"] = getParamValue(r, "region")
		config["bucket"] = getParamValue(r, "bucket")
		config["accessKey"] = getParamValue(r, "accessKey")
		config["secretKey"] = getParamValue(r, "secretKey")
		config["prefix"] = getParamValue(r, "prefix")
	case "gcs", "google":
		config["bucket"] = getParamValue(r, "bucket")
		config["credentialFile"] = getParamValue(r, "credentialFile")
		config["prefix"] = getParamValue(r, "prefix")
	}

	return config
}

// getParamValue gets a parameter value from the request
func getParamValue(r *http.Request, name string) string {
	// Try query param first
	value := r.URL.Query().Get(name)
	if value != "" {
		return value
	}

	// Then try form value
	return r.FormValue(name)
}

// sendJSONResponse sends a JSON response to the client
func sendJSONResponse(w http.ResponseWriter, response interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// sendJSONError sends a JSON error response to the client
func sendJSONError(w http.ResponseWriter, message string, status int) {
	response := models.APIResponse{
		Success: false,
		Error:   message,
	}

	sendJSONResponse(w, response, status)
}
