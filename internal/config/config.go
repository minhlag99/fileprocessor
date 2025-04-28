// Package config provides configuration management for the file processor application
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig  `json:"server"`
	Storage  StorageConfig `json:"storage"`
	Workers  WorkerConfig  `json:"workers"`
	Features FeatureConfig `json:"features"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Port            int    `json:"port"`
	UIDir           string `json:"uiDir"`
	UploadsDir      string `json:"uploadsDir"`
	CertFile        string `json:"certFile"`
	KeyFile         string `json:"keyFile"`
	ShutdownTimeout int    `json:"shutdownTimeout"`
	Host            string `json:"host"`
	AllowedOrigins  string `json:"allowedOrigins"`
}

// StorageConfig contains storage-related configuration
type StorageConfig struct {
	DefaultProvider string            `json:"defaultProvider"`
	Local           map[string]string `json:"local"`
	S3              map[string]string `json:"s3"`
	Google          map[string]string `json:"google"`
}

// WorkerConfig contains worker pool configuration
type WorkerConfig struct {
	Count       int `json:"count"`
	QueueSize   int `json:"queueSize"`
	MaxAttempts int `json:"maxAttempts"`
}

// FeatureConfig contains feature flags
type FeatureConfig struct {
	EnableLAN             bool `json:"enableLAN"`
	EnableProcessing      bool `json:"enableProcessing"`
	EnableCloudStorage    bool `json:"enableCloudStorage"`
	EnableProgressUpdates bool `json:"enableProgressUpdates"`
}

// AppConfig is the global application configuration
var AppConfig Config

// LoadConfig loads configuration from a file and environment variables
func LoadConfig(configFile string) error {
	// Set defaults
	AppConfig = Config{
		Server: ServerConfig{
			Port:            8080,
			UIDir:           "./ui",
			UploadsDir:      "./uploads",
			ShutdownTimeout: 30,
			Host:            "0.0.0.0", // Default to all interfaces
		},
		Storage: StorageConfig{
			DefaultProvider: "local",
			Local:           map[string]string{"basePath": "./uploads"},
		},
		Workers: WorkerConfig{
			Count:       runtime.NumCPU(),
			QueueSize:   100,
			MaxAttempts: 3,
		},
		Features: FeatureConfig{
			EnableLAN:             true,
			EnableProcessing:      true,
			EnableCloudStorage:    true,
			EnableProgressUpdates: true,
		},
	}

	// Load from config file if it exists
	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			data, err := os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("error reading config file: %w", err)
			}

			if err := json.Unmarshal(data, &AppConfig); err != nil {
				return fmt.Errorf("error parsing config file: %w", err)
			}
		}
	}

	// Override with environment variables
	overrideWithEnv()

	// Create required directories
	if err := ensureDirectoriesExist(); err != nil {
		return err
	}

	return nil
}

// overrideWithEnv overrides configuration with environment variables
func overrideWithEnv() {
	// Server config
	if port := os.Getenv("FP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			AppConfig.Server.Port = p
		}
	}

	if uiDir := os.Getenv("FP_UI_DIR"); uiDir != "" {
		AppConfig.Server.UIDir = uiDir
	}

	if uploadsDir := os.Getenv("FP_UPLOADS_DIR"); uploadsDir != "" {
		AppConfig.Server.UploadsDir = uploadsDir
	}

	if host := os.Getenv("FP_HOST"); host != "" {
		AppConfig.Server.Host = host
	}

	if certFile := os.Getenv("FP_CERT_FILE"); certFile != "" {
		AppConfig.Server.CertFile = certFile
	}

	if keyFile := os.Getenv("FP_KEY_FILE"); keyFile != "" {
		AppConfig.Server.KeyFile = keyFile
	}

	// Worker config
	if workerCount := os.Getenv("FP_WORKER_COUNT"); workerCount != "" {
		if wc, err := strconv.Atoi(workerCount); err == nil {
			AppConfig.Workers.Count = wc
		}
	}

	// Feature flags
	if enableLAN := os.Getenv("FP_ENABLE_LAN"); enableLAN != "" {
		AppConfig.Features.EnableLAN = enableLAN == "true" || enableLAN == "1"
	}
}

// ensureDirectoriesExist creates required directories if they don't exist
func ensureDirectoriesExist() error {
	dirs := []string{
		AppConfig.Server.UIDir,
		AppConfig.Server.UploadsDir,
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}

		// Resolve relative paths
		if !filepath.IsAbs(dir) {
			dir = filepath.Clean(dir)
		}

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	return nil
}

// GetAddressString returns the address string for the server to listen on
func GetAddressString() string {
	return fmt.Sprintf("%s:%d", AppConfig.Server.Host, AppConfig.Server.Port)
}
