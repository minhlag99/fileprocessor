// Package main is the entry point for the file processor server
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/example/fileprocessor/internal/auth"
	"github.com/example/fileprocessor/internal/config"
	"github.com/example/fileprocessor/internal/handlers"
	"github.com/example/fileprocessor/internal/middleware"
	"github.com/example/fileprocessor/internal/processors"
)

var (
	configFile = flag.String("config", "fileprocessor.ini", "Configuration file path")
	testConfig = flag.Bool("test-config", false, "Test configuration and exit")
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	version    = "1.3.1"    // Updated version number for the application
	startTime  = time.Now() // Track server start time for uptime reporting
)

// isPortInUse checks if the given port is already in use
func isPortInUse(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Port is likely in use
		return true
	}
	// Close the listener and return false to indicate port is available
	listener.Close()
	return false
}

// findFreePort tries to find a free port starting from the given port
// and incrementing by 1 if the port is in use. Stops searching after
// trying 100 ports or reaching port 65535.
func findFreePort(startPort int) int {
	port := startPort
	maxPortToTry := startPort + 100
	if maxPortToTry > 65535 {
		maxPortToTry = 65535
	}

	for port <= maxPortToTry {
		if !isPortInUse(port) {
			return port
		}
		port++
	}

	// If no free port found in the range, return the original port
	// (will still fail when the server tries to start)
	return startPort
}

// setupGlobalErrorHandling configures global error handling for unhandled panics
func setupGlobalErrorHandling() {
	// Set up a custom handler for unhandled panics
	prev := debug.SetPanicOnFault(true)

	if prev {
		log.Println("Global error handling was already enabled")
	}

	// Create a handler for uncaught panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				log.Printf("FATAL: Recovered from panic: %v\n%s", r, stack)

				// Create a crash log file
				crashLog := fmt.Sprintf("crash-%s.log", time.Now().Format("20060102-150405"))
				f, err := os.Create(crashLog)
				if err == nil {
					fmt.Fprintf(f, "Time: %s\n", time.Now().Format(time.RFC3339))
					fmt.Fprintf(f, "Version: %s\n", version)
					fmt.Fprintf(f, "Error: %v\n\n", r)
					fmt.Fprintf(f, "Stack Trace:\n%s\n", stack)
					f.Close()
					log.Printf("Crash report written to %s", crashLog)
				}

				// In critical cases, we might want to force exit
				// os.Exit(1)
			}
		}()

		// This will block until a signal is received
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGABRT)
		<-sigChan
	}()
}

// checkDependencies validates that required dependencies are available
func checkDependencies() error {
	// Check for required directories
	requiredDirs := []string{
		config.AppConfig.Storage.Local.BasePath,
		config.AppConfig.Server.UIDir,
		filepath.Join(config.AppConfig.Server.UploadsDir, "temp"), // Using uploads/temp as TempDir
	}

	for _, dir := range requiredDirs {
		if dir == "" {
			continue
		}

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			// Try to create the directory if it doesn't exist
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("required directory %s does not exist and could not be created: %w", dir, err)
			}
			log.Printf("Created missing directory: %s", dir)
		}
	}

	// Check permissions on directories
	for _, dir := range requiredDirs {
		if dir == "" {
			continue
		}

		// Try to create a test file to verify write permissions
		testFile := filepath.Join(dir, ".permission_test")
		f, err := os.Create(testFile)
		if err != nil {
			return fmt.Errorf("no write permission in directory %s: %w", dir, err)
		}
		f.Close()
		os.Remove(testFile)
	}

	return nil
}

// validateConfiguration performs additional validation on the loaded configuration
func validateConfiguration() error {
	// Check that the port is in a valid range
	if config.AppConfig.Server.Port <= 0 || config.AppConfig.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", config.AppConfig.Server.Port)
	}

	// Ensure worker count is reasonable
	if config.AppConfig.Workers.Count <= 0 {
		log.Println("Worker count not specified, using default of 4")
		config.AppConfig.Workers.Count = 4
	} else if config.AppConfig.Workers.Count > 32 {
		log.Printf("Warning: High worker count (%d) may cause excessive resource usage",
			config.AppConfig.Workers.Count)
	}

	// Check TLS configuration consistency
	if (config.AppConfig.SSL.CertFile != "" && config.AppConfig.SSL.KeyFile == "") ||
		(config.AppConfig.SSL.CertFile == "" && config.AppConfig.SSL.KeyFile != "") {
		return fmt.Errorf("inconsistent TLS configuration: both cert and key files must be specified")
	}

	// Validate that TLS certificates exist if specified
	if config.AppConfig.SSL.CertFile != "" && config.AppConfig.SSL.KeyFile != "" {
		if _, err := os.Stat(config.AppConfig.SSL.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate file not found: %s", config.AppConfig.SSL.CertFile)
		}
		if _, err := os.Stat(config.AppConfig.SSL.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file not found: %s", config.AppConfig.SSL.KeyFile)
		}
	}

	// Check UI directory
	if config.AppConfig.Server.UIDir == "" {
		// Default to 'ui' directory in the same folder as the executable
		exePath, err := os.Executable()
		if err == nil {
			config.AppConfig.Server.UIDir = filepath.Join(filepath.Dir(exePath), "ui")
			log.Printf("UI directory not specified, using default: %s", config.AppConfig.Server.UIDir)
		} else {
			return fmt.Errorf("UI directory not specified and could not determine executable path")
		}
	}

	// Check OAuth settings if auth is enabled
	if config.AppConfig.Features.EnableAuth {
		if config.AppConfig.Auth.GoogleClientID == "" || config.AppConfig.Auth.GoogleClientSecret == "" {
			return fmt.Errorf("authentication is enabled but OAuth credentials are missing")
		}
	}

	return nil
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Set up global error handling early to catch any panics
	setupGlobalErrorHandling()

	// Load application configuration
	if err := config.LoadConfig(*configFile); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := validateConfiguration(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Test configuration if requested
	if *testConfig {
		fmt.Println("Configuration test successful")
		return
	}

	// Check for required dependencies and permissions
	if err := checkDependencies(); err != nil {
		log.Fatalf("Dependency check failed: %v", err)
	}

	// Print banner and version
	fmt.Printf("\n=================================\n")
	fmt.Printf("File Management System v%s\n", version)
	fmt.Printf("=================================\n\n")
	fmt.Printf("Server starting with %d worker threads\n", config.AppConfig.Workers.Count)
	if config.AppConfig.Features.EnableLAN {
		fmt.Printf("LAN file transfer capabilities enabled\n")
	}
	if config.AppConfig.Features.EnableAuth {
		fmt.Printf("Authentication enabled\n")
	}

	// Initialize worker pool with configured number of workers
	log.Printf("Initializing worker pool with %d workers", config.AppConfig.Workers.Count)
	processors.InitializeWorkerPool(config.AppConfig.Workers.Count, config.AppConfig.Workers.QueueSize)

	// Initialize file handler
	fileHandler, err := handlers.NewFileHandler()
	if err != nil {
		log.Fatalf("Failed to initialize file handler: %v", err)
	}

	// Initialize LAN transfer handler if enabled
	var lanHandler *handlers.LANTransferHandler
	if config.AppConfig.Features.EnableLAN {
		log.Println("Initializing LAN file transfer capabilities")
		lanHandler, err = handlers.NewLANTransferHandler()
		if err != nil {
			log.Printf("Failed to initialize LAN transfer handler: %v", err)
			log.Println("LAN file transfer will be disabled")
		} else {
			if err := lanHandler.Start(); err != nil {
				log.Printf("Failed to start LAN transfer service: %v", err)
				log.Println("LAN file transfer will be disabled")
				lanHandler = nil
			}
		}
	}

	// Initialize authentication if enabled
	var authHandler *auth.AuthHandler
	if config.AppConfig.Features.EnableAuth {
		log.Println("Initializing authentication system")

		// Get base URL for OAuth redirect
		redirectURL := config.AppConfig.Auth.OAuthRedirectURL
		if (redirectURL == "") || (redirectURL == "http://localhost:8080/api/auth/callback") {
			// Auto-configure redirect URL based on host and port
			proto := "http"
			if config.AppConfig.SSL.CertFile != "" && config.AppConfig.SSL.KeyFile != "" {
				proto = "https"
			}
			redirectURL = fmt.Sprintf("%s://%s:%d/api/auth/callback",
				proto,
				config.AppConfig.Server.Host,
				config.AppConfig.Server.Port)
		}

		// Initialize auth system
		auth.Init(
			config.AppConfig.Auth.GoogleClientID,
			config.AppConfig.Auth.GoogleClientSecret,
			redirectURL,
		)

		authHandler = auth.DefaultAuthHandler
		log.Printf("OAuth redirects configured to: %s", redirectURL)
	}

	// Create router with middleware
	mux := http.NewServeMux()

	// Add middleware
	handler := middleware.Chain(
		mux,
		middleware.Logger(),
		middleware.Recover(),
		middleware.CORS(config.AppConfig.Server.AllowedOrigins),
	)

	// Set up routes
	setupRoutes(mux, fileHandler, lanHandler, authHandler)

	// Create HTTP server
	// Check if the configured port is available, if not find a free port
	originalPort := config.AppConfig.Server.Port
	if isPortInUse(originalPort) {
		newPort := findFreePort(originalPort)
		if newPort != originalPort {
			log.Printf("Port %d is already in use. Switching to alternative port %d", originalPort, newPort)
			// Update the port in the configuration
			config.AppConfig.Server.Port = newPort

			// If using OAuth, update the redirectURL with the new port
			if config.AppConfig.Features.EnableAuth {
				proto := "http"
				if config.AppConfig.SSL.CertFile != "" && config.AppConfig.SSL.KeyFile != "" {
					proto = "https"
				}
				// Only update if the redirect URL contains the original port
				if config.AppConfig.Auth.OAuthRedirectURL != "" &&
					config.AppConfig.Auth.OAuthRedirectURL != "http://localhost:8080/api/auth/callback" {
					newRedirectURL := fmt.Sprintf("%s://%s:%d/api/auth/callback",
						proto,
						config.AppConfig.Server.Host,
						newPort)
					config.AppConfig.Auth.OAuthRedirectURL = newRedirectURL
					// Re-initialize auth with the new URL
					auth.Init(
						config.AppConfig.Auth.GoogleClientID,
						config.AppConfig.Auth.GoogleClientSecret,
						newRedirectURL,
					)
					log.Printf("OAuth redirects updated to: %s", newRedirectURL)
				}
			}
		} else {
			log.Printf("Warning: Port %d is in use, but no alternative port was found. The server may fail to start.", originalPort)
		}
	}

	// Get the (possibly updated) address
	addr := config.GetAddressString()
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Verify that we're binding to the right addresses for external access
	log.Printf("Server will be accessible at %s", addr)
	if config.AppConfig.Server.Host == "0.0.0.0" || config.AppConfig.Server.Host == "" {
		log.Printf("Binding to all network interfaces (0.0.0.0)")
	} else {
		log.Printf("Warning: Binding only to %s, which may prevent external access", config.AppConfig.Server.Host)
	}

	// Create a context for coordinating graceful shutdown
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	// WaitGroup to track all running components
	var wg sync.WaitGroup

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start monitoring goroutine
	wg.Add(1)
	go monitorSystem(shutdownCtx, &wg)

	// Start the server in a goroutine
	serverErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Starting server on %s", addr)

		var err error
		if config.AppConfig.SSL.CertFile != "" && config.AppConfig.SSL.KeyFile != "" {
			// HTTPS server
			log.Printf("Using TLS with cert file %s and key file %s",
				config.AppConfig.SSL.CertFile,
				config.AppConfig.SSL.KeyFile)
			err = server.ListenAndServeTLS(
				config.AppConfig.SSL.CertFile,
				config.AppConfig.SSL.KeyFile)
		} else {
			// HTTP server
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Printf("Server failed: %v", err)
			serverErr <- err
			// Signal main goroutine to shut down
			stop <- syscall.SIGTERM
		}
	}()

	// Wait for signal or server error
	select {
	case <-stop:
		log.Println("Shutdown signal received")
	case err := <-serverErr:
		log.Printf("Server error caused shutdown: %v", err)
	}

	// Notify all running components that we're shutting down
	shutdownCancel()
	log.Println("Shutting down server...")

	// Notify connected clients via WebSocket about the shutdown
	handlers.DefaultWebSocketHub.BroadcastSystemMessage("shutdown", map[string]interface{}{
		"message": "Server is shutting down",
		"code":    1000, // Normal closure
	})

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(config.AppConfig.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Stop the worker pool with a timeout
	log.Println("Stopping worker pool...")
	done := make(chan bool, 1)
	go func() {
		processors.ShutdownWorkerPool()
		done <- true
	}()

	select {
	case <-done:
		log.Println("Worker pool stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Worker pool forced to stop after timeout")
	} // Stop LAN transfer service if it was enabled
	if lanHandler != nil {
		log.Println("Stopping LAN transfer service...")
		lanHandler.Stop()
		log.Println("LAN transfer service stopped")
	}

	// Clean up any temporary files
	cleanupTempFiles()

	// Wait for all components to finish (with timeout)
	wgDone := make(chan bool, 1)
	go func() {
		wg.Wait()
		wgDone <- true
	}()

	select {
	case <-wgDone:
		log.Println("All components stopped gracefully")
	case <-time.After(15 * time.Second):
		log.Println("Timeout waiting for components to stop")
	}

	log.Println("Server shutdown complete")
}

// cleanupTempFiles removes temporary files created during operation
func cleanupTempFiles() {
	tempDir := filepath.Join(config.AppConfig.Server.UploadsDir, "temp")

	log.Println("Cleaning up temporary files...")

	// Get all files in temp directory
	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Printf("Error reading temp directory: %v", err)
		return
	}

	// Delete temp files with our prefix
	count := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "fp-temp-") {
			err := os.Remove(filepath.Join(tempDir, file.Name()))
			if err != nil {
				log.Printf("Error removing temp file %s: %v", file.Name(), err)
			} else {
				count++
			}
		}
	}

	log.Printf("Removed %d temporary files", count)
}

// monitorSystem periodically collects system metrics and logs them
func monitorSystem(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Only monitor if verbose logging is enabled
	if !*verbose {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Log worker pool status
			activeWorkers, queueSize := processors.GetWorkerPoolStats()
			log.Printf("System status: %d active workers, %d tasks in queue", activeWorkers, queueSize)

			// Log memory usage
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("Memory usage: Alloc=%v MiB, TotalAlloc=%v MiB, Sys=%v MiB, NumGC=%v",
				m.Alloc/1024/1024,
				m.TotalAlloc/1024/1024,
				m.Sys/1024/1024,
				m.NumGC)

		case <-ctx.Done():
			log.Println("System monitoring stopped")
			return
		}
	}
}

// setupRoutes configures all the HTTP routes for the server
func setupRoutes(mux *http.ServeMux, fileHandler *handlers.FileHandler,
	lanHandler *handlers.LANTransferHandler, authHandler *auth.AuthHandler) {
	// File API routes
	mux.HandleFunc("/api/upload", fileHandler.UploadFile)
	mux.HandleFunc("/api/download", fileHandler.DownloadFile)
	mux.HandleFunc("/api/list", fileHandler.ListFiles)
	mux.HandleFunc("/api/url", fileHandler.GetSignedURL)
	mux.HandleFunc("/api/delete", fileHandler.DeleteFile)
	mux.HandleFunc("/api/storage/status", fileHandler.GetStorageProviderStatus)
	mux.HandleFunc("/api/preview/", fileHandler.MediaPreviewHandler) // Endpoint for media previews

	// WebSocket endpoint for real-time updates
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.ServeWs(handlers.DefaultWebSocketHub, w, r)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","version":"` + version + `"}`))
	})

	// Server info endpoint for metrics and status
	mux.HandleFunc("/api/server/info", func(w http.ResponseWriter, r *http.Request) {
		activeWorkers, queueSize := processors.GetWorkerPoolStats()
		info := map[string]interface{}{
			"version":       version,
			"uptime":        time.Since(startTime).String(),
			"activeWorkers": activeWorkers,
			"queueSize":     queueSize,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})
	// LAN transfer routes if enabled
	if lanHandler != nil {
		mux.HandleFunc("/api/lan/discover", lanHandler.HandleDiscoverDevices)
		mux.HandleFunc("/api/lan/initiate", lanHandler.HandleStartTransfer)
		mux.HandleFunc("/api/lan/accept", lanHandler.HandleCancelTransfer)
		mux.HandleFunc("/api/lan/status", lanHandler.HandleGetTransferStatus)
	}

	// Auth routes if enabled
	if authHandler != nil {
		mux.HandleFunc("/api/auth/login", authHandler.HandleLogin)
		mux.HandleFunc("/api/auth/callback", authHandler.HandleCallback)
		mux.HandleFunc("/api/auth/profile", authHandler.HandleProfile)
		mux.HandleFunc("/api/auth/logout", authHandler.HandleLogout)
		mux.HandleFunc("/api/auth/cloud-config", authHandler.HandleCloudConfig)
	}

	// Special handler for workflow.gif
	mux.HandleFunc("/workflow.gif", func(w http.ResponseWriter, r *http.Request) {
		// For demonstration purposes, we'll serve the workflow HTML page
		// which animates the workflow process
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, filepath.Join(config.AppConfig.Server.UIDir, "images/workflow.html"))
	})

	// Serve static UI files
	mux.Handle("/", http.FileServer(http.Dir(config.AppConfig.Server.UIDir)))
}
