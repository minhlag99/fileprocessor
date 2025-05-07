// Package config provides configuration management for the file processor application
package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// AppConfig holds the application configuration
	AppConfig     Config
	configInitMtx sync.Mutex
	initialized   bool
)

// Config defines the application configuration
type Config struct {
	Server struct {
		Port           int      `json:"port"`
		Host           string   `json:"host"`
		UIDir          string   `json:"uiDir"`
		UploadsDir     string   `json:"uploadsDir"`
		AllowedOrigins []string `json:"allowedOrigins"`
		ReadTimeout    int      `json:"readTimeout"`
		WriteTimeout   int      `json:"writeTimeout"`
		IdleTimeout    int      `json:"idleTimeout"`
	} `json:"server"`
	Storage struct {
		DefaultProvider string `json:"defaultProvider"`
		Local           struct {
			BasePath string `json:"basePath"`
		} `json:"local"`
		S3 struct {
			Region    string `json:"region"`
			Bucket    string `json:"bucket"`
			AccessKey string `json:"accessKey"`
			SecretKey string `json:"secretKey"`
			Prefix    string `json:"prefix"`
		} `json:"s3"`
		Google struct {
			Bucket         string `json:"bucket"`
			CredentialFile string `json:"credentialFile"`
			Prefix         string `json:"prefix"`
		} `json:"google"`
	} `json:"storage"`
	Workers struct {
		Count       int `json:"count"`
		QueueSize   int `json:"queueSize"`
		MaxAttempts int `json:"maxAttempts"`
	} `json:"workers"`
	Features struct {
		EnableLAN             bool `json:"enableLAN"`
		EnableProcessing      bool `json:"enableProcessing"`
		EnableCloudStorage    bool `json:"enableCloudStorage"`
		EnableProgressUpdates bool `json:"enableProgressUpdates"`
		EnableAuth            bool `json:"enableAuth"`
		EnableMediaPreview    bool `json:"enableMediaPreview"`
	} `json:"features"`
	Auth struct {
		GoogleClientID     string `json:"googleClientID"`
		GoogleClientSecret string `json:"googleClientSecret"`
		OAuthRedirectURL   string `json:"oauthRedirectURL"`
		SessionSecret      string `json:"sessionSecret"`
		CookieSecure       bool   `json:"cookieSecure"`
		CookieMaxAge       int    `json:"cookieMaxAge"`
	} `json:"auth"`
	SSL struct {
		Enable   bool   `json:"enable"`
		CertFile string `json:"certFile"`
		KeyFile  string `json:"keyFile"`
	} `json:"ssl"`
}

// InitConfig initializes the application configuration
func InitConfig(configPath string) error {
	configInitMtx.Lock()
	defer configInitMtx.Unlock()

	if initialized {
		return nil
	}

	// Set default configuration
	setDefaults()

	// Load configuration from file if provided
	if configPath != "" {
		if err := loadConfigFile(configPath); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override configuration with environment variables
	overrideWithEnv()

	// Ensure required directories exist
	if err := ensureDirectories(); err != nil {
		return fmt.Errorf("failed to create required directories: %w", err)
	}

	// Generate a secure random session secret if not set
	if AppConfig.Auth.SessionSecret == "" || AppConfig.Auth.SessionSecret == "default-insecure-secret-change-me" {
		secret, err := generateRandomSecret(32)
		if err != nil {
			return fmt.Errorf("failed to generate secure session secret: %w", err)
		}
		AppConfig.Auth.SessionSecret = secret
	}

	// Ensure server timeouts are reasonable
	enforceMinimumTimeouts()

	// Validate the configuration
	if err := validateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Secure sensitive files and credentials
	if err := secureCredentialFiles(); err != nil {
		return fmt.Errorf("failed to secure credential files: %w", err)
	}

	initialized = true
	return nil
}

// generateRandomSecret generates a secure random string for use as a secret key
func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	AppConfig.Server.Port = 8080
	AppConfig.Server.Host = "0.0.0.0"
	AppConfig.Server.UIDir = "./ui"
	AppConfig.Server.UploadsDir = "./uploads"
	AppConfig.Server.AllowedOrigins = []string{"*"}
	AppConfig.Server.ReadTimeout = 30
	AppConfig.Server.WriteTimeout = 60
	AppConfig.Server.IdleTimeout = 120

	// Storage defaults
	AppConfig.Storage.DefaultProvider = "local"
	AppConfig.Storage.Local.BasePath = "./uploads"

	// Workers defaults
	AppConfig.Workers.Count = 4
	AppConfig.Workers.QueueSize = 100
	AppConfig.Workers.MaxAttempts = 3

	// Features defaults
	AppConfig.Features.EnableProcessing = true
	AppConfig.Features.EnableCloudStorage = true
	AppConfig.Features.EnableProgressUpdates = true
	AppConfig.Features.EnableAuth = false
	AppConfig.Features.EnableLAN = false
	AppConfig.Features.EnableMediaPreview = true

	// Auth defaults
	AppConfig.Auth.OAuthRedirectURL = "http://localhost:8080/api/auth/callback"
	AppConfig.Auth.CookieMaxAge = 86400 * 7 // 7 days

	// SSL defaults
	AppConfig.SSL.Enable = false
}

// loadConfigFile loads configuration from a file
func loadConfigFile(configPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Open and read the file
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// Decode JSON
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&AppConfig); err != nil {
		return fmt.Errorf("failed to decode JSON config: %w", err)
	}

	return nil
}

// overrideWithEnv overrides configuration with environment variables
func overrideWithEnv() {
	// Server overrides
	if port := os.Getenv("FP_PORT"); port != "" {
		if parsedPort, err := strconv.Atoi(port); err == nil {
			AppConfig.Server.Port = parsedPort
		}
	}
	if host := os.Getenv("FP_HOST"); host != "" {
		AppConfig.Server.Host = host
	}
	if uiDir := os.Getenv("FP_UI_DIR"); uiDir != "" {
		AppConfig.Server.UIDir = uiDir
	}
	if uploadsDir := os.Getenv("FP_UPLOADS_DIR"); uploadsDir != "" {
		AppConfig.Server.UploadsDir = uploadsDir
	}
	if origins := os.Getenv("FP_ALLOWED_ORIGINS"); origins != "" {
		AppConfig.Server.AllowedOrigins = strings.Split(origins, ",")
	}
	if readTimeout := os.Getenv("FP_READ_TIMEOUT"); readTimeout != "" {
		if parsed, err := strconv.Atoi(readTimeout); err == nil {
			AppConfig.Server.ReadTimeout = parsed
		}
	}
	if writeTimeout := os.Getenv("FP_WRITE_TIMEOUT"); writeTimeout != "" {
		if parsed, err := strconv.Atoi(writeTimeout); err == nil {
			AppConfig.Server.WriteTimeout = parsed
		}
	}
	if idleTimeout := os.Getenv("FP_IDLE_TIMEOUT"); idleTimeout != "" {
		if parsed, err := strconv.Atoi(idleTimeout); err == nil {
			AppConfig.Server.IdleTimeout = parsed
		}
	}

	// Storage overrides
	if provider := os.Getenv("FP_STORAGE_PROVIDER"); provider != "" {
		AppConfig.Storage.DefaultProvider = provider
	}
	if basePath := os.Getenv("FP_LOCAL_PATH"); basePath != "" {
		AppConfig.Storage.Local.BasePath = basePath
	}

	// S3 overrides
	if region := os.Getenv("FP_S3_REGION"); region != "" {
		AppConfig.Storage.S3.Region = region
	}
	if bucket := os.Getenv("FP_S3_BUCKET"); bucket != "" {
		AppConfig.Storage.S3.Bucket = bucket
	}
	if accessKey := os.Getenv("FP_S3_ACCESS_KEY"); accessKey != "" {
		AppConfig.Storage.S3.AccessKey = accessKey
	}
	if secretKey := os.Getenv("FP_S3_SECRET_KEY"); secretKey != "" {
		AppConfig.Storage.S3.SecretKey = secretKey
	}
	if prefix := os.Getenv("FP_S3_PREFIX"); prefix != "" {
		AppConfig.Storage.S3.Prefix = prefix
	}

	// Google overrides
	if bucket := os.Getenv("FP_GOOGLE_BUCKET"); bucket != "" {
		AppConfig.Storage.Google.Bucket = bucket
	}
	if credFile := os.Getenv("FP_GOOGLE_CRED_FILE"); credFile != "" {
		AppConfig.Storage.Google.CredentialFile = credFile
	}
	if prefix := os.Getenv("FP_GOOGLE_PREFIX"); prefix != "" {
		AppConfig.Storage.Google.Prefix = prefix
	}

	// Workers overrides
	if count := os.Getenv("FP_WORKER_COUNT"); count != "" {
		if parsed, err := strconv.Atoi(count); err == nil {
			AppConfig.Workers.Count = parsed
		}
	}

	// Features overrides
	if enableLAN := os.Getenv("FP_ENABLE_LAN"); enableLAN != "" {
		AppConfig.Features.EnableLAN = strings.ToLower(enableLAN) == "true"
	}
	if enableAuth := os.Getenv("FP_ENABLE_AUTH"); enableAuth != "" {
		AppConfig.Features.EnableAuth = strings.ToLower(enableAuth) == "true"
	}
	if enableProcessing := os.Getenv("FP_ENABLE_PROCESSING"); enableProcessing != "" {
		AppConfig.Features.EnableProcessing = strings.ToLower(enableProcessing) == "true"
	}
	if enableCloudStorage := os.Getenv("FP_ENABLE_CLOUD_STORAGE"); enableCloudStorage != "" {
		AppConfig.Features.EnableCloudStorage = strings.ToLower(enableCloudStorage) == "true"
	}

	// Auth overrides
	if clientID := os.Getenv("FP_GOOGLE_CLIENT_ID"); clientID != "" {
		AppConfig.Auth.GoogleClientID = clientID
	}
	if clientSecret := os.Getenv("FP_GOOGLE_CLIENT_SECRET"); clientSecret != "" {
		AppConfig.Auth.GoogleClientSecret = clientSecret
	}
	if redirectURL := os.Getenv("FP_OAUTH_REDIRECT_URL"); redirectURL != "" {
		AppConfig.Auth.OAuthRedirectURL = redirectURL
	}
	if sessionSecret := os.Getenv("FP_SESSION_SECRET"); sessionSecret != "" {
		AppConfig.Auth.SessionSecret = sessionSecret
	}
	if cookieSecure := os.Getenv("FP_COOKIE_SECURE"); cookieSecure != "" {
		AppConfig.Auth.CookieSecure = strings.ToLower(cookieSecure) == "true"
	}

	// SSL overrides
	if enableSSL := os.Getenv("FP_SSL_ENABLE"); enableSSL != "" {
		AppConfig.SSL.Enable = strings.ToLower(enableSSL) == "true"
	}
	if certFile := os.Getenv("FP_CERT_FILE"); certFile != "" {
		AppConfig.SSL.CertFile = certFile
	}
	if keyFile := os.Getenv("FP_KEY_FILE"); keyFile != "" {
		AppConfig.SSL.KeyFile = keyFile
	}

	// Force cookie security with SSL
	if AppConfig.SSL.Enable {
		AppConfig.Auth.CookieSecure = true
	}
}

// ensureDirectories ensures that required directories exist
func ensureDirectories() error {
	// Ensure uploads directory exists
	if err := os.MkdirAll(AppConfig.Server.UploadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create uploads directory: %w", err)
	}

	// Create a directory for temporary files
	tempDir := filepath.Join(AppConfig.Server.UploadsDir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig() error {
	// Validate server configuration
	if AppConfig.Server.Port <= 0 || AppConfig.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", AppConfig.Server.Port)
	}

	// Validate storage configuration
	if AppConfig.Storage.DefaultProvider != "local" && AppConfig.Storage.DefaultProvider != "s3" && AppConfig.Storage.DefaultProvider != "google" {
		return fmt.Errorf("invalid default storage provider: %s", AppConfig.Storage.DefaultProvider)
	}

	// Validate worker configuration
	if AppConfig.Workers.Count <= 0 {
		return fmt.Errorf("worker count must be positive: %d", AppConfig.Workers.Count)
	}
	if AppConfig.Workers.QueueSize <= 0 {
		return fmt.Errorf("worker queue size must be positive: %d", AppConfig.Workers.QueueSize)
	}
	if AppConfig.Workers.MaxAttempts <= 0 {
		return fmt.Errorf("worker max attempts must be positive: %d", AppConfig.Workers.MaxAttempts)
	}

	// Validate SSL configuration if enabled
	if AppConfig.SSL.Enable {
		if AppConfig.SSL.CertFile == "" {
			return fmt.Errorf("SSL enabled but no certificate file provided")
		}
		if AppConfig.SSL.KeyFile == "" {
			return fmt.Errorf("SSL enabled but no key file provided")
		}
		// Check if cert and key files exist
		if _, err := os.Stat(AppConfig.SSL.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("SSL certificate file does not exist: %s", AppConfig.SSL.CertFile)
		}
		if _, err := os.Stat(AppConfig.SSL.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("SSL key file does not exist: %s", AppConfig.SSL.KeyFile)
		}
	}

	// Validate authentication configuration if enabled
	if AppConfig.Features.EnableAuth {
		if AppConfig.Auth.GoogleClientID == "" {
			return fmt.Errorf("authentication is enabled but no Google client ID provided")
		}
		if AppConfig.Auth.GoogleClientSecret == "" {
			return fmt.Errorf("authentication is enabled but no Google client secret provided")
		}
		if AppConfig.Auth.OAuthRedirectURL == "" {
			return fmt.Errorf("authentication is enabled but no OAuth redirect URL provided")
		}
	}

	return nil
}

// secureCredentialFiles ensures sensitive credential files have proper permissions
func secureCredentialFiles() error {
	// Secure Google Cloud credential file if provided
	if AppConfig.Storage.Google.CredentialFile != "" {
		if _, err := os.Stat(AppConfig.Storage.Google.CredentialFile); err == nil {
			// Set file permissions to read/write for owner only (0600)
			if err := os.Chmod(AppConfig.Storage.Google.CredentialFile, 0600); err != nil {
				return fmt.Errorf("failed to set secure permissions on Google credential file: %w", err)
			}
		}
	}

	// If we're storing AWS credentials in the config file, create a secure credentials file
	if AppConfig.Storage.S3.AccessKey != "" && AppConfig.Storage.S3.SecretKey != "" {
		credsFile := filepath.Join(os.TempDir(), "fileprocessor-aws-"+getUserHash())
		awsCredentials := fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n",
			AppConfig.Storage.S3.AccessKey, AppConfig.Storage.S3.SecretKey)

		// Write with secure permissions
		if err := writeSecureFile(credsFile, []byte(awsCredentials), 0600); err != nil {
			return fmt.Errorf("failed to write secure AWS credentials file: %w", err)
		}

		// Set the AWS_SHARED_CREDENTIALS_FILE environment variable
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsFile)

		// Clear credentials from memory (they'll still be in the environment and the secure file)
		// Note: This doesn't actually clear memory, but helps avoid credentials appearing in logs/errors
		AppConfig.Storage.S3.AccessKey = "*** stored in secure file ***"
		AppConfig.Storage.S3.SecretKey = "*** stored in secure file ***"
	}

	return nil
}

// writeSecureFile writes data to a file with specific permissions
func writeSecureFile(filename string, data []byte, perm os.FileMode) error {
	// Create the file with restricted permissions
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the data
	_, err = file.Write(data)
	return err
}

// getUserHash returns a unique hash for the current user to prevent credential file collisions
func getUserHash() string {
	// Get user information for a unique but consistent hash
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	// Create a hash of the username and hostname
	hasher := sha256.New()
	hostname, _ := os.Hostname()

	io.WriteString(hasher, username+hostname)
	return hex.EncodeToString(hasher.Sum(nil))[:8]
}

// enforceMinimumTimeouts ensures server timeouts are not set too low
func enforceMinimumTimeouts() {
	if AppConfig.Server.ReadTimeout < 5 {
		AppConfig.Server.ReadTimeout = 5 // Minimum 5 seconds
	}
	if AppConfig.Server.WriteTimeout < 10 {
		AppConfig.Server.WriteTimeout = 10 // Minimum 10 seconds
	}
	if AppConfig.Server.IdleTimeout < 30 {
		AppConfig.Server.IdleTimeout = 30 // Minimum 30 seconds
	}
}

// GetConfig returns a copy of the current configuration
func GetConfig() Config {
	return AppConfig
}

// SaveConfig saves the current configuration to a file
func SaveConfig(configPath string) error {
	data, err := json.MarshalIndent(AppConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create backup of existing file if it exists
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak." + time.Now().Format("20060102-150405")
		if err := os.Rename(configPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup of config file: %w", err)
		}
	}

	// Write with secure permissions
	if err := writeSecureFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
