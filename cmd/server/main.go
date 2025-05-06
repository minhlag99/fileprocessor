// Package main is the entry point for the file processor server
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	version    = "1.3.0" // Version number for the application
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

func main() {
	// Parse command line flags
	flag.Parse()

	// Load application configuration
	if err := config.LoadConfig(*configFile); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Test configuration if requested
	if *testConfig {
		fmt.Println("Configuration test successful")
		return
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
		if redirectURL == "" {
			// Auto-configure redirect URL based on host and port
			proto := "http"
			if config.AppConfig.Server.CertFile != "" && config.AppConfig.Server.KeyFile != "" {
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

	// File API routes
	mux.HandleFunc("/api/upload", fileHandler.UploadFile)
	mux.HandleFunc("/api/download", fileHandler.DownloadFile)
	mux.HandleFunc("/api/list", fileHandler.ListFiles)
	mux.HandleFunc("/api/url", fileHandler.GetSignedURL)
	mux.HandleFunc("/api/delete", fileHandler.DeleteFile)
	mux.HandleFunc("/api/storage/status", fileHandler.GetStorageProviderStatus)

	// WebSocket endpoint for real-time updates
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.ServeWs(handlers.DefaultWebSocketHub, w, r)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// LAN transfer routes if enabled
	if lanHandler != nil {
		mux.HandleFunc("/api/lan/discover", lanHandler.HandleDiscoverPeers)
		mux.HandleFunc("/api/lan/initiate", lanHandler.HandleInitiateTransfer)
		mux.HandleFunc("/api/lan/accept", lanHandler.HandleAcceptTransfer)
		mux.HandleFunc("/api/lan/status", lanHandler.HandleTransferStatus)
	}

	// Auth routes if enabled
	if config.AppConfig.Features.EnableAuth {
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
				if config.AppConfig.Server.CertFile != "" && config.AppConfig.Server.KeyFile != "" {
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
		Addr:    addr,
		Handler: handler,
	}

	// Verify that we're binding to the right addresses for external access
	log.Printf("Server will be accessible at %s", addr)
	if config.AppConfig.Server.Host == "0.0.0.0" || config.AppConfig.Server.Host == "" {
		log.Printf("Binding to all network interfaces (0.0.0.0)")
	} else {
		log.Printf("Warning: Binding only to %s, which may prevent external access", config.AppConfig.Server.Host)
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on %s", addr)

		var err error
		if config.AppConfig.Server.CertFile != "" && config.AppConfig.Server.KeyFile != "" {
			// HTTPS server
			log.Printf("Using TLS with cert file %s and key file %s",
				config.AppConfig.Server.CertFile,
				config.AppConfig.Server.KeyFile)
			err = server.ListenAndServeTLS(
				config.AppConfig.Server.CertFile,
				config.AppConfig.Server.KeyFile)
		} else {
			// HTTP server
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(config.AppConfig.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Stop the worker pool
	processors.ShutdownWorkerPool()
	log.Println("Worker pool stopped")

	// Stop LAN transfer service if it was enabled
	if lanHandler != nil {
		lanHandler.Stop()
	}

	log.Println("Server shutdown complete")
}
