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
	AppConfig     Config
	configInitMtx sync.Mutex
	initialized   bool
)

type Config struct {
	Server struct {
		Port            int      `json:"port"`
		Host            string   `json:"host"`
		UIDir           string   `json:"uiDir"`
		UploadsDir      string   `json:"uploadsDir"`
		AllowedOrigins  []string `json:"allowedOrigins"`
		ReadTimeout     int      `json:"readTimeout"`
		WriteTimeout    int      `json:"writeTimeout"`
		IdleTimeout     int      `json:"idleTimeout"`
		ShutdownTimeout int      `json:"shutdownTimeout"`
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

func InitConfig(configPath string) error {
	configInitMtx.Lock()
	defer configInitMtx.Unlock()

	if initialized {
		return nil
	}

	setDefaults()

	if configPath != "" {
		if err := loadConfigFile(configPath); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	overrideWithEnv()

	if err := ensureDirectories(); err != nil {
		return fmt.Errorf("failed to create required directories: %w", err)
	}

	if AppConfig.Auth.SessionSecret == "" || AppConfig.Auth.SessionSecret == "default-insecure-secret-change-me" {
		secret, err := generateRandomSecret(32)
		if err != nil {
			return fmt.Errorf("failed to generate secure session secret: %w", err)
		}
		AppConfig.Auth.SessionSecret = secret
	}

	enforceMinimumTimeouts()

	if err := validateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if err := secureCredentialFiles(); err != nil {
		return fmt.Errorf("failed to secure credential files: %w", err)
	}

	initialized = true
	return nil
}

func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func setDefaults() {
	AppConfig.Server.Port = 8080
	AppConfig.Server.Host = "0.0.0.0"
	AppConfig.Server.UIDir = "./ui"
	AppConfig.Server.UploadsDir = "./uploads"
	AppConfig.Server.AllowedOrigins = []string{"*"}
	AppConfig.Server.ReadTimeout = 30
	AppConfig.Server.WriteTimeout = 60
	AppConfig.Server.IdleTimeout = 120
	AppConfig.Server.ShutdownTimeout = 30

	AppConfig.Storage.DefaultProvider = "local"
	AppConfig.Storage.Local.BasePath = "./uploads"

	AppConfig.Workers.Count = 4
	AppConfig.Workers.QueueSize = 100
	AppConfig.Workers.MaxAttempts = 3

	AppConfig.Features.EnableProcessing = true
	AppConfig.Features.EnableCloudStorage = true
	AppConfig.Features.EnableProgressUpdates = true
	AppConfig.Features.EnableAuth = false
	AppConfig.Features.EnableLAN = false
	AppConfig.Features.EnableMediaPreview = true

	AppConfig.Auth.OAuthRedirectURL = "http://localhost:8080/api/auth/callback"
	AppConfig.Auth.CookieMaxAge = 86400 * 7

	AppConfig.SSL.Enable = false
}

func loadConfigFile(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", configPath)
	}

	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&AppConfig); err != nil {
		return fmt.Errorf("failed to decode JSON config: %w", err)
	}

	return nil
}

func overrideWithEnv() {
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

	if provider := os.Getenv("FP_STORAGE_PROVIDER"); provider != "" {
		AppConfig.Storage.DefaultProvider = provider
	}
	if basePath := os.Getenv("FP_LOCAL_PATH"); basePath != "" {
		AppConfig.Storage.Local.BasePath = basePath
	}

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

	if bucket := os.Getenv("FP_GOOGLE_BUCKET"); bucket != "" {
		AppConfig.Storage.Google.Bucket = bucket
	}
	if credFile := os.Getenv("FP_GOOGLE_CRED_FILE"); credFile != "" {
		AppConfig.Storage.Google.CredentialFile = credFile
	}
	if prefix := os.Getenv("FP_GOOGLE_PREFIX"); prefix != "" {
		AppConfig.Storage.Google.Prefix = prefix
	}

	if count := os.Getenv("FP_WORKER_COUNT"); count != "" {
		if parsed, err := strconv.Atoi(count); err == nil {
			AppConfig.Workers.Count = parsed
		}
	}

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

	if enableSSL := os.Getenv("FP_SSL_ENABLE"); enableSSL != "" {
		AppConfig.SSL.Enable = strings.ToLower(enableSSL) == "true"
	}
	if certFile := os.Getenv("FP_CERT_FILE"); certFile != "" {
		AppConfig.SSL.CertFile = certFile
	}
	if keyFile := os.Getenv("FP_KEY_FILE"); keyFile != "" {
		AppConfig.SSL.KeyFile = keyFile
	}

	if AppConfig.SSL.Enable {
		AppConfig.Auth.CookieSecure = true
	}
}

func ensureDirectories() error {
	if err := os.MkdirAll(AppConfig.Server.UploadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create uploads directory: %w", err)
	}

	tempDir := filepath.Join(AppConfig.Server.UploadsDir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	return nil
}

func validateConfig() error {
	if AppConfig.Server.Port <= 0 || AppConfig.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", AppConfig.Server.Port)
	}

	if AppConfig.Storage.DefaultProvider != "local" && AppConfig.Storage.DefaultProvider != "s3" && AppConfig.Storage.DefaultProvider != "google" {
		return fmt.Errorf("invalid default storage provider: %s", AppConfig.Storage.DefaultProvider)
	}

	if AppConfig.Workers.Count <= 0 {
		return fmt.Errorf("worker count must be positive: %d", AppConfig.Workers.Count)
	}
	if AppConfig.Workers.QueueSize <= 0 {
		return fmt.Errorf("worker queue size must be positive: %d", AppConfig.Workers.QueueSize)
	}
	if AppConfig.Workers.MaxAttempts <= 0 {
		return fmt.Errorf("worker max attempts must be positive: %d", AppConfig.Workers.MaxAttempts)
	}

	if AppConfig.SSL.Enable {
		if AppConfig.SSL.CertFile == "" {
			return fmt.Errorf("SSL enabled but no certificate file provided")
		}
		if AppConfig.SSL.KeyFile == "" {
			return fmt.Errorf("SSL enabled but no key file provided")
		}
		if _, err := os.Stat(AppConfig.SSL.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("SSL certificate file does not exist: %s", AppConfig.SSL.CertFile)
		}
		if _, err := os.Stat(AppConfig.SSL.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("SSL key file does not exist: %s", AppConfig.SSL.KeyFile)
		}
	}

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

func secureCredentialFiles() error {
	if AppConfig.Storage.Google.CredentialFile != "" {
		if _, err := os.Stat(AppConfig.Storage.Google.CredentialFile); err == nil {
			if err := os.Chmod(AppConfig.Storage.Google.CredentialFile, 0600); err != nil {
				return fmt.Errorf("failed to set secure permissions on Google credential file: %w", err)
			}
		}
	}

	if AppConfig.Storage.S3.AccessKey != "" && AppConfig.Storage.S3.SecretKey != "" {
		credsFile := filepath.Join(os.TempDir(), "fileprocessor-aws-"+getUserHash())
		awsCredentials := fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n",
			AppConfig.Storage.S3.AccessKey, AppConfig.Storage.S3.SecretKey)

		if err := writeSecureFile(credsFile, []byte(awsCredentials), 0600); err != nil {
			return fmt.Errorf("failed to write secure AWS credentials file: %w", err)
		}

		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsFile)

		AppConfig.Storage.S3.AccessKey = "*** stored in secure file ***"
		AppConfig.Storage.S3.SecretKey = "*** stored in secure file ***"
	}

	return nil
}

func writeSecureFile(filename string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func getUserHash() string {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	hasher := sha256.New()
	hostname, _ := os.Hostname()

	io.WriteString(hasher, username+hostname)
	return hex.EncodeToString(hasher.Sum(nil))[:8]
}

func enforceMinimumTimeouts() {
	if AppConfig.Server.ReadTimeout < 5 {
		AppConfig.Server.ReadTimeout = 5
	}
	if AppConfig.Server.WriteTimeout < 10 {
		AppConfig.Server.WriteTimeout = 10
	}
	if AppConfig.Server.IdleTimeout < 30 {
		AppConfig.Server.IdleTimeout = 30
	}
}

func GetAddressString() string {
	return fmt.Sprintf("%s:%d", AppConfig.Server.Host, AppConfig.Server.Port)
}

func GetConfig() Config {
	return AppConfig
}

func SaveConfig(configPath string) error {
	data, err := json.MarshalIndent(AppConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak." + time.Now().Format("20060102-150405")
		if err := os.Rename(configPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup of config file: %w", err)
		}
	}

	if err := writeSecureFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func LoadConfig(configPath string) error {
	return InitConfig(configPath)
}
