package tests

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const (
	baseURL = "http://localhost:8080"
)

func TestFileUpload(t *testing.T) {
	filePath := "testdata/testfile.txt"
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/api/upload", body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestFileDownload(t *testing.T) {
	fileID := "testfile.txt"
	resp, err := http.Get(baseURL + "/api/download?file=" + fileID)
	if err != nil {
		t.Fatalf("Failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
}

func TestFileList(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/list")
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var files []string
	err = json.NewDecoder(resp.Body).Decode(&files)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("Expected non-empty file list")
	}
}

func TestFileDelete(t *testing.T) {
	fileID := "testfile.txt"
	req, err := http.NewRequest("DELETE", baseURL+"/api/delete?file="+fileID, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestMediaPreview(t *testing.T) {
	fileID := "testfile.txt"
	resp, err := http.Get(baseURL + "/api/preview/" + fileID)
	if err != nil {
		t.Fatalf("Failed to get media preview: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
}

func TestAuthLogin(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/auth/login")
	if err != nil {
		t.Fatalf("Failed to initiate login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAuthCallback(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/auth/callback")
	if err != nil {
		t.Fatalf("Failed to handle callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAuthProfile(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/auth/profile")
	if err != nil {
		t.Fatalf("Failed to get profile: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var profile map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&profile)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if profile["email"] == "" {
		t.Fatalf("Expected non-empty email in profile")
	}
}

func TestAuthLogout(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/auth/logout")
	if err != nil {
		t.Fatalf("Failed to logout: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestConfigOptions(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/server/info")
	if err != nil {
		t.Fatalf("Failed to get server info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var info map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if info["version"] == "" {
		t.Fatalf("Expected non-empty version in server info")
	}
}
