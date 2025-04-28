// Package main is the entry point for the file processor server
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"fileprocessor/internal/config"
	"fileprocessor/internal/handlers"
	"fileprocessor/internal/middleware"
	"fileprocessor/internal/processors"
)

var (
	configFile = flag.String("config", "fileprocessor.ini", "Configuration file path")
	version    = "1.2.0"  // Version number for the application
)

func main() {
	// Parse command line flags
	flag.Parse()

	// Load application configuration
	if err := config.LoadConfig(*configFile); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print banner and version
	fmt.Printf("\n=================================\n")
	fmt.Printf("File Management System v%s\n", version)
	fmt.Printf("=================================\n\n")
	fmt.Printf("Server starting with %d worker threads\n", config.AppConfig.Workers.Count)
	if config.AppConfig.Features.EnableLAN {
		fmt.Printf("LAN file transfer capabilities enabled\n")
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
	addr := config.GetAddressString()
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
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