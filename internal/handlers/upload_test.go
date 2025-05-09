package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/example/fileprocessor/internal/processors"
)

func TestUploadAndProcessFile(t *testing.T) {
	// Create a test file
	tempDir, err := os.MkdirTemp("", "fileprocessor-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := "This is test content for upload"
	err = os.WriteFile(testFilePath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a buffer to store our request body as multipart/form-data
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Create form file
	fileWriter, err := multipartWriter.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	// Open test file
	file, err := os.Open(testFilePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	// Copy test file content to form file
	fileContent, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	fileWriter.Write(fileContent)

	// Add other form fields
	multipartWriter.WriteField("storageType", "local")
	multipartWriter.WriteField("processFile", "true")

	// Close multipart writer
	multipartWriter.Close()

	// Create request
	req, err := http.NewRequest("POST", "/api/upload", &requestBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	// Create response recorder
	rr := httptest.NewRecorder()

	// Create file handler
	fileHandler, err := NewFileHandler()
	if err != nil {
		t.Fatalf("Failed to create file handler: %v", err)
	}
	// Initialize WebSocket hub
	if DefaultWebSocketHub == nil {
		DefaultWebSocketHub = NewWebSocketHub()
		go DefaultWebSocketHub.Run()
	}
	// Initialize the worker pool if needed
	if processors.DefaultPool == nil {
		processors.InitializeWorkerPool(2, 5) // Create worker pool with 2 workers and queue size 5
	}

	// Perform the request
	t.Log("Performing upload request...")
	fileHandler.UploadFile(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
		return
	}

	// Check that response contains success message
	response := rr.Body.String()
	if !strings.Contains(response, `"success":true`) {
		t.Errorf("Expected successful response, got: %s", response)
		return
	}

	t.Logf("Upload successful, checking processing status...")

	// If the response contains a task ID, we can check task status
	if strings.Contains(response, `"taskId"`) {
		// Extract taskId
		parts := strings.Split(response, `"taskId":"`)
		if len(parts) < 2 {
			t.Errorf("Could not find taskId in response: %s", response)
			return
		}

		taskID := strings.Split(parts[1], `"`)[0]
		t.Logf("Found task ID: %s", taskID)

		// Wait for processing to complete (with timeout)
		timeout := time.After(10 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C: // Get the task status using the processors package
				status := processors.GetTaskStatus(taskID)
				if status == nil {
					t.Logf("Task status not available yet")
					continue
				}

				t.Logf("Task status: %+v", status)

				if statusStr, ok := status["status"].(string); ok {
					if statusStr == "complete" {
						t.Logf("Task completed successfully")
						return
					} else if statusStr == "error" {
						t.Errorf("Task failed with error: %v", status["error"])
						return
					}
				}

				// Continue waiting
			case <-timeout:
				t.Fatalf("Test timed out waiting for task to complete")
				return
			}
		}
	}

	t.Log("Upload and process test completed")
}
